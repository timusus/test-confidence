package analyze

import (
	"strings"

	"github.com/timusus/test-confidence/internal/model"
	"github.com/timusus/test-confidence/internal/parse"
)

// Strong assertion function names — exact equality, containment, structural checks.
var strongAssertions = map[string]bool{
	"assertEquals":      true,
	"isEqualTo":         true,
	"shouldBe":          true,
	"contains":          true,
	"hasSize":           true,
	"isEmpty":           true,
	"containsExactly":   true,
	"doesNotContain":    true, // Google Truth negated containment
	"containsNoneOf":    true,
	"containsAtLeast":   true,
	"isIn":              true,
	"is":                true, // Hamcrest matcher
	"equalTo":           true, // Hamcrest
	// XCTest strong assertions
	"XCTAssertEqual":            true,
	"XCTAssertNotEqual":         true,
	"XCTAssertIdentical":        true,
	"XCTAssertNotIdentical":     true,
	"XCTAssertGreaterThan":      true,
	"XCTAssertLessThan":         true,
	"XCTAssertGreaterThanOrEqual": true,
	"XCTAssertLessThanOrEqual":    true,
}

// Medium assertion function names — boolean conditions.
var mediumAssertions = map[string]bool{
	"assertTrue":         true,
	"assertFalse":        true,
	"assertIsDisplayed":  true,
	"assertExists":       true,
	// XCTest medium assertions
	"XCTAssertTrue":     true,
	"XCTAssertFalse":    true,
	"XCTAssert":         true,
	"XCTFail":           true,
}

// Weak assertion function names — null checks, emptiness checks.
var weakAssertions = map[string]bool{
	"assertNotNull": true,
	"assertNull":    true,
	"isNotNull":     true,
	"isNotEmpty":    true,
	"isNull":        true, // assertk
	// XCTest weak assertions
	"XCTAssertNil":    true,
	"XCTAssertNotNil": true,
	"XCTUnwrap":       true,
}

// Exception assertion function names.
var exceptionAssertions = map[string]bool{
	"assertThrows":    true,
	"shouldThrow":     true,
	"assertFailsWith": true,
	"assertFails":     true, // kotlin.test
	// XCTest exception assertions
	"XCTAssertThrowsError": true,
	"XCTAssertNoThrow":     true,
}

