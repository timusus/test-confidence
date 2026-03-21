package parse

import (
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/timusus/test-confidence/internal/model"
)

// ParseProductionFile parses a production (non-test) file and extracts method metadata.
// It dispatches to the appropriate language-specific parser based on lang.
func ParseProductionFile(path string, src []byte, lang model.Language) (*model.ParsedProductionFile, error) {
	if lang == model.Swift {
		return ParseSwiftProductionFile(path, src)
	}
	return parseKotlinProductionFile(path, src, lang)
}

// parseKotlinProductionFile parses a production (non-test) Kotlin file and extracts method metadata.
func parseKotlinProductionFile(path string, src []byte, lang model.Language) (*model.ParsedProductionFile, error) {
	tree, err := ParseKotlin(src)
	if err != nil {
		return nil, err
	}

	root := tree.RootNode()

	result := &model.ParsedProductionFile{
		Path:     path,
		Language: lang,
		Source:   src,
		Tree:     tree,
	}

	// Find all class declarations and extract their methods.
	for i := 0; i < int(root.NamedChildCount()); i++ {
		child := root.NamedChild(i)
		if child.Type() == "class_declaration" {
			extractProductionMethods(child, src, result)
		}
	}

	return result, nil
}

func extractProductionMethods(classNode *sitter.Node, src []byte, result *model.ParsedProductionFile) {
	for i := 0; i < int(classNode.NamedChildCount()); i++ {
		child := classNode.NamedChild(i)
		if child.Type() == "class_body" {
			for j := 0; j < int(child.NamedChildCount()); j++ {
				member := child.NamedChild(j)
				if member.Type() == "function_declaration" {
					method := extractProductionMethod(member, src)
					result.Methods = append(result.Methods, method)
				}
			}
		}
	}
}

func extractProductionMethod(funcNode *sitter.Node, src []byte) model.ProductionMethod {
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
			method.IsSingleExpr = isSingleExpressionBody(child)
			method.BranchCount = countBranches(child, src)
			method.CallCount = countCalls(child, src)
		}
	}

	return method
}

// isSingleExpressionBody checks if a function_body uses `= expr` syntax
// (no block/statements child, just a direct expression).
func isSingleExpressionBody(funcBody *sitter.Node) bool {
	// Expression body: function_body has NO "statements" child.
	// It starts with "=" and contains a single expression directly.
	for i := 0; i < int(funcBody.NamedChildCount()); i++ {
		child := funcBody.NamedChild(i)
		if child.Type() == "statements" {
			return false
		}
	}
	// If there's at least one named child (the expression) and no statements, it's a single expression.
	return funcBody.NamedChildCount() > 0
}

// countBranches counts branch nodes: if_expression, when_expression, elvis_expression.
func countBranches(node *sitter.Node, src []byte) int {
	count := 0
	switch node.Type() {
	case "if_expression", "when_expression", "elvis_expression":
		count++
	}
	for i := 0; i < int(node.NamedChildCount()); i++ {
		count += countBranches(node.NamedChild(i), src)
	}
	return count
}

// countCalls counts call_expression nodes in the body.
func countCalls(node *sitter.Node, src []byte) int {
	count := 0
	if node.Type() == "call_expression" {
		count++
	}
	for i := 0; i < int(node.NamedChildCount()); i++ {
		count += countCalls(node.NamedChild(i), src)
	}
	return count
}
