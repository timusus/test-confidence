package parse

import (
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/timusus/test-confidence/internal/model"
)

// setupAnnotations are method annotations that indicate setup/teardown methods.
var setupAnnotations = map[string]bool{
	"Before":     true,
	"BeforeEach": true,
	"After":      true,
	"AfterEach":  true,
}

// ParseTestFile parses a test file and extracts structured data.
// It dispatches to the appropriate language-specific parser based on lang.
func ParseTestFile(path string, src []byte, lang model.Language) (*model.ParsedTestFile, error) {
	if lang == model.Swift {
		return ParseSwiftTestFile(path, src)
	}
	return parseKotlinTestFile(path, src, lang)
}

// parseKotlinTestFile parses a Kotlin test file and extracts structured data.
func parseKotlinTestFile(path string, src []byte, lang model.Language) (*model.ParsedTestFile, error) {
	tree, err := ParseKotlin(src)
	if err != nil {
		return nil, err
	}

	root := tree.RootNode()

	result := &model.ParsedTestFile{
		Path:     path,
		Language: lang,
		Source:   src,
		Tree:     tree,
	}

	for i := 0; i < int(root.NamedChildCount()); i++ {
		child := root.NamedChild(i)
		switch child.Type() {
		case "import_list":
			result.Imports = extractImports(child, src)
		case "class_declaration":
			cls := extractClass(child, src)
			result.Classes = append(result.Classes, cls)
		case "prefix_expression":
			// Tree-sitter Kotlin grammar sometimes wraps annotated classes
			// (e.g., @RunWith @Config class Foo) in prefix_expression nodes
			// instead of putting annotations in the class's modifiers.
			if classNode := findClassInPrefixExpression(child, src); classNode != nil {
				cls := extractClass(classNode, src)
				result.Classes = append(result.Classes, cls)
			} else if cls := extractClassFromMisparsedInfix(child, src); cls != nil {
				result.Classes = append(result.Classes, *cls)
			}
		}
	}

	return result, nil
}

// findClassInPrefixExpression recursively unwraps nested prefix_expression nodes
// to find a class_declaration inside. This handles cases where tree-sitter wraps
// annotated classes (e.g., @RunWith(...) @Config(...) class Foo) in prefix_expression
// nodes rather than attaching annotations as modifiers on the class_declaration.
func findClassInPrefixExpression(node *sitter.Node, src []byte) *sitter.Node {
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		switch child.Type() {
		case "class_declaration":
			return child
		case "prefix_expression":
			if found := findClassInPrefixExpression(child, src); found != nil {
				return found
			}
		case "infix_expression":
			// Tree-sitter Kotlin grammar sometimes misparses annotated classes as
			// infix_expression with children: simple_identifier("class"), simple_identifier(name),
			// lambda_literal({body}). Detect this pattern and return nil so we fall through
			// to extractClassFromMisparsedInfix.
		}
	}
	return nil
}

// extractClassFromMisparsedInfix handles a tree-sitter misparse where an annotated class
// is parsed as prefix_expression -> ... -> infix_expression("class", Name, {body}).
// This happens with certain @Config annotation arguments like `sdk = [35]`.
func extractClassFromMisparsedInfix(node *sitter.Node, src []byte) *model.TestClass {
	// Walk to find the infix_expression
	infix := findInfixInPrefix(node, src)
	if infix == nil {
		return nil
	}

	// Check if this is a misparse of "class ClassName { ... }"
	if infix.NamedChildCount() < 3 {
		return nil
	}
	child0 := infix.NamedChild(0)
	child1 := infix.NamedChild(1)
	child2 := infix.NamedChild(2)
	if child0.Type() != "simple_identifier" || nodeText(child0, src) != "class" {
		return nil
	}
	if child1.Type() != "simple_identifier" {
		return nil
	}
	if child2.Type() != "lambda_literal" {
		return nil
	}

	className := nodeText(child1, src)
	cls := model.TestClass{Name: className}

	// The lambda_literal contains the class body — extract properties and methods.
	extractClassBodyFromLambda(child2, src, &cls)
	return &cls
}

