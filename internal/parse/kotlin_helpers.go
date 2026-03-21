package parse

import (
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/timusus/test-confidence/internal/model"
)

// verifyNames are function names that indicate a verify (interaction assertion) call.
var verifyNames = map[string]bool{
	"verify":   true,
	"coVerify": true,
}

// stubNames are function names that indicate a stubbing call.
var stubNames = map[string]bool{
	"every":   true,
	"coEvery": true,
}

// assertionPrefixes are prefixes for assertion function names.
var assertionPrefixes = []string{
	"assert",
	"should",
	"expect",
	"XCTAssert",
	"XCTFail",
}

// IsVerifyCall returns true if the node is a MockK verify/coVerify or Mockito.verify() call.
// MockK: verify { repo.save(user) } — top-level call_expression with name "verify" or "coVerify"
// Mockito: Mockito.verify(mock).method() — call_expression chain
func IsVerifyCall(node model.ASTNode) bool {
	// Swift: Mockable verify(mock)...
	if IsSwiftVerifyCall(node) {
		return true
	}

	name := callName(node)
	if verifyNames[name] {
		return true
	}
	// Mockito: navigation_expression with "verify" as method on "Mockito"
	if node.Type() == "call_expression" {
		for i := 0; i < int(node.Node.NamedChildCount()); i++ {
			child := node.Node.NamedChild(i)
			if child.Type() == "navigation_expression" {
				navText := nodeText(child, node.Source)
				if strings.Contains(navText, "Mockito.verify") {
					return true
				}
			}
		}
	}
	return false
}

// VerifyHasContentAssertion returns true if a verify call's lambda contains `withArg`
// blocks that have real assertions inside them. This distinguishes:
//   - verify { repo.save(any()) }                                    → no content assertion
//   - verify { withArg<T> { assertThat(it.field).isEqualTo(x) } }   → has content assertion
func VerifyHasContentAssertion(node model.ASTNode) bool {
	if !IsVerifyCall(node) {
		return false
	}
	// Find the lambda inside the verify call and search for withArg + assertions
	return nodeContainsWithArgAssertion(node.Node, node.Source)
}

func nodeContainsWithArgAssertion(node *sitter.Node, src []byte) bool {
	text := nodeText(node, src)
	// Quick text checks for content assertions inside verify blocks:
	// - MockK: withArg<T> { assertThat(it.field).isEqualTo(x) }
	// - Mockable: .matching { ... } closures with content checks
	// - Mockable: .value(specificValue) specific value matching
	if strings.Contains(text, ".matching(") || strings.Contains(text, ".matching {") {
		return true
	}
	if strings.Contains(text, ".value(") {
		return true
	}
	if !strings.Contains(text, "withArg") {
		return false
	}
	// Walk recursively looking for withArg calls that contain assertions
	return walkForWithArgAssertion(node, src)
}

func walkForWithArgAssertion(node *sitter.Node, src []byte) bool {
	if node.Type() == "call_expression" {
		n := model.ASTNode{Node: node, Source: src}
		name := callName(n)
		if name == "withArg" {
			// Check if this withArg's lambda contains assertion calls
			text := nodeText(node, src)
			for _, prefix := range assertionPrefixes {
				if strings.Contains(text, prefix) {
					return true
				}
			}
		}
	}
	for i := 0; i < int(node.NamedChildCount()); i++ {
		if walkForWithArgAssertion(node.NamedChild(i), src) {
			return true
		}
	}
	return false
}

