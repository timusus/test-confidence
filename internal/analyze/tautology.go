package analyze

import (
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/timusus/test-confidence/internal/model"
	"github.com/timusus/test-confidence/internal/parse"
)

// stubInfo tracks a stub's return value identifier and the method being stubbed.
type stubInfo struct {
	returnValue string
	methodName  string
}

// tautologyWithContext pairs a flag with the stub method name for SUT lookup.
type tautologyWithContext struct {
	flag           model.TautologyFlag
	stubMethodName string
}

// AnalyzeTautology detects tautological tests where the asserted value is
// the same identifier as the stub return value (pass-through testing).
// If prodFile is non-nil, it also checks SUT complexity to annotate findings.
func AnalyzeTautology(file *model.ParsedTestFile, prodFile *model.ParsedProductionFile) []model.TautologyFlag {
	var results []model.TautologyFlag

	for _, cls := range file.Classes {
		for _, method := range cls.Methods {
			tagged := analyzeMethodTautology(file, method)
			for i := range tagged {
				if prodFile != nil {
					annotateSUTComplexity(&tagged[i], prodFile)
				}
				results = append(results, tagged[i].flag)
			}
		}
	}

	return results
}

func analyzeMethodTautology(file *model.ParsedTestFile, method model.TestMethod) []tautologyWithContext {
	// Step 1: Collect return value identifiers and stub method names from stub calls.
	var stubs []stubInfo
	for _, stmt := range method.Statements {
		if parse.IsStubCall(stmt) {
			rv := parse.ExtractReturnValue(stmt)
			if rv != "" {
				_, methodName := parse.ExtractCallTarget(stmt)
				stubs = append(stubs, stubInfo{returnValue: rv, methodName: methodName})
			}
		}
	}
	if len(stubs) == 0 {
		return nil
	}

	// Step 2: Build assignment map (val x = y where y is a simple identifier).
	assignMap := buildAssignmentMap(method.Statements)

	// Step 3: Find assertion calls and check for tautology.
	var tagged []tautologyWithContext
	for _, stmt := range method.Statements {
		if !parse.IsAssertionCall(stmt) {
			continue
		}
		_, expected := parse.ExtractAssertedValues(stmt)
		if expected == "" {
			continue
		}

		// Resolve through assignment map.
		resolved := resolveIdentifier(expected, assignMap)

		// Check if resolved expected matches any return value identifier.
		for _, stub := range stubs {
			if resolved == stub.returnValue {
				tagged = append(tagged, tautologyWithContext{
					flag: model.TautologyFlag{
						File:             file.Path,
						Line:             int(stmt.Node.StartPoint().Row) + 1,
						Method:           method.Name,
						StubIdentifier:   stub.returnValue,
						AssertIdentifier: expected,
					},
					stubMethodName: stub.methodName,
				})
				break
			}
		}
	}

	return tagged
}

// buildAssignmentMap scans statements for simple `val x = y` patterns where
// y is a simple identifier, and returns a map from x to y.
func buildAssignmentMap(stmts []model.ASTNode) map[string]string {
	m := make(map[string]string)
	for _, stmt := range stmts {
		if stmt.Type() != "property_declaration" {
			continue
		}
		name, value := extractSimpleAssignment(stmt.Node, stmt.Source)
		if name != "" && value != "" {
			m[name] = value
		}
	}
	return m
}

// extractSimpleAssignment extracts (name, value) from a property_declaration
// where the value is a simple identifier (not a call or complex expression).
func extractSimpleAssignment(node *sitter.Node, src []byte) (name, value string) {
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		switch child.Type() {
		case "variable_declaration":
			for j := 0; j < int(child.NamedChildCount()); j++ {
				varChild := child.NamedChild(j)
				if varChild.Type() == "simple_identifier" {
					name = string(src[varChild.StartByte():varChild.EndByte()])
				}
			}
		case "simple_identifier":
			// A direct simple_identifier child of property_declaration (not inside
			// variable_declaration) is the RHS value. This handles: val x = y.
			value = string(src[child.StartByte():child.EndByte()])
		}
	}
	return name, value
}

// resolveIdentifier follows the assignment map to resolve an identifier.
// Stops after one level to avoid infinite loops on circular assignments.
func resolveIdentifier(ident string, assignMap map[string]string) string {
	if resolved, ok := assignMap[ident]; ok {
		return resolved
	}
	return ident
}

// annotateSUTComplexity sets SUTSimple on a tautology flag based on the
// production method's complexity. It matches the stub's method name against
// production methods.
func annotateSUTComplexity(tc *tautologyWithContext, prodFile *model.ParsedProductionFile) {
	stubMethod := tc.stubMethodName
	if stubMethod == "" {
		return
	}

	for _, pm := range prodFile.Methods {
		if pm.Name == stubMethod {
			if pm.IsSingleExpr && pm.BranchCount == 0 {
				tc.flag.SUTSimple = boolPtr(true)
			} else {
				tc.flag.SUTSimple = boolPtr(false)
			}
			return
		}
	}
}

func boolPtr(b bool) *bool {
	return &b
}