// findInfixInPrefix recursively finds the infix_expression inside nested prefix_expressions.
func findInfixInPrefix(node *sitter.Node, src []byte) *sitter.Node {
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		switch child.Type() {
		case "infix_expression":
			return child
		case "prefix_expression":
			if found := findInfixInPrefix(child, src); found != nil {
				return found
			}
		}
	}
	return nil
}

// extractClassBodyFromLambda extracts test methods and properties from a lambda_literal
// that tree-sitter produced by misparsing a class body.
func extractClassBodyFromLambda(lambda *sitter.Node, src []byte, cls *model.TestClass) {
	for i := 0; i < int(lambda.NamedChildCount()); i++ {
		child := lambda.NamedChild(i)
		if child.Type() == "statements" {
			for j := 0; j < int(child.NamedChildCount()); j++ {
				stmt := child.NamedChild(j)
				switch stmt.Type() {
				case "property_declaration":
					prop := extractProperty(stmt, src)
					cls.Properties = append(cls.Properties, prop)
				case "function_declaration":
					annotations := extractFunctionAnnotations(stmt, src)
					if isTestMethod(annotations) {
						method := extractMethod(stmt, src, annotations)
						cls.Methods = append(cls.Methods, method)
					} else if isSetupMethod(annotations) {
						setupNode := extractFunctionBodyNode(stmt, src)
						if setupNode != nil {
							cls.SetupBlocks = append(cls.SetupBlocks, *setupNode)
						}
					}
				}
			}
		}
	}
}

func extractImports(node *sitter.Node, src []byte) []model.Import {
	var imports []model.Import
	for i := 0; i < int(node.NamedChildCount()); i++ {
		header := node.NamedChild(i)
		if header.Type() != "import_header" {
			continue
		}
		// The identifier child contains the full import path.
		for j := 0; j < int(header.NamedChildCount()); j++ {
			ident := header.NamedChild(j)
			if ident.Type() == "identifier" {
				imports = append(imports, model.Import{
					Path: nodeText(ident, src),
					Line: int(ident.StartPoint().Row) + 1,
				})
			}
		}
	}
	return imports
}

func extractClass(node *sitter.Node, src []byte) model.TestClass {
	cls := model.TestClass{}

	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		switch child.Type() {
		case "type_identifier":
			cls.Name = nodeText(child, src)
		case "modifiers":
			cls.Annotations = extractAnnotations(child, src)
		case "class_body":
			extractClassBody(child, src, &cls)
		}
	}

	return cls
}

func extractClassBody(node *sitter.Node, src []byte, cls *model.TestClass) {
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		switch child.Type() {
		case "property_declaration":
			prop := extractProperty(child, src)
			cls.Properties = append(cls.Properties, prop)
		case "function_declaration":
			annotations := extractFunctionAnnotations(child, src)
			if isTestMethod(annotations) {
				method := extractMethod(child, src, annotations)
				cls.Methods = append(cls.Methods, method)
			} else if isSetupMethod(annotations) {
				setupNode := extractFunctionBodyNode(child, src)
				if setupNode != nil {
					cls.SetupBlocks = append(cls.SetupBlocks, *setupNode)
				}
			}
		}
	}
}