// AnalyzeAssertions classifies assertions in a parsed test file by strength and target.
// Verify calls are classified as Interaction target and are NOT counted in TotalAssertions.
func AnalyzeAssertions(file *model.ParsedTestFile) model.AssertionAnalysis {
	result := model.AssertionAnalysis{
		StrengthDistribution: make(map[model.Strength]int),
		TargetDistribution:   make(map[model.Target]int),
	}

	// Check if this file imports Compose test packages.
	isComposeTest := false
	for _, imp := range file.Imports {
		if strings.Contains(imp.Path, "androidx.compose.ui.test") ||
			strings.Contains(imp.Path, "compose.ui.test") {
			isComposeTest = true
			break
		}
	}
	if isComposeTest {
		result.ComposeTestFiles = 1
	}

	for _, cls := range file.Classes {
		for _, method := range cls.Methods {
			assertionCount := 0
			verifyCount := 0
			verifyWithContentCount := 0 // verify calls that contain content assertions (.matching, .value, withArg)
			unclassifiedCount := 0

			for _, stmt := range method.Statements {
				if parse.IsVerifyCall(stmt) {
					verifyCount++
					if parse.VerifyHasContentAssertion(stmt) {
						verifyWithContentCount++
					}
					result.TargetDistribution[model.Interaction]++
					continue
				}

				if parse.IsAssertionCall(stmt) {
					strength, target := classifyAssertion(stmt)
					result.TotalAssertions++
					result.TargetDistribution[target]++
					if target != model.Exception {
						result.StrengthDistribution[strength]++
					}
					assertionCount++
				} else if !parse.IsStubCall(stmt) {
					// Text-based fallback for assertions and verify calls nested in lambdas.
					// Catches: Turbine flow.test { shouldBe }, Compose .assertIsDisplayed(),
					// verify { } inside composeTestRule.runOnIdle { }, and any other
					// assertion/verify inside a nested block the AST walker misses.
					stmtText := stmt.Text()

					// Check for verify calls nested inside lambdas
					// (e.g., composeTestRule.runOnIdle { verify { ... } })
					verifyMatches := countVerifyMatches(stmtText)
					if verifyMatches > 0 {
						verifyCount += verifyMatches
						result.TargetDistribution[model.Interaction] += verifyMatches
					}

					// Count assertion occurrences individually with strength classification
					matchResult := countAssertionMatchesWithStrength(stmtText)
					if matchResult.total > 0 {
						result.TotalAssertions += matchResult.total
						for s, n := range matchResult.byStrength {
							result.StrengthDistribution[s] += n
						}
						result.TargetDistribution[model.Output] += matchResult.total
						assertionCount += matchResult.total
					} else if verifyMatches == 0 {
						unclassifiedCount++
					}
				}
			}

			result.UnclassifiedStatements += unclassifiedCount

			// Method-level text fallback: when statement-level analysis found 0 assertions,
			// scan the raw source of the method body for assertion patterns. This catches
			// cases where tree-sitter misparses constructs (e.g., Swift #expect is parsed
			// as ERROR + tuple_expression, losing the #expect text from the statement nodes).
			if assertionCount == 0 && verifyCount == 0 && file.Source != nil &&
				method.LineStart > 0 && method.LineEnd > method.LineStart {
				methodText := extractMethodBodyText(file.Source, method.LineStart, method.LineEnd)
				if methodText != "" {
					bodyResult := countAssertionMatchesWithStrength(methodText)
					if bodyResult.total > 0 {
						result.TotalAssertions += bodyResult.total
						for s, n := range bodyResult.byStrength {
							result.StrengthDistribution[s] += n
						}
						result.TargetDistribution[model.Output] += bodyResult.total
						assertionCount += bodyResult.total
						// Reduce unclassified count — these statements were real assertions
						result.UnclassifiedStatements -= unclassifiedCount
						if result.UnclassifiedStatements < 0 {
							result.UnclassifiedStatements = 0
						}
					}
					bodyVerify := countVerifyMatches(methodText)
					if bodyVerify > 0 {
						verifyCount += bodyVerify
						result.TargetDistribution[model.Interaction] += bodyVerify
					}
				}
			}

			// JUnit 4 @Test(expected = SomeException::class) counts as 1 exception assertion.
			if hasExpectedAnnotation(method) {
				result.TotalAssertions++
				result.TargetDistribution[model.Exception]++
				assertionCount++
			}

			// Per-method style classification:
			// - Has verify calls with bare call-count checks → structural
			// - Has verify calls but ALL contain content assertions (.matching/.value/withArg)
			//   → behavioral (testing output through side-effect capture, e.g., analytics events)
			// - Has output assertions only → behavioral
			// - Compose test methods with undetected assertions → behavioral
			// - Neither → unclassified
			if verifyCount > 0 && verifyCount > verifyWithContentCount {
				// At least one verify is a bare call-count check → structural
				result.StructuralMethods++
			} else if assertionCount > 0 || verifyWithContentCount > 0 || isComposeTest {
				// Has output assertions, or ALL verify calls check content → behavioral
				result.BehavioralMethods++
			} else {
				result.UnclassifiedMethods++
			}

			if assertionCount == 0 && verifyCount == 0 && !isComposeTest {
				result.ZeroAssertionMethods = append(result.ZeroAssertionMethods, model.MethodRef{
					File:   file.Path,
					Class:  cls.Name,
					Method: method.Name,
					Line:   method.LineStart,
				})
			}
		}
	}

	// Compute OutputInteractionRatio
	outputException := result.TargetDistribution[model.Output] + result.TargetDistribution[model.Exception]
	interaction := result.TargetDistribution[model.Interaction]
	denom := outputException + interaction
	if denom == 0 {
		result.OutputInteractionRatio = 1.0
	} else {
		result.OutputInteractionRatio = float64(outputException) / float64(denom)
	}

	return result
}