// IsStubCall returns true if the node is a MockK every/coEvery or Mockito whenever/given call.
// MockK: every { repo.getUser() } returns user — infix_expression with every/coEvery as left call
// Mockito: whenever(mock.method()).thenReturn(value) — call_expression chain
func IsStubCall(node model.ASTNode) bool {
	// Swift: Mockable given(mock)...willReturn(...)
	if IsSwiftStubCall(node) {
		return true
	}

	// MockK stub: infix_expression where left side is a call_expression named every/coEvery
	if node.Type() == "infix_expression" {
		for i := 0; i < int(node.Node.NamedChildCount()); i++ {
			child := node.Node.NamedChild(i)
			if child.Type() == "call_expression" {
				childNode := model.ASTNode{Node: child, Source: node.Source}
				name := callName(childNode)
				if stubNames[name] {
					return true
				}
			}
		}
	}
	// Mockito: whenever(...).thenReturn(...)
	if node.Type() == "call_expression" {
		name := deepCallName(node)
		if name == "whenever" || name == "given" {
			return true
		}
	}
	return false
}

// IsAssertionCall returns true if the node is any assertion call (assert*, assertThat, shouldBe, expect).
// NOT verify calls — those are interaction assertions handled separately.
// Handles both:
//   - Standard: assertThat(result).isEqualTo(expected) — root name is "assertThat"
//   - Compose:  composeTestRule.onNodeWithText("...").assertIsDisplayed() — terminal name is "assertIsDisplayed"
func IsAssertionCall(node model.ASTNode) bool {
	if IsVerifyCall(node) {
		return false
	}

	// Swift: #expect(...), #require(...), XCTest assertions
	if IsSwiftAssertionCall(node) {
		return true
	}

	// Kotest infix assertions: `result shouldBe expected`, `result shouldNotBe null`
	// These produce infix_expression nodes, not call_expression nodes.
	if node.Type() == "infix_expression" {
		infixOp := infixOperatorName(node)
		if isAssertionName(infixOp) {
			return true
		}
	}

	// Check root call name (handles assertThat, assertEquals, etc.)
	rootName := deepCallName(node)
	if isAssertionName(rootName) {
		return true
	}

	// Check terminal method name in chained calls (handles Compose test assertions)
	// e.g., composeTestRule.onNodeWithText("...").assertIsDisplayed()
	termName := terminalCallName(node)
	if termName != rootName && isAssertionName(termName) {
		return true
	}

	return false
}

// infixOperatorName extracts the infix operator name from an infix_expression.
// For `result shouldBe expected`, returns "shouldBe".
// Tree-sitter represents this as 3 named children, all simple_identifier:
// [0]=left ("result"), [1]=operator ("shouldBe"), [2]=right ("expected").
// The operator is always the second simple_identifier child.
func infixOperatorName(node model.ASTNode) string {
	identCount := 0
	for i := 0; i < int(node.Node.NamedChildCount()); i++ {
		child := node.Node.NamedChild(i)
		if child.Type() == "simple_identifier" {
			identCount++
			if identCount == 2 {
				return nodeText(child, node.Source)
			}
		}
	}
	return ""
}

