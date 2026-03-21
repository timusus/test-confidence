package parse

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/swift"
	"github.com/timusus/test-confidence/internal/model"
)

// ParseSwift parses Swift source code and returns the resulting tree-sitter tree.
func ParseSwift(src []byte) (*sitter.Tree, error) {
	parser := sitter.NewParser()
	parser.SetLanguage(swift.GetLanguage())

	tree, err := parser.ParseCtx(context.Background(), nil, src)
	if err != nil {
		return nil, fmt.Errorf("tree-sitter parse (swift): %w", err)
	}

	return tree, nil
}

// preProcessSwiftMacros replaces Swift freestanding macros with parseable function calls
// so tree-sitter (which doesn't support macros in our grammar version) can produce proper
// AST nodes instead of ERROR + tuple_expression.
// Only replaces #expect( and #require( — not compiler directives like #if, #available, etc.
func preProcessSwiftMacros(src []byte) []byte {
	// Fast path: no macros to replace
	if !strings.Contains(string(src), "#expect(") && !strings.Contains(string(src), "#require(") {
		return src
	}
	result := make([]byte, 0, len(src))
	i := 0
	for i < len(src) {
		if src[i] == '#' && i+1 < len(src) {
			// Check for #expect( or #require(
			rest := src[i:]
			if len(rest) >= 8 && string(rest[:8]) == "#expect(" {
				result = append(result, "__expect("...)
				i += 8
				continue
			}
			if len(rest) >= 9 && string(rest[:9]) == "#require(" {
				result = append(result, "__require("...)
				i += 9
				continue
			}
		}
		result = append(result, src[i])
		i++
	}
	return result
}

// ParseSwiftTestFile parses a Swift test file and extracts structured data.
func ParseSwiftTestFile(path string, src []byte) (*model.ParsedTestFile, error) {
	// Pre-process Swift macros so tree-sitter produces proper AST nodes.
	// The original source is preserved in the result for text-based analysis.
	parseSrc := preProcessSwiftMacros(src)
	tree, err := ParseSwift(parseSrc)
	if err != nil {
		return nil, err
	}

	root := tree.RootNode()

	result := &model.ParsedTestFile{
		Path:     path,
		Language: model.Swift,
		Source:   parseSrc, // Use pre-processed source for AST node text extraction
		Tree:     tree,
	}

	// Build a map of class name -> index for merging extensions later
	classIndex := map[string]int{}

	// All AST extraction uses parseSrc since tree-sitter nodes reference
	// the pre-processed source byte offsets.
	src = parseSrc

	// First pass: imports and class declarations
	for i := 0; i < int(root.NamedChildCount()); i++ {
		child := root.NamedChild(i)
		switch child.Type() {
		case "import_declaration":
			imp := extractSwiftImport(child, src)
			if imp.Path != "" {
				result.Imports = append(result.Imports, imp)
			}
		case "class_declaration":
			name, isExtension := swiftClassOrExtensionName(child, src)
			if isExtension {
				// Will handle in second pass
				continue
			}
			if isSwiftTestingClass(child, src) {
				cls := extractSwiftTestingClass(child, src)
				classIndex[cls.Name] = len(result.Classes)
				result.Classes = append(result.Classes, cls)
			} else if isQuickSpecClass(child, src) {
				cls := extractQuickSpecClass(child, src, name)
				classIndex[cls.Name] = len(result.Classes)
				result.Classes = append(result.Classes, cls)
			} else if isTestClassName(name) {
				cls := extractSwiftClass(child, src)
				classIndex[cls.Name] = len(result.Classes)
				result.Classes = append(result.Classes, cls)
			}
		}
	}

	// Second pass: extensions — merge methods into matching classes
	for i := 0; i < int(root.NamedChildCount()); i++ {
		child := root.NamedChild(i)
		if child.Type() != "class_declaration" {
			continue
		}
		name, isExtension := swiftClassOrExtensionName(child, src)
		if !isExtension {
			continue
		}
		if !isTestClassName(name) {
			continue
		}

		// Extract methods/properties from the extension body
		extCls := extractSwiftExtensionBody(child, src, name)

		if idx, ok := classIndex[name]; ok {
			// Merge into existing class
			result.Classes[idx].Methods = append(result.Classes[idx].Methods, extCls.Methods...)
			result.Classes[idx].Properties = append(result.Classes[idx].Properties, extCls.Properties...)
			result.Classes[idx].SetupBlocks = append(result.Classes[idx].SetupBlocks, extCls.SetupBlocks...)
		} else if len(extCls.Methods) > 0 || len(extCls.Properties) > 0 {
			// No matching class — create a new entry
			classIndex[name] = len(result.Classes)
			result.Classes = append(result.Classes, extCls)
		}
	}

	// Also check for top-level @Test functions (Swift Testing without a wrapping class)
	if len(result.Classes) == 0 {
		cls := extractTopLevelSwiftTestingMethods(root, src, path)
		if len(cls.Methods) > 0 {
			result.Classes = append(result.Classes, cls)
		}
	}

	return result, nil
}