// classifyAssertion determines the strength and target of an assertion call.
func classifyAssertion(node model.ASTNode) (model.Strength, model.Target) {
	// Swift Testing (#expect, #require) — macro-based, not call_expression
	text := node.Text()
	if strings.Contains(text, "#expect(") || strings.Contains(text, "#require(") {
		return parse.ClassifySwiftAssertion(node)
	}

	// Get the effective function name for classification.
	// For simple calls like assertEquals(...), use the call name directly.
	// For chained calls like assertThat(result).isEqualTo(expected), use the terminal method.
	funcName := effectiveAssertionName(node)

	// Check exception first (it's a distinct target)
	if exceptionAssertions[funcName] {
		return model.Strong, model.Exception
	}

	// Determine strength
	strength := classifyStrength(funcName)

	return strength, model.Output
}

// effectiveAssertionName extracts the function name used for classification.
// For chained calls (assertThat(...).isEqualTo(...)), returns the terminal method name.
// For simple calls (assertEquals(...)), returns the call name directly.
func effectiveAssertionName(node model.ASTNode) string {
	if node.Type() != "call_expression" {
		return ""
	}

	// Try to get the terminal method name from a chained call.
	// In a chained call like assertThat(result).isEqualTo(expected),
	// the outer call_expression has a navigation_expression child that
	// contains a navigation_suffix with the terminal method name.
	for i := 0; i < int(node.Node.NamedChildCount()); i++ {
		child := node.Node.NamedChild(i)
		if child.Type() == "navigation_expression" {
			// Extract the navigation_suffix (the terminal method)
			for j := 0; j < int(child.NamedChildCount()); j++ {
				navChild := child.NamedChild(j)
				if navChild.Type() == "navigation_suffix" {
					for k := 0; k < int(navChild.NamedChildCount()); k++ {
						suffChild := navChild.NamedChild(k)
						if suffChild.Type() == "simple_identifier" {
							return string(node.Source[suffChild.StartByte():suffChild.EndByte()])
						}
					}
				}
			}
		}
	}

	// Simple call — use the direct name.
	// Also check the full text for patterns like assertThrows<...> which may
	// be parsed as a call_expression with type arguments.
	text := node.Text()
	for name := range exceptionAssertions {
		if strings.HasPrefix(text, name) {
			return name
		}
	}

	// Fall back to the deep call name
	name := deepCallNameLocal(node)
	return name
}

// deepCallNameLocal finds the root call name — duplicates parse logic to avoid circular deps.
func deepCallNameLocal(node model.ASTNode) string {
	if node.Type() != "call_expression" {
		return ""
	}
	for i := 0; i < int(node.Node.NamedChildCount()); i++ {
		child := node.Node.NamedChild(i)
		if child.Type() == "simple_identifier" {
			return string(node.Source[child.StartByte():child.EndByte()])
		}
	}
	return ""
}

// containsAssertionText checks if a statement's text contains known assertion
// patterns at any nesting depth. Used as a fallback when AST-level classification
// fails (e.g., assertions inside Turbine .test { } or Compose lambdas).
func containsAssertionText(text string) bool {
	return countAssertionMatches(text) > 0
}

// strengthPattern pairs a text pattern with its assertion strength.
type strengthPattern struct {
	pattern  string
	strength model.Strength
}