func extractProperty(node *sitter.Node, src []byte) model.Property {
	prop := model.Property{
		Line: int(node.StartPoint().Row) + 1,
	}

	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		switch child.Type() {
		case "modifiers":
			prop.Annotations = extractAnnotations(child, src)
		case "variable_declaration":
			for j := 0; j < int(child.NamedChildCount()); j++ {
				varChild := child.NamedChild(j)
				switch varChild.Type() {
				case "simple_identifier":
					prop.Name = nodeText(varChild, src)
				case "user_type":
					prop.TypeName = extractTypeName(varChild, src)
				}
			}
		case "call_expression":
			// Initializer like `FakeDatabase()` or `TestUserRepository()` —
			// extract type from call name when no explicit type annotation.
			prop.HasInitializer = true
			callText := nodeText(child, src)
			prop.InitText = initTextPrefix(callText)
			if prop.TypeName == "" {
				for j := 0; j < int(child.NamedChildCount()); j++ {
					callChild := child.NamedChild(j)
					if callChild.Type() == "simple_identifier" {
						prop.TypeName = nodeText(callChild, src)
						break
					}
				}
			}
		default:
			// Any other named child after modifiers/variable_declaration is an
			// initializer expression (navigation_expression, literals, etc.)
			if child.Type() != "property_delegate" && child.Type() != "getter" && child.Type() != "setter" {
				prop.HasInitializer = true
				text := nodeText(child, src)
				prop.InitText = initTextPrefix(text)
			}
		}
	}

	return prop
}

func extractAnnotations(modifiers *sitter.Node, src []byte) []model.Annotation {
	var annotations []model.Annotation
	for i := 0; i < int(modifiers.NamedChildCount()); i++ {
		child := modifiers.NamedChild(i)
		if child.Type() == "annotation" {
			ann := model.Annotation{
				Line: int(child.StartPoint().Row) + 1,
			}
			for j := 0; j < int(child.NamedChildCount()); j++ {
				annChild := child.NamedChild(j)
				switch annChild.Type() {
				case "user_type":
					ann.Name = extractTypeName(annChild, src)
				case "constructor_invocation":
					// @Ignore("reason") or @Test(expected = ...) parses as constructor_invocation.
					// The constructor_invocation has user_type and value_arguments children.
					for k := 0; k < int(annChild.NamedChildCount()); k++ {
						ciChild := annChild.NamedChild(k)
						switch ciChild.Type() {
						case "user_type":
							ann.Name = extractTypeName(ciChild, src)
						case "value_arguments":
							ann.Args = nodeText(ciChild, src)
						}
					}
				case "value_arguments":
					ann.Args = nodeText(annChild, src)
				}
			}
			// Text-based fallback: if tree-sitter didn't extract a name,
			// check the raw annotation text for known patterns.
			if ann.Name == "" {
				annText := nodeText(child, src)
				if strings.HasPrefix(annText, "@Ignore") {
					ann.Name = "Ignore"
				} else if strings.HasPrefix(annText, "@Disabled") {
					ann.Name = "Disabled"
				} else if strings.HasPrefix(annText, "@Test") {
					ann.Name = "Test"
				}
			}
			annotations = append(annotations, ann)
		}
	}
	return annotations
}

func extractFunctionAnnotations(funcNode *sitter.Node, src []byte) []model.Annotation {
	for i := 0; i < int(funcNode.NamedChildCount()); i++ {
		child := funcNode.NamedChild(i)
		if child.Type() == "modifiers" {
			return extractAnnotations(child, src)
		}
	}
	return nil
}

// testAnnotationNames are annotation names that mark a method as a test.
var testAnnotationNames = map[string]bool{
	"Test":              true,
	"ParameterizedTest": true,
	"RepeatedTest":      true,
}

func isTestMethod(annotations []model.Annotation) bool {
	for _, ann := range annotations {
		if testAnnotationNames[ann.Name] {
			return true
		}
	}
	return false
}

func isSetupMethod(annotations []model.Annotation) bool {
	for _, ann := range annotations {
		if setupAnnotations[ann.Name] {
			return true
		}
	}
	return false
}

func extractMethod(node *sitter.Node, src []byte, annotations []model.Annotation) model.TestMethod {
	method := model.TestMethod{
		Annotations: annotations,
		LineStart:   int(node.StartPoint().Row) + 1,
		LineEnd:     int(node.EndPoint().Row) + 1,
	}

	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		switch child.Type() {
		case "simple_identifier":
			method.Name = stripBackticks(nodeText(child, src))
		case "function_body":
			method.Statements = extractStatements(child, src)
		}
	}

	return method
}