// swiftClassOrExtensionName extracts the name from a class_declaration node
// and reports whether it's an extension (user_type) or a regular class (type_identifier).
func swiftClassOrExtensionName(node *sitter.Node, src []byte) (name string, isExtension bool) {
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		switch child.Type() {
		case "type_identifier":
			return nodeText(child, src), false
		case "user_type":
			// Extensions show up with user_type in tree-sitter Swift grammar
			return nodeText(child, src), true
		}
	}
	return "", false
}

// isTestClassName returns true if the class name matches common test naming patterns.
// The file is already known to be a test file, so name-based detection is safe.
func isTestClassName(name string) bool {
	if name == "" {
		return false
	}
	return strings.HasSuffix(name, "Tests") ||
		strings.HasSuffix(name, "Test") ||
		strings.HasSuffix(name, "TestCase") ||
		strings.HasSuffix(name, "Spec")
}

// extractSwiftExtensionBody extracts methods, properties, and setup blocks
// from an extension's class_body. Handles both XCTest (func test*) and
// Swift Testing (@Test) method patterns.
func extractSwiftExtensionBody(node *sitter.Node, src []byte, name string) model.TestClass {
	cls := model.TestClass{Name: name}
	// Check if the extension body contains @Test attributes (Swift Testing)
	text := nodeText(node, src)
	hasSwiftTesting := strings.Contains(text, "@Test")
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		if child.Type() == "class_body" {
			if hasSwiftTesting {
				extractSwiftTestingClassBody(child, src, &cls)
			} else {
				extractSwiftClassBody(child, src, &cls)
			}
		}
	}
	return cls
}

// ParseSwiftProductionFile parses a Swift production file and extracts method metadata.
func ParseSwiftProductionFile(path string, src []byte) (*model.ParsedProductionFile, error) {
	tree, err := ParseSwift(src)
	if err != nil {
		return nil, err
	}

	root := tree.RootNode()

	result := &model.ParsedProductionFile{
		Path:     path,
		Language: model.Swift,
		Source:   src,
		Tree:     tree,
	}

	for i := 0; i < int(root.NamedChildCount()); i++ {
		child := root.NamedChild(i)
		if child.Type() == "class_declaration" {
			extractSwiftProductionMethods(child, src, result)
		}
	}

	return result, nil
}

// extractSwiftImport extracts an import path from an import_declaration node.
func extractSwiftImport(node *sitter.Node, src []byte) model.Import {
	imp := model.Import{
		Line: int(node.StartPoint().Row) + 1,
	}
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		if child.Type() == "identifier" {
			imp.Path = nodeText(child, src)
		}
	}
	return imp
}

// isSwiftTestingClass checks if a class/struct has @Test-annotated methods (Swift Testing framework).
func isSwiftTestingClass(node *sitter.Node, src []byte) bool {
	text := nodeText(node, src)
	return strings.Contains(text, "@Test")
}

