package parse

import (
	"strings"

	"github.com/timusus/test-confidence/internal/model"
	ios "github.com/timusus/test-confidence/internal/platform/ios"
)

// xcAssertionNames maps XCTest assertion function names for quick lookup.
var xcAssertionNames = map[string]bool{
	"XCTAssertEqual":         true,
	"XCTAssertNotEqual":      true,
	"XCTAssertIdentical":     true,
	"XCTAssertNotIdentical":  true,
	"XCTAssertGreaterThan":   true,
	"XCTAssertLessThan":      true,
	"XCTAssertGreaterThanOrEqual": true,
	"XCTAssertLessThanOrEqual":    true,
	"XCTAssertTrue":          true,
	"XCTAssertFalse":         true,
	"XCTAssertNil":           true,
	"XCTAssertNotNil":        true,
	"XCTAssertThrowsError":   true,
	"XCTAssertNoThrow":       true,
	"XCTAssert":              true,
	"XCTFail":                true,
	"XCTUnwrap":              true,
}

// xcExceptionAssertions are exception-target assertions.
var xcExceptionAssertions = map[string]bool{
	"XCTAssertThrowsError": true,
	"XCTAssertNoThrow":     true,
}

// xcStrongAssertions are strong (exact) assertions.
var xcStrongAssertions map[string]bool

// xcMediumAssertions are medium (boolean) assertions.
var xcMediumAssertions map[string]bool

// xcWeakAssertions are weak (nil) assertions.
var xcWeakAssertions map[string]bool

func init() {
	xcStrongAssertions = map[string]bool{}
	for _, name := range ios.StrongAssertions {
		xcStrongAssertions[name] = true
	}
	xcMediumAssertions = map[string]bool{}
	for _, name := range ios.MediumAssertions {
		xcMediumAssertions[name] = true
	}
	xcWeakAssertions = map[string]bool{}
	for _, name := range ios.WeakAssertions {
		xcWeakAssertions[name] = true
	}
}

// IsSwiftAssertionCall returns true if the node is an XCTest, Swift Testing, or Nimble assertion call.
func IsSwiftAssertionCall(node model.ASTNode) bool {
	text := nodeText(node.Node, node.Source)
	trimmed := strings.TrimSpace(text)
	// Swift Testing: #expect(...) and #require(...)
	// After pre-processing, these appear as __expect(...)/__require(...) in the AST.
	// Also handle the original forms for text-based matching.
	if strings.HasPrefix(trimmed, "#expect(") || strings.HasPrefix(trimmed, "#require(") ||
		strings.HasPrefix(trimmed, "__expect(") || strings.HasPrefix(trimmed, "__require(") {
		return true
	}
	// Nimble: expect(x).to(...) — only match when the node starts with expect(
	if strings.HasPrefix(trimmed, "expect(") && isNimbleAssertion(text) {
		return true
	}
	if node.Type() != "call_expression" {
		return false
	}
	name := swiftCallExprName(node.Node, node.Source)
	if xcAssertionNames[name] || name == "__expect" || name == "__require" {
		return true
	}
	return false
}

// IsSwiftVerifyCall returns true if the node looks like a mock verification in Swift.
// Handles Mockable framework `verify(mock)...` calls and Cuckoo `verify(mock)` calls.
func IsSwiftVerifyCall(node model.ASTNode) bool {
	text := nodeText(node.Node, node.Source)
	// Check the first line for verify( — handles leading whitespace and chained expressions
	firstLine := firstLineOf(text)
	if strings.Contains(firstLine, "verify(") {
		return true
	}
	return false
}

// IsSwiftStubCall returns true if the node looks like a stub setup in Swift.
// Handles Mockable framework `given(mock)...willReturn(...)` and Cuckoo `stub(mock)` calls.
func IsSwiftStubCall(node model.ASTNode) bool {
	text := nodeText(node.Node, node.Source)
	firstLine := firstLineOf(text)
	// Mockable framework: given(mock).method().willReturn(value)
	if strings.Contains(firstLine, "given(") && strings.Contains(text, "willReturn") {
		return true
	}
	// Cuckoo framework: stub(mock) { ... }
	if strings.Contains(firstLine, "stub(") {
		return true
	}
	return false
}

// ClassifySwiftAssertion classifies an XCTest/Swift Testing/Nimble assertion by strength and target.
func ClassifySwiftAssertion(node model.ASTNode) (model.Strength, model.Target) {
	text := nodeText(node.Node, node.Source)

	// Swift Testing: #require/__require is always strong (force-unwrap assertion)
	if strings.Contains(text, "#require(") || strings.Contains(text, "__require(") {
		if strings.Contains(text, "throws:") {
			return model.Strong, model.Exception
		}
		return model.Strong, model.Output
	}
	// Swift Testing: #expect/__expect — strength depends on the expression inside
	if strings.Contains(text, "#expect(") || strings.Contains(text, "__expect(") {
		return classifyExpectStrength(text), classifyExpectTarget(text)
	}

	// Nimble: classify by matcher name
	if isNimbleAssertion(text) {
		strength := classifyNimbleStrength(text)
		return strength, model.Output
	}

	name := swiftCallExprName(node.Node, node.Source)

	if xcExceptionAssertions[name] {
		return model.Strong, model.Exception
	}

	strength := classifySwiftAssertionStrength(name)
	return strength, model.Output
}