func extractStatements(funcBody *sitter.Node, src []byte) []model.ASTNode {
	var stmts []model.ASTNode
	for i := 0; i < int(funcBody.NamedChildCount()); i++ {
		child := funcBody.NamedChild(i)
		if child.Type() == "statements" {
			// Block body: fun test() { stmt1; stmt2 }
			for j := 0; j < int(child.NamedChildCount()); j++ {
				stmt := child.NamedChild(j)
				stmts = append(stmts, model.ASTNode{Node: stmt, Source: src})
			}
			return stmts
		}
	}

	// Expression body: fun test() = runTest { ... } or fun test() = expr
	// Try to unwrap lambda body from calls like runTest { }, runBlocking { }, etc.
	for i := 0; i < int(funcBody.NamedChildCount()); i++ {
		child := funcBody.NamedChild(i)
		if lambdaStmts := extractStatementsFromExpression(child, src); len(lambdaStmts) > 0 {
			return lambdaStmts
		}
		// If it's a single expression that's not a lambda-wrapping call, treat the whole thing as one statement
		stmts = append(stmts, model.ASTNode{Node: child, Source: src})
	}
	return stmts
}

// extractStatementsFromExpression recursively unwraps call expressions with trailing lambdas
// to find the actual statements inside. Handles: runTest { ... }, runBlocking { ... }, etc.
func extractStatementsFromExpression(node *sitter.Node, src []byte) []model.ASTNode {
	if node.Type() == "call_expression" {
		// Look for a trailing lambda in the call_suffix
		for i := 0; i < int(node.NamedChildCount()); i++ {
			child := node.NamedChild(i)
			if child.Type() == "call_suffix" {
				return extractStatementsFromCallSuffix(child, src)
			}
		}
	}
	return nil
}

func extractStatementsFromCallSuffix(callSuffix *sitter.Node, src []byte) []model.ASTNode {
	for i := 0; i < int(callSuffix.NamedChildCount()); i++ {
		child := callSuffix.NamedChild(i)
		if child.Type() == "annotated_lambda" || child.Type() == "lambda_literal" {
			return extractStatementsFromLambda(child, src)
		}
	}
	return nil
}

func extractStatementsFromLambda(node *sitter.Node, src []byte) []model.ASTNode {
	lambda := node
	// If annotated_lambda, find the lambda_literal inside
	if node.Type() == "annotated_lambda" {
		for i := 0; i < int(node.NamedChildCount()); i++ {
			child := node.NamedChild(i)
			if child.Type() == "lambda_literal" {
				lambda = child
				break
			}
		}
	}

	// lambda_literal contains statements
	var stmts []model.ASTNode
	for i := 0; i < int(lambda.NamedChildCount()); i++ {
		child := lambda.NamedChild(i)
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

func extractFunctionBodyNode(funcNode *sitter.Node, src []byte) *model.ASTNode {
	for i := 0; i < int(funcNode.NamedChildCount()); i++ {
		child := funcNode.NamedChild(i)
		if child.Type() == "function_body" {
			return &model.ASTNode{Node: child, Source: src}
		}
	}
	return nil
}

// initTextPrefix returns a short prefix of the initializer text for use in
// doubles detection (e.g., "mockk<", "mockk(", "mock(", "spyk(").
func initTextPrefix(text string) string {
	if len(text) > 40 {
		return text[:40]
	}
	return text
}

func extractTypeName(userType *sitter.Node, src []byte) string {
	for i := 0; i < int(userType.NamedChildCount()); i++ {
		child := userType.NamedChild(i)
		if child.Type() == "type_identifier" {
			return nodeText(child, src)
		}
	}
	return nodeText(userType, src)
}

func nodeText(node *sitter.Node, src []byte) string {
	return string(src[node.StartByte():node.EndByte()])
}

func stripBackticks(s string) string {
	return strings.Trim(s, "`")
}