// extractSwiftTestingClass extracts a TestClass from a Swift Testing class/struct.
func extractSwiftTestingClass(node *sitter.Node, src []byte) model.TestClass {
	cls := model.TestClass{}

	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		switch child.Type() {
		case "type_identifier", "simple_identifier":
			if cls.Name == "" {
				cls.Name = nodeText(child, src)
			}
		case "class_body":
			extractSwiftTestingClassBody(child, src, &cls)
		}
	}

	return cls
}

// extractSwiftTestingClassBody handles Swift Testing class/struct bodies.
func extractSwiftTestingClassBody(node *sitter.Node, src []byte, cls *model.TestClass) {
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		switch child.Type() {
		case "property_declaration":
			prop := extractSwiftProperty(child, src)
			cls.Properties = append(cls.Properties, prop)
		case "function_declaration":
			if hasSwiftTestAttribute(child, src) {
				method := extractSwiftMethod(child, src)
				cls.Methods = append(cls.Methods, method)
			} else {
				name := swiftFuncName(child, src)
				if isSwiftSetupMethod(name) {
					setupNode := extractSwiftFunctionBodyNode(child, src)
					if setupNode != nil {
						cls.SetupBlocks = append(cls.SetupBlocks, *setupNode)
					}
				} else if strings.HasPrefix(name, "test") {
					method := extractSwiftMethod(child, src)
					cls.Methods = append(cls.Methods, method)
				}
			}
		}
	}
}

// hasSwiftTestAttribute checks if a function has the @Test attribute.
func hasSwiftTestAttribute(node *sitter.Node, src []byte) bool {
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		if child.Type() == "attribute" {
			text := nodeText(child, src)
			if strings.Contains(text, "Test") {
				return true
			}
		}
		// Tree-sitter may wrap attributes inside a modifiers node
		if child.Type() == "modifiers" {
			for j := 0; j < int(child.NamedChildCount()); j++ {
				modChild := child.NamedChild(j)
				if modChild.Type() == "attribute" {
					text := nodeText(modChild, src)
					if strings.Contains(text, "Test") {
						return true
					}
				}
			}
		}
	}
	return false
}

// extractTopLevelSwiftTestingMethods extracts @Test functions at the top level.
func extractTopLevelSwiftTestingMethods(root *sitter.Node, src []byte, path string) model.TestClass {
	cls := model.TestClass{
		Name: strings.TrimSuffix(strings.TrimSuffix(filepath.Base(path), ".swift"), "Tests"),
	}

	for i := 0; i < int(root.NamedChildCount()); i++ {
		child := root.NamedChild(i)
		if child.Type() == "function_declaration" {
			if hasSwiftTestAttribute(child, src) {
				method := extractSwiftMethod(child, src)
				cls.Methods = append(cls.Methods, method)
			}
		}
	}

	return cls
}

// extractSwiftClass extracts a TestClass from a Swift class_declaration node.
func extractSwiftClass(node *sitter.Node, src []byte) model.TestClass {
	cls := model.TestClass{}

	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		switch child.Type() {
		case "type_identifier":
			cls.Name = nodeText(child, src)
		case "class_body":
			extractSwiftClassBody(child, src, &cls)
		}
	}

	return cls
}

// extractSwiftClassBody extracts properties, test methods, and setup blocks from a class body.
func extractSwiftClassBody(node *sitter.Node, src []byte, cls *model.TestClass) {
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		switch child.Type() {
		case "property_declaration":
			prop := extractSwiftProperty(child, src)
			cls.Properties = append(cls.Properties, prop)
		case "function_declaration":
			name := swiftFuncName(child, src)
			if isSwiftSetupMethod(name) {
				setupNode := extractSwiftFunctionBodyNode(child, src)
				if setupNode != nil {
					cls.SetupBlocks = append(cls.SetupBlocks, *setupNode)
				}
			} else if strings.HasPrefix(name, "test") {
				method := extractSwiftMethod(child, src)
				cls.Methods = append(cls.Methods, method)
			}
		}
	}
}

