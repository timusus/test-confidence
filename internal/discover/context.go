package discover

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/timusus/test-confidence/internal/config"
	"github.com/timusus/test-confidence/internal/model"
	"github.com/timusus/test-confidence/internal/parse"
	"github.com/timusus/test-confidence/internal/platform/android"
)

// BuildProjectContext scans all .kt files in the project to populate a
// ProjectContext with DI boundary types, fake types, type-to-package mappings,
// and third-party package prefixes. This runs once before per-file analysis.
func BuildProjectContext(root string, cfg config.Config) (*model.ProjectContext, error) {
	ctx := &model.ProjectContext{
		Platform:        DetectPlatform(root),
		Language:        model.Kotlin,
		DIBoundaryTypes: make(map[string]bool),
		FakeTypes:       make(map[string]bool),
		TypePackages:    make(map[string]string),
		ThirdPartyPkgs:  android.ThirdPartyPkgs,
	}

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".kt") {
			return nil
		}

		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		tree, err := parse.ParseKotlin(src)
		if err != nil {
			return nil // skip unparseable files
		}
		defer tree.Close()

		root := tree.RootNode()
		scanFile(root, src, ctx)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return ctx, nil
}

// scanFile extracts DI boundaries, fake types, and import mappings from a
// single parsed Kotlin file.
func scanFile(root *sitter.Node, src []byte, ctx *model.ProjectContext) {
	for i := 0; i < int(root.NamedChildCount()); i++ {
		child := root.NamedChild(i)
		switch child.Type() {
		case "import_list":
			extractImportMappings(child, src, ctx)
		case "class_declaration":
			scanClassDeclaration(child, src, ctx)
		}
	}
}

// extractImportMappings builds TypePackages from import statements.
// For "com.example.network.ApiClient", maps ApiClient -> "com.example.network".
func extractImportMappings(importList *sitter.Node, src []byte, ctx *model.ProjectContext) {
	for i := 0; i < int(importList.NamedChildCount()); i++ {
		header := importList.NamedChild(i)
		if header.Type() != "import_header" {
			continue
		}
		for j := 0; j < int(header.NamedChildCount()); j++ {
			ident := header.NamedChild(j)
			if ident.Type() == "identifier" {
				path := sitterText(ident, src)
				lastDot := strings.LastIndex(path, ".")
				if lastDot > 0 {
					typeName := path[lastDot+1:]
					pkg := path[:lastDot]
					// Only map type names (start with uppercase)
					if len(typeName) > 0 && typeName[0] >= 'A' && typeName[0] <= 'Z' {
						ctx.TypePackages[typeName] = pkg
					}
				}
			}
		}
	}
}

// scanClassDeclaration checks for @Module classes (DI boundaries) and
// Fake-prefixed classes.
func scanClassDeclaration(node *sitter.Node, src []byte, ctx *model.ProjectContext) {
	className := ""
	var modifiersNode *sitter.Node
	var classBody *sitter.Node

	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		switch child.Type() {
		case "type_identifier":
			className = sitterText(child, src)
		case "modifiers":
			modifiersNode = child
		case "class_body":
			classBody = child
		}
	}

	// Check for Fake-prefixed classes
	if strings.HasPrefix(className, "Fake") && len(className) > 4 {
		baseType := className[4:] // strip "Fake" prefix
		ctx.FakeTypes[baseType] = true
	}

	// Check for @Module annotation -> scan for @Binds methods
	if modifiersNode != nil && hasAnnotation(modifiersNode, src, "Module") && classBody != nil {
		scanModuleBody(classBody, src, ctx)
	}
}

// hasAnnotation checks if a modifiers node contains an annotation with the given name.
func hasAnnotation(modifiers *sitter.Node, src []byte, name string) bool {
	for i := 0; i < int(modifiers.NamedChildCount()); i++ {
		child := modifiers.NamedChild(i)
		if child.Type() != "annotation" {
			continue
		}
		for j := 0; j < int(child.NamedChildCount()); j++ {
			annChild := child.NamedChild(j)
			if annChild.Type() == "user_type" {
				for k := 0; k < int(annChild.NamedChildCount()); k++ {
					ti := annChild.NamedChild(k)
					if ti.Type() == "type_identifier" && sitterText(ti, src) == name {
						return true
					}
				}
			}
		}
	}
	return false
}

// scanModuleBody looks for @Binds methods and extracts their return types
// as DI boundary types.
func scanModuleBody(classBody *sitter.Node, src []byte, ctx *model.ProjectContext) {
	for i := 0; i < int(classBody.NamedChildCount()); i++ {
		child := classBody.NamedChild(i)
		if child.Type() != "function_declaration" {
			continue
		}

		// Check if this function has @Binds annotation
		if !functionHasAnnotation(child, src, "Binds") {
			continue
		}

		// The return type is a user_type child of function_declaration
		// (not inside function_value_parameters)
		returnType := extractFunctionReturnType(child, src)
		if returnType != "" {
			ctx.DIBoundaryTypes[returnType] = true
		}
	}
}

// functionHasAnnotation checks if a function_declaration has a specific annotation.
func functionHasAnnotation(funcNode *sitter.Node, src []byte, name string) bool {
	for i := 0; i < int(funcNode.NamedChildCount()); i++ {
		child := funcNode.NamedChild(i)
		if child.Type() == "modifiers" {
			return hasAnnotation(child, src, name)
		}
	}
	return false
}

// extractFunctionReturnType gets the return type name from a function declaration.
// The return type is a user_type child directly under function_declaration
// (distinct from user_type nodes inside function_value_parameters).
func extractFunctionReturnType(funcNode *sitter.Node, src []byte) string {
	for i := 0; i < int(funcNode.NamedChildCount()); i++ {
		child := funcNode.NamedChild(i)
		if child.Type() == "user_type" {
			// Extract the type_identifier from user_type
			for j := 0; j < int(child.NamedChildCount()); j++ {
				ti := child.NamedChild(j)
				if ti.Type() == "type_identifier" {
					return sitterText(ti, src)
				}
			}
			return sitterText(child, src)
		}
	}
	return ""
}

func sitterText(node *sitter.Node, src []byte) string {
	return string(src[node.StartByte():node.EndByte()])
}