// assertionPatterns lists all text patterns that indicate an assertion,
// with their strength classification for accurate fallback scoring.
var assertionPatterns = []strengthPattern{
	// Kotest infix matchers — Strong (equality/containment)
	{" shouldBe ", model.Strong},
	{".shouldBe(", model.Strong},
	{".shouldNotBe(", model.Strong},
	{".shouldContain(", model.Strong},
	{".shouldHaveSize(", model.Strong},
	{".shouldContainExactly(", model.Strong},
	{".shouldContainInOrder(", model.Strong},
	{".shouldHaveCount(", model.Strong},
	{".shouldBeGreaterThan(", model.Strong},
	// Kotest Arrow matchers — Strong
	{".shouldBeRight(", model.Strong},
	{".shouldBeLeft(", model.Strong},
	{" shouldBeRight ", model.Strong},
	{" shouldBeLeft ", model.Strong},
	// Kotest boolean/null — Medium/Weak
	{" shouldNot ", model.Medium},
	{".shouldBeTrue()", model.Medium},
	{".shouldBeFalse()", model.Medium},
	{".shouldBeInstanceOf", model.Medium},
	{".shouldBeNull()", model.Weak},
	{".shouldNotBeNull()", model.Weak},
	{".shouldBeEmpty()", model.Weak},
	// Paparazzi screenshot testing — Strong
	{"paparazzi.snapshot", model.Strong},
	{".snapshot(", model.Strong},
	// Standard assertions nested in lambdas
	{"assertEquals(", model.Strong},
	{"assertThat(", model.Strong},     // chained: assertThat(...).isEqualTo(...) — at least Strong
	{"assertTrue(", model.Medium},
	{"assertFalse(", model.Medium},
	{"assertNotNull(", model.Weak},
	{"assertNull(", model.Weak},
	{"assertThrows", model.Strong},
	// Kotlin stdlib assert()
	{"assert(", model.Medium},
	// Compose assertions — Strong (equality/displayed) or Medium (exists/boolean)
	{".assertIsDisplayed()", model.Medium},
	{".assertExists()", model.Medium},
	{".assertDoesNotExist()", model.Medium},
	{".assertTextEquals(", model.Strong},
	{".assertTextContains(", model.Strong},
	{".assertHasClickAction()", model.Medium},
	{".assertHasNoClickAction()", model.Medium},
	{".assertIsNotDisplayed()", model.Medium},
	{".assertCountEquals(", model.Strong},
	{".assertIsEnabled()", model.Medium},
	{".assertIsNotEnabled()", model.Medium},
	{".assertIsSelected()", model.Medium},
	{".assertIsNotSelected()", model.Medium},
	{".assertIsOn()", model.Medium},
	{".assertIsOff()", model.Medium},
	{".assertIsFocused()", model.Medium},
	{".assertContentDescriptionEquals(", model.Strong},
	{".assertContentDescriptionContains(", model.Strong},
	{".assertIsToggleable()", model.Medium},
	{".assertHeightIsEqualTo(", model.Strong},
	{".assertWidthIsEqualTo(", model.Strong},
	// Roborazzi screenshot assertions — Strong
	{".captureRoboImage(", model.Strong},
	{".captureMultiTheme(", model.Strong},
	{".captureMultiDevice(", model.Strong},
	{".captureForDevice(", model.Strong},
	{"captureScreenRoboImage(", model.Strong},
	// XCTest assertions
	{"XCTAssertEqual(", model.Strong},
	{"XCTAssertTrue(", model.Medium},
	{"XCTAssertFalse(", model.Medium},
	{"XCTAssertNil(", model.Weak},
	{"XCTAssertNotNil(", model.Weak},
	{"XCTAssertThrowsError(", model.Strong},
	{"XCTAssertNoThrow(", model.Strong},
	{"XCTFail(", model.Medium},
	// Custom assertion helpers and Swift-specific patterns
	{"XCTAssertThrows(", model.Strong},
	{"XCTExpectFailure(", model.Medium},
	{"store.send(", model.Strong},
	{"store.receive(", model.Strong},
	{"waitForExpectations(", model.Medium},
	{"fulfill()", model.Medium},
	{".fulfill()", model.Medium},
	// Swift Testing framework
	{"#expect(", model.Strong},
	{"#require(", model.Strong},
	{"__expect(", model.Strong},
	{"__require(", model.Strong},
	// Swift Testing confirmation (async synchronization with implicit assertion)
	{"confirmation(", model.Medium},
	// Nimble matchers (Quick/Nimble BDD framework)
	// "expect(" matches both Nimble and inside "#expect(" — the deduplication
	// in countAssertionMatchesWithStrength subtracts #expect counts to avoid double-counting.
	{"expect(", model.Strong},
}