// swiftSetupMethods are method names that indicate setup/teardown in XCTest.
var swiftSetupMethods = map[string]bool{
	"setUp":              true,
	"setUpWithError":     true,
	"tearDown":           true,
	"tearDownWithError":  true,
}

// isSwiftSetupMethod returns true if the function name is a known setup/teardown method.
func isSwiftSetupMethod(name string) bool {
	return swiftSetupMethods[name]
}

// swiftFuncName extracts the simple_identifier name from a function_declaration.
func swiftFuncName(node *sitter.Node, src []byte) string {
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		if child.Type() == "simple_identifier" {
			return nodeText(child, src)
		}
	}
	return ""
}

// extractSwiftProperty extracts a Property from a Swift property_declaration.
func extractSwiftProperty(node *sitter.Node, src []byte) model.Property {
	prop := model.Property{
		Line: int(node.StartPoint().Row) + 1,
	}

	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		switch child.Type() {
		case "pattern":
			for j := 0; j < int(child.NamedChildCount()); j++ {
				patChild := child.NamedChild(j)
				if patChild.Type() == "simple_identifier" {
					prop.Name = nodeText(patChild, src)
				}
			}
		case "type_annotation":
			prop.TypeName = extractSwiftTypeName(child, src)
		case "call_expression":
			// Initializer like `MockUserRepository()` — extract type from call name
			if prop.TypeName == "" {
				prop.TypeName = swiftCallExprName(child, src)
			}
			prop.HasInitializer = true
		default:
			// Other expression types as initializers (e.g., makeSUT(), literal values)
			childType := child.Type()
			if childType != "modifiers" && childType != "type_annotation" && childType != "pattern" {
				prop.HasInitializer = true
			}
		}
	}

	return prop
}

// extractSwiftTypeName extracts the type name from a type_annotation node.
func extractSwiftTypeName(node *sitter.Node, src []byte) string {
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		if child.Type() == "user_type" {
			for j := 0; j < int(child.NamedChildCount()); j++ {
				typeChild := child.NamedChild(j)
				if typeChild.Type() == "type_identifier" {
					return nodeText(typeChild, src)
				}
			}
		}
		// Handle optional types (e.g., UserViewModel!)
		if child.Type() == "optional_type" || child.Type() == "force_unwrap_expression" {
			text := nodeText(child, src)
			text = strings.TrimSuffix(text, "!")
			text = strings.TrimSuffix(text, "?")
			return text
		}
	}
	return ""
}

// swiftCallExprName extracts the top-level function name from a call_expression.
func swiftCallExprName(node *sitter.Node, src []byte) string {
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		if child.Type() == "simple_identifier" {
			return nodeText(child, src)
		}
	}
	return ""
}

// extractSwiftMethod extracts a TestMethod from a Swift function_declaration.
func extractSwiftMethod(node *sitter.Node, src []byte) model.TestMethod {
	method := model.TestMethod{
		LineStart: int(node.StartPoint().Row) + 1,
		LineEnd:   int(node.EndPoint().Row) + 1,
	}

	// Swift test methods don't use @Test — they start with "test".
	// We add a synthetic "Test" annotation so analyzers can treat them uniformly.
	method.Annotations = []model.Annotation{{Name: "Test"}}

	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		switch child.Type() {
		case "simple_identifier":
			method.Name = nodeText(child, src)
		case "function_body":
			method.Statements = extractSwiftStatements(child, src)
		}
	}

	// Detect XCTSkip in the body — treat as ignored test
	for _, stmt := range method.Statements {
		if containsXCTSkip(stmt.Node, src) {
			method.Annotations = append(method.Annotations, model.Annotation{
				Name: "Ignore",
				Line: method.LineStart,
			})
			break
		}
	}

	return method
}