// classifySwiftAssertionStrength maps assertion names to strength levels.
func classifySwiftAssertionStrength(name string) model.Strength {
	if xcStrongAssertions[name] {
		return model.Strong
	}
	if xcMediumAssertions[name] {
		return model.Medium
	}
	if xcWeakAssertions[name] {
		return model.Weak
	}
	// XCTAssert (bare) is medium, XCTFail is medium, XCTUnwrap is weak
	if name == "XCTUnwrap" {
		return model.Weak
	}
	return model.Medium
}

// IsSwiftConditionalNode returns true if the node type is a Swift conditional statement.
func IsSwiftConditionalNode(nodeType string) bool {
	return nodeType == "if_statement" || nodeType == "switch_statement" || nodeType == "guard_statement"
}

// HasSwiftClosureAncestor checks if a node is nested inside a closure_expression.
func HasSwiftClosureAncestor(node model.ASTNode) bool {
	current := node.Node.Parent()
	for current != nil {
		if current.Type() == "closure_expression" {
			return true
		}
		current = current.Parent()
	}
	return false
}

// IsSwiftManualMockProperty checks if a property name matches manual mock tracking patterns
// (e.g., saveCalled, saveCallCount, lastArgument).
func IsSwiftManualMockProperty(propName string) bool {
	for _, suffix := range ios.MockPropertySuffixes {
		if strings.HasSuffix(propName, suffix) {
			return true
		}
	}
	return false
}

// ContainsSwiftAssertionText checks if text contains assertion patterns,
// used as a fallback for nested assertions the AST walker might miss.
func ContainsSwiftAssertionText(text string) bool {
	// Swift Testing
	if strings.Contains(text, "#expect(") || strings.Contains(text, "#require(") {
		return true
	}
	// XCTest
	for name := range xcAssertionNames {
		if strings.Contains(text, name+"(") {
			return true
		}
	}
	// Nimble
	if isNimbleAssertion(text) {
		return true
	}
	return false
}

// isNimbleAssertion returns true if text contains a Nimble expect(...).to/toNot/notTo/toEventually pattern.
func isNimbleAssertion(text string) bool {
	if !strings.Contains(text, "expect(") {
		return false
	}
	return strings.Contains(text, ".to(") ||
		strings.Contains(text, ".toNot(") ||
		strings.Contains(text, ".notTo(") ||
		strings.Contains(text, ".toEventually(")
}

// nimbleStrongMatchers are Nimble matchers that produce strong (exact value) assertions.
var nimbleStrongMatchers = []string{"equal(", "contain(", "haveCount(", "beginWith(", "endWith("}

// nimbleMediumMatchers are Nimble matchers that produce medium (boolean/comparison) assertions.
var nimbleMediumMatchers = []string{"beTrue()", "beFalse()", "beGreaterThan(", "beLessThan(", "satisfy("}

// nimbleWeakMatchers are Nimble matchers that produce weak (nil/type) assertions.
var nimbleWeakMatchers = []string{"beNil()", "beEmpty()", "beAnInstanceOf("}

// classifyNimbleStrength classifies a Nimble assertion by the matcher used.
func classifyNimbleStrength(text string) model.Strength {
	for _, m := range nimbleStrongMatchers {
		if strings.Contains(text, m) {
			return model.Strong
		}
	}
	for _, m := range nimbleMediumMatchers {
		if strings.Contains(text, m) {
			return model.Medium
		}
	}
	for _, m := range nimbleWeakMatchers {
		if strings.Contains(text, m) {
			return model.Weak
		}
	}
	// Default to medium for unknown Nimble matchers
	return model.Medium
}

// classifyExpectTarget determines the target of an #expect() assertion.
func classifyExpectTarget(text string) model.Target {
	if strings.Contains(text, "throws:") || strings.Contains(text, "throws ") {
		return model.Exception
	}
	return model.Output
}

// classifyExpectStrength determines the strength of an #expect() assertion.
// #expect(x == y) or #expect(x != y) → Strong (equality)
// #expect(x > y) or #expect(x < y) etc. → Medium (comparison)
// #expect(boolExpr) → Medium (boolean check)
func classifyExpectStrength(text string) model.Strength {
	// Extract the content inside #expect(...)
	idx := strings.Index(text, "#expect(")
	if idx < 0 {
		return model.Medium
	}
	inner := text[idx+len("#expect("):]

	// Check for equality operators — strong assertions
	if strings.Contains(inner, " == ") || strings.Contains(inner, " != ") {
		return model.Strong
	}
	// Comparison operators — medium
	if strings.Contains(inner, " > ") || strings.Contains(inner, " < ") ||
		strings.Contains(inner, " >= ") || strings.Contains(inner, " <= ") {
		return model.Medium
	}
	// Everything else is a boolean expression — medium
	return model.Medium
}

// firstLineOf returns the first line of a string, trimmed of leading whitespace.
func firstLineOf(text string) string {
	text = strings.TrimLeft(text, " \t\n\r")
	if idx := strings.IndexByte(text, '\n'); idx >= 0 {
		return text[:idx]
	}
	return text
}