// customAssertionPrefixes lists prefixes for custom assertion helper method calls.
// A standalone call like assertColorSchemesEqual(...) or verifySnapshot(...) is counted
// as 1 assertion if its name starts with one of these prefixes.
var customAssertionPrefixes = []string{
	"assert",
	"verify",
	"check",
	"validate",
	"ensure",
	"perform",
}

// assertionMatchResult holds the total count and per-strength breakdown
// from the text-based assertion fallback.
type assertionMatchResult struct {
	total    int
	byStrength map[model.Strength]int
}

// countAssertionMatches counts individual assertion occurrences in statement text.
// Unlike containsAssertionText (which returns bool), this counts each match so that
// blocks like Turbine .test { } containing multiple assertions are not undercounted.
func countAssertionMatches(text string) int {
	return countAssertionMatchesWithStrength(text).total
}

// countAssertionMatchesWithStrength counts assertion occurrences and classifies by strength.
func countAssertionMatchesWithStrength(text string) assertionMatchResult {
	result := assertionMatchResult{byStrength: make(map[model.Strength]int)}
	for _, sp := range assertionPatterns {
		n := strings.Count(text, sp.pattern)
		if n > 0 {
			// Deduplicate: "expect(" matches inside "#expect(" — subtract #expect count
			// so Swift Testing macros aren't double-counted with Nimble.
			if sp.pattern == "expect(" {
				hashExpect := strings.Count(text, "#expect(")
				n -= hashExpect
				if n <= 0 {
					continue
				}
			}
			result.total += n
			result.byStrength[sp.strength] += n
		}
	}
	// Custom assertion helpers: count standalone calls like assertColorSchemesEqual(...)
	// These are matched by scanning for prefix followed by an uppercase letter (camelCase).
	// Only count if not already matched by a known pattern above.
	if result.total == 0 {
		for _, prefix := range customAssertionPrefixes {
			idx := 0
			for {
				pos := strings.Index(text[idx:], prefix)
				if pos < 0 {
					break
				}
				pos += idx
				afterPrefix := pos + len(prefix)
				if afterPrefix < len(text) {
					ch := text[afterPrefix]
					if ch >= 'A' && ch <= 'Z' {
						result.total++
						result.byStrength[model.Medium]++
					}
				}
				idx = pos + 1
			}
		}
	}
	return result
}

// verifyPatterns are text patterns indicating verify calls nested inside lambdas.
var verifyPatterns = []string{
	"verify {",
	"verify(",
	"coVerify {",
	"coVerify(",
	"Mockito.verify(",
}

// countVerifyMatches counts verify call occurrences in statement text.
// Used as a text-based fallback when verify calls are nested inside lambdas
// (e.g., composeTestRule.runOnIdle { verify { ... } }).
func countVerifyMatches(text string) int {
	count := 0
	for _, pattern := range verifyPatterns {
		count += strings.Count(text, pattern)
	}
	return count
}

// extractMethodBodyText extracts the raw source text between a method's start and end lines.
func extractMethodBodyText(src []byte, lineStart, lineEnd int) string {
	// Convert line numbers to byte offsets
	line := 1
	start := 0
	end := len(src)
	for i, b := range src {
		if line == lineStart && start == 0 {
			start = i
		}
		if line > lineEnd {
			end = i
			break
		}
		if b == '\n' {
			line++
		}
	}
	if start >= end || start >= len(src) {
		return ""
	}
	return string(src[start:end])
}

// classifyStrength determines the strength level of an assertion by function name.
func classifyStrength(name string) model.Strength {
	if strongAssertions[name] {
		return model.Strong
	}
	if mediumAssertions[name] {
		return model.Medium
	}
	if weakAssertions[name] {
		return model.Weak
	}
	// Default to Medium for unknown assertion functions
	return model.Medium
}

// hasExpectedAnnotation checks if a test method has @Test(expected = SomeException::class),
// which is JUnit 4's way of declaring an implicit exception assertion.
func hasExpectedAnnotation(method model.TestMethod) bool {
	for _, ann := range method.Annotations {
		if ann.Name == "Test" && strings.Contains(ann.Args, "expected") {
			return true
		}
	}
	return false
}