// extractSwiftStatements extracts top-level statements from a function_body.
func extractSwiftStatements(funcBody *sitter.Node, src []byte) []model.ASTNode {
	var stmts []model.ASTNode
	for i := 0; i < int(funcBody.NamedChildCount()); i++ {
		child := funcBody.NamedChild(i)
		if child.Type() == "statements" {
			for j := 0; j < int(child.NamedChildCount()); j++ {
				stmt := child.NamedChild(j)
				stmts = append(stmts, model.ASTNode{Node: stmt, Source: src})
			}
			return stmts
		}
	}
	return stmts
}

// extractSwiftFunctionBodyNode returns the function_body ASTNode for a function_declaration.
func extractSwiftFunctionBodyNode(funcNode *sitter.Node, src []byte) *model.ASTNode {
	for i := 0; i < int(funcNode.NamedChildCount()); i++ {
		child := funcNode.NamedChild(i)
		if child.Type() == "function_body" {
			return &model.ASTNode{Node: child, Source: src}
		}
	}
	return nil
}

// containsXCTSkip recursively checks if a node contains an XCTSkip call.
func containsXCTSkip(node *sitter.Node, src []byte) bool {
	if node.Type() == "call_expression" {
		name := swiftCallExprName(node, src)
		if name == "XCTSkip" {
			return true
		}
	}
	text := nodeText(node, src)
	return strings.Contains(text, "XCTSkip")
}

// extractSwiftProductionMethods extracts production methods from a Swift class.
func extractSwiftProductionMethods(classNode *sitter.Node, src []byte, result *model.ParsedProductionFile) {
	for i := 0; i < int(classNode.NamedChildCount()); i++ {
		child := classNode.NamedChild(i)
		if child.Type() == "class_body" {
			for j := 0; j < int(child.NamedChildCount()); j++ {
				member := child.NamedChild(j)
				if member.Type() == "function_declaration" {
					method := extractSwiftProductionMethod(member, src)
					result.Methods = append(result.Methods, method)
				}
			}
		}
	}
}

// extractSwiftProductionMethod extracts a ProductionMethod from a Swift function_declaration.
func extractSwiftProductionMethod(funcNode *sitter.Node, src []byte) model.ProductionMethod {
	method := model.ProductionMethod{
		LineStart: int(funcNode.StartPoint().Row) + 1,
		LineEnd:   int(funcNode.EndPoint().Row) + 1,
	}

	for i := 0; i < int(funcNode.NamedChildCount()); i++ {
		child := funcNode.NamedChild(i)
		switch child.Type() {
		case "simple_identifier":
			method.Name = nodeText(child, src)
		case "function_body":
			method.Body = []model.ASTNode{{Node: child, Source: src}}
			method.BranchCount = countSwiftBranches(child, src)
			method.CallCount = countSwiftCalls(child, src)
		}
	}

	return method
}

// countSwiftBranches counts branching nodes in Swift: if_statement, switch_statement, guard_statement.
func countSwiftBranches(node *sitter.Node, _ []byte) int {
	count := 0
	switch node.Type() {
	case "if_statement", "switch_statement", "guard_statement":
		count++
	}
	for i := 0; i < int(node.NamedChildCount()); i++ {
		count += countSwiftBranches(node.NamedChild(i), nil)
	}
	return count
}

// countSwiftCalls counts call_expression nodes in a Swift function body.
func countSwiftCalls(node *sitter.Node, _ []byte) int {
	count := 0
	if node.Type() == "call_expression" {
		count++
	}
	for i := 0; i < int(node.NamedChildCount()); i++ {
		count += countSwiftCalls(node.NamedChild(i), nil)
	}
	return count
}

// --- Quick/Nimble support ---

// isQuickSpecClass checks if a class inherits from QuickSpec.
func isQuickSpecClass(node *sitter.Node, src []byte) bool {
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		if child.Type() == "inheritance_specifier" || child.Type() == "type_identifier" {
			text := nodeText(child, src)
			if strings.Contains(text, "QuickSpec") || strings.Contains(text, "QuickConfiguration") {
				return true
			}
		}
	}
	// Fallback: check full class text for ": QuickSpec" pattern
	text := nodeText(node, src)
	return strings.Contains(text, ": QuickSpec") || strings.Contains(text, ": QuickConfiguration")
}