// isAssertionName checks if a function name matches any assertion prefix.
func isAssertionName(name string) bool {
	if name == "" {
		return false
	}
	for _, prefix := range assertionPrefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

// terminalCallName returns the last method name in a chained call expression.
// For composeTestRule.onNodeWithText("...").assertIsDisplayed(), returns "assertIsDisplayed".
// For simple calls like assertEquals(a, b), returns "assertEquals".
func terminalCallName(node model.ASTNode) string {
	if node.Type() != "call_expression" {
		return ""
	}

	// Walk the AST to find the rightmost navigation_suffix with a simple_identifier.
	// In a chain like a.b().c().d(), the tree-sitter structure nests navigation_expressions
	// and the terminal method name is in the outermost call_expression's navigation_suffix
	// or in a navigation_expression child.
	var lastIdent string
	walkForTerminalName(node.Node, node.Source, &lastIdent)
	if lastIdent != "" {
		return lastIdent
	}
	return callExprName(node)
}

// walkForTerminalName recursively walks the AST looking for navigation_suffix
// nodes, recording the last simple_identifier found (which is the terminal method name).
func walkForTerminalName(node *sitter.Node, src []byte, lastIdent *string) {
	if node.Type() == "navigation_suffix" {
		for i := 0; i < int(node.NamedChildCount()); i++ {
			child := node.NamedChild(i)
			if child.Type() == "simple_identifier" {
				*lastIdent = nodeText(child, src)
			}
		}
	}
	for i := 0; i < int(node.NamedChildCount()); i++ {
		walkForTerminalName(node.NamedChild(i), src, lastIdent)
	}
}

// ExtractCallTarget extracts the receiver and method name from a call inside verify/every blocks.
// MockK:    verify { repo.save(user) } → ("repo", "save")
// MockK:    every { repo.getUser() } returns user → ("repo", "getUser")
// Mockable: verify(mock).method() → ("mock", "method")
// Mockable: given(mock).method().willReturn(value) → ("mock", "method")
// Cuckoo:   stub(mock) { ... } → ("mock", "")
func ExtractCallTarget(node model.ASTNode) (receiver, method string) {
	// Find the lambda inside the verify/every/coVerify/coEvery call.
	var callExpr model.ASTNode

	switch node.Type() {
	case "call_expression":
		callExpr = node
	case "infix_expression":
		// Left side is the call_expression (every { ... })
		if node.Node.NamedChildCount() > 0 {
			first := node.Node.NamedChild(0)
			if first.Type() == "call_expression" {
				callExpr = model.ASTNode{Node: first, Source: node.Source}
			}
		}
	default:
		// Fallback: try text-based extraction for chained calls
		// e.g., verify(mock).method().called(count: .once)
		return extractMockableTarget(node)
	}

	// Try lambda-based extraction first (MockK pattern)
	// Guard: callExpr may be zero-value if the AST structure was unexpected
	// (e.g., Mockito-Kotlin infix_expression where first child isn't a call_expression).
	if callExpr.Node == nil {
		return "", ""
	}
	lambda := findLambda(callExpr)
	if lambda != nil {
		lambdaNode := model.ASTNode{Node: lambda, Source: node.Source}
		return extractNavCallTarget(lambdaNode)
	}

	// No lambda — try Mockable/Cuckoo pattern: verify(mock).method()
	// Extract the first argument of the call as the mock variable
	return extractMockableTarget(callExpr)
}

// extractMockableTarget extracts the mock variable from patterns like:
// verify(mock).method() → ("mock", "")
// given(mock).method().willReturn(value) → ("mock", "")
func extractMockableTarget(node model.ASTNode) (string, string) {
	text := nodeText(node.Node, node.Source)
	// Look for verify(X) or given(X) or stub(X) pattern
	for _, prefix := range []string{"verify(", "given(", "stub("} {
		idx := strings.Index(text, prefix)
		if idx < 0 {
			continue
		}
		// Extract the argument inside the parens
		start := idx + len(prefix)
		depth := 1
		end := start
		for end < len(text) && depth > 0 {
			if text[end] == '(' {
				depth++
			} else if text[end] == ')' {
				depth--
			}
			if depth > 0 {
				end++
			}
		}
		arg := strings.TrimSpace(text[start:end])
		// Simple variable name (no dots, no parens)
		if len(arg) > 0 && !strings.ContainsAny(arg, ".( ") {
			return arg, ""
		}
	}
	return "", ""
}

// ExtractReturnValue extracts the identifier used as the return value in a stub.
// every { repo.getUser() } returns user → "user"
// every { repo.getUser() } returns listOf(testUser) → "" (not a simple identifier)
func ExtractReturnValue(node model.ASTNode) string {
	if node.Type() != "infix_expression" {
		return ""
	}
	// The infix_expression has: call_expression, "returns" identifier, value identifier
	// Structure: [call_expression, simple_identifier("returns"), simple_identifier("user")]
	count := int(node.Node.NamedChildCount())
	if count < 3 {
		return ""
	}
	last := node.Node.NamedChild(count - 1)
	if last.Type() == "simple_identifier" {
		return nodeText(last, node.Source)
	}
	return ""
}

// ExtractAssertedValues extracts the actual and expected identifiers from an assertion.
// assertThat(result).isEqualTo(expected) → ("result", "expected")
// assertEquals(expected, actual) → ("actual", "expected")
func ExtractAssertedValues(node model.ASTNode) (actual, expected string) {
	if node.Type() != "call_expression" {
		return "", ""
	}

	name := deepCallName(node)

	switch {
	case name == "assertThat":
		// assertThat(result).isEqualTo(expected)
		// Structure: call_expression { navigation_expression { call_expression("assertThat", args=(result)), ".isEqualTo" }, call_suffix(args=(expected)) }
		actual = extractFirstArg(findDeepestCall(node))
		expected = extractFirstArg(node)
		return actual, expected

	case name == "assertEquals":
		// assertEquals(expected, actual) — JUnit convention: expected first, actual second
		args := extractArgs(node)
		if len(args) >= 2 {
			return args[1], args[0]
		}
	}

	return "", ""
}

// HasParentOfType checks if any ancestor of the node has the given call name.
// Used for: is this delay() inside a runTest {} block?
func HasParentOfType(node model.ASTNode, callName string) bool {
	current := node.Node.Parent()
	for current != nil {
		if current.Type() == "call_expression" {
			n := model.ASTNode{Node: current, Source: node.Source}
			if cn := callExprName(n); cn == callName {
				return true
			}
		}
		current = current.Parent()
	}
	return false
}

// callName returns the simple_identifier name of a call_expression, or "".
func callName(node model.ASTNode) string {
	if node.Type() != "call_expression" {
		return ""
	}
	return callExprName(node)
}

// callExprName extracts the direct simple_identifier child of a call_expression.
func callExprName(node model.ASTNode) string {
	for i := 0; i < int(node.Node.NamedChildCount()); i++ {
		child := node.Node.NamedChild(i)
		if child.Type() == "simple_identifier" {
			return nodeText(child, node.Source)
		}
	}
	return ""
}

// deepCallName finds the root call name in a chained call expression.
// For assertThat(result).isEqualTo(expected), it returns "assertThat".
// For simple calls like verify { }, it returns "verify".
func deepCallName(node model.ASTNode) string {
	if node.Type() != "call_expression" {
		return ""
	}
	// Check for direct simple_identifier
	name := callExprName(node)
	if name != "" {
		return name
	}
	// Check for navigation_expression chain: dig into the leftmost call
	for i := 0; i < int(node.Node.NamedChildCount()); i++ {
		child := node.Node.NamedChild(i)
		if child.Type() == "navigation_expression" {
			return deepNavCallName(child, node.Source)
		}
	}
	return ""
}

// deepNavCallName walks a navigation_expression to find the root call name.
func deepNavCallName(nav *sitter.Node, src []byte) string {
	for i := 0; i < int(nav.NamedChildCount()); i++ {
		child := nav.NamedChild(i)
		if child.Type() == "call_expression" {
			n := model.ASTNode{Node: child, Source: src}
			return deepCallName(n)
		}
		if child.Type() == "simple_identifier" {
			return nodeText(child, src)
		}
	}
	return ""
}

// findLambda finds the lambda_literal node inside a call_expression.
func findLambda(node model.ASTNode) *sitter.Node {
	for i := 0; i < int(node.Node.NamedChildCount()); i++ {
		child := node.Node.NamedChild(i)
		if child.Type() == "call_suffix" {
			for j := 0; j < int(child.NamedChildCount()); j++ {
				suffixChild := child.NamedChild(j)
				if suffixChild.Type() == "annotated_lambda" {
					for k := 0; k < int(suffixChild.NamedChildCount()); k++ {
						lambdaChild := suffixChild.NamedChild(k)
						if lambdaChild.Type() == "lambda_literal" {
							return lambdaChild
						}
					}
				}
			}
		}
	}
	return nil
}

// extractNavCallTarget walks into a lambda body and finds the navigation call target.
func extractNavCallTarget(lambdaNode model.ASTNode) (receiver, method string) {
	// Walk into statements
	for i := 0; i < int(lambdaNode.Node.NamedChildCount()); i++ {
		child := lambdaNode.Node.NamedChild(i)
		if child.Type() == "statements" {
			for j := 0; j < int(child.NamedChildCount()); j++ {
				stmt := child.NamedChild(j)
				if stmt.Type() == "call_expression" {
					return extractNavFromCall(stmt, lambdaNode.Source)
				}
			}
		}
	}
	return "", ""
}

// extractNavFromCall extracts receiver.method from a call_expression with a navigation_expression.
func extractNavFromCall(callNode *sitter.Node, src []byte) (receiver, method string) {
	for i := 0; i < int(callNode.NamedChildCount()); i++ {
		child := callNode.NamedChild(i)
		if child.Type() == "navigation_expression" {
			parts := extractNavParts(child, src)
			if len(parts) >= 2 {
				return parts[0], parts[1]
			}
		}
	}
	return "", ""
}

// extractNavParts extracts [receiver, method] from a navigation_expression.
func extractNavParts(nav *sitter.Node, src []byte) []string {
	var parts []string
	for i := 0; i < int(nav.NamedChildCount()); i++ {
		child := nav.NamedChild(i)
		switch child.Type() {
		case "simple_identifier":
			parts = append(parts, nodeText(child, src))
		case "navigation_suffix":
			for j := 0; j < int(child.NamedChildCount()); j++ {
				suffChild := child.NamedChild(j)
				if suffChild.Type() == "simple_identifier" {
					parts = append(parts, nodeText(suffChild, src))
				}
			}
		}
	}
	return parts
}

// extractFirstArg extracts the first argument identifier from a call_expression.
func extractFirstArg(node model.ASTNode) string {
	args := extractArgs(node)
	if len(args) > 0 {
		return args[0]
	}
	return ""
}

// extractArgs extracts argument identifier names from a call_expression's value_arguments.
func extractArgs(node model.ASTNode) []string {
	if node.Type() != "call_expression" {
		return nil
	}
	for i := 0; i < int(node.Node.NamedChildCount()); i++ {
		child := node.Node.NamedChild(i)
		if child.Type() == "call_suffix" {
			for j := 0; j < int(child.NamedChildCount()); j++ {
				suffixChild := child.NamedChild(j)
				if suffixChild.Type() == "value_arguments" {
					return extractArgValues(suffixChild, node.Source)
				}
			}
		}
	}
	return nil
}

// extractArgValues extracts simple identifier values from value_arguments.
func extractArgValues(args *sitter.Node, src []byte) []string {
	var result []string
	for i := 0; i < int(args.NamedChildCount()); i++ {
		child := args.NamedChild(i)
		if child.Type() == "value_argument" {
			for j := 0; j < int(child.NamedChildCount()); j++ {
				argChild := child.NamedChild(j)
				if argChild.Type() == "simple_identifier" {
					result = append(result, nodeText(argChild, src))
				}
			}
		}
	}
	return result
}

// findDeepestCall walks through navigation_expression chains to find the innermost call_expression.
// For assertThat(result).isEqualTo(expected), it returns the assertThat(result) call_expression.
func findDeepestCall(node model.ASTNode) model.ASTNode {
	if node.Type() != "call_expression" {
		return node
	}
	for i := 0; i < int(node.Node.NamedChildCount()); i++ {
		child := node.Node.NamedChild(i)
		if child.Type() == "navigation_expression" {
			// Find the call_expression inside the navigation_expression
			for j := 0; j < int(child.NamedChildCount()); j++ {
				navChild := child.NamedChild(j)
				if navChild.Type() == "call_expression" {
					return findDeepestCall(model.ASTNode{Node: navChild, Source: node.Source})
				}
			}
		}
	}
	return node
}