// extractQuickSpecClass extracts test methods from a Quick spec by finding `it()` blocks
// inside the `spec()` method's `describe`/`context` hierarchy.
func extractQuickSpecClass(node *sitter.Node, src []byte, name string) model.TestClass {
	cls := model.TestClass{Name: name}

	// Find the class_body, then the spec() function
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		if child.Type() == "class_body" {
			for j := 0; j < int(child.NamedChildCount()); j++ {
				member := child.NamedChild(j)
				if member.Type() == "function_declaration" {
					fname := swiftFuncName(member, src)
					if fname == "spec" || fname == "sharedExamples" {
						// Walk the function body for it() blocks
						extractQuickItBlocks(member, src, &cls)
					}
				}
			}
		}
	}

	return cls
}

// extractQuickItBlocks recursively walks a Quick spec function body to find
// it("description") { ... } and itBehavesLike("...") calls, treating each as a test method.
func extractQuickItBlocks(node *sitter.Node, src []byte, cls *model.TestClass) {
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)

		if child.Type() == "call_expression" {
			callName := swiftCallExprName(child, src)
			if callName == "it" || callName == "itBehavesLike" {
				method := model.TestMethod{
					Name:        extractQuickItDescription(child, src, callName),
					Annotations: []model.Annotation{{Name: "Test"}},
					LineStart:   int(child.StartPoint().Row) + 1,
					LineEnd:     int(child.EndPoint().Row) + 1,
					Statements:  extractQuickItStatements(child, src),
				}
				cls.Methods = append(cls.Methods, method)
				continue // Don't recurse into it() bodies for more it() blocks
			}
			if callName == "beforeEach" || callName == "beforeSuite" {
				// Treat as setup block
				stmts := extractQuickItStatements(child, src)
				if len(stmts) > 0 {
					cls.SetupBlocks = append(cls.SetupBlocks, stmts[0])
				}
				continue
			}
		}

		// Recurse into describe/context/other blocks to find nested it() calls
		extractQuickItBlocks(child, src, cls)
	}
}

// extractQuickItDescription extracts the string description from an it("...") call.
func extractQuickItDescription(callNode *sitter.Node, src []byte, prefix string) string {
	text := nodeText(callNode, src)
	// Find the string argument: it("some description")
	start := strings.Index(text, "\"")
	if start < 0 {
		return prefix
	}
	end := strings.Index(text[start+1:], "\"")
	if end < 0 {
		return prefix
	}
	desc := text[start+1 : start+1+end]
	if len(desc) > 80 {
		desc = desc[:80]
	}
	return desc
}

// extractQuickItStatements extracts the statements from the trailing closure of an it() call.
func extractQuickItStatements(callNode *sitter.Node, src []byte) []model.ASTNode {
	var stmts []model.ASTNode
	// Walk children to find the lambda/closure body
	walkForClosure(callNode, src, &stmts)
	return stmts
}

// walkForClosure recursively finds closure_expression or lambda nodes and extracts their statements.
func walkForClosure(node *sitter.Node, src []byte, stmts *[]model.ASTNode) {
	nodeType := node.Type()
	if nodeType == "lambda_literal" || nodeType == "closure_expression" {
		// Extract top-level statements from the closure body
		for i := 0; i < int(node.NamedChildCount()); i++ {
			child := node.NamedChild(i)
			if child.Type() == "statements" {
				for j := 0; j < int(child.NamedChildCount()); j++ {
					stmt := child.NamedChild(j)
					*stmts = append(*stmts, model.ASTNode{Node: stmt, Source: src})
				}
				return
			}
		}
		// Single-expression closure
		for i := 0; i < int(node.NamedChildCount()); i++ {
			child := node.NamedChild(i)
			*stmts = append(*stmts, model.ASTNode{Node: child, Source: src})
		}
		return
	}
	for i := 0; i < int(node.NamedChildCount()); i++ {
		walkForClosure(node.NamedChild(i), src, stmts)
	}
}
