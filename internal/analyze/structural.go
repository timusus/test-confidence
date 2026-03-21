package analyze

import (
	"strings"

	"github.com/timusus/test-confidence/internal/model"
	"github.com/timusus/test-confidence/internal/parse"
)

// AnalyzeStructural detects definitive structural coupling patterns in a parsed test file.
// All indicators are Tier 1 (>90% accuracy).
func AnalyzeStructural(file *model.ParsedTestFile) []model.StructuralIndicator {
	var results []model.StructuralIndicator

	for _, cls := range file.Classes {
		for _, method := range cls.Methods {
			detectVerifyOrdering(file, method, &results)
			detectArgumentCaptor(file, method, &results)
			detectStubAndVerify(file, method, &results)
			detectVerifyOnlyTest(file, method, &results)
			detectPropertyAccessVerify(file, method, &results)
		}
	}

	return results
}

// detectVerifyOrdering flags tests that use verifyOrder, verifySequence, or Mockito InOrder.
func detectVerifyOrdering(file *model.ParsedTestFile, method model.TestMethod, results *[]model.StructuralIndicator) {
	for _, stmt := range method.Statements {
		text := stmt.Text()
		if containsVerifyOrdering(text) {
			line := int(stmt.Node.StartPoint().Row) + 1
			*results = append(*results, model.StructuralIndicator{
				Type:   model.VerifyOrdering,
				File:   file.Path,
				Line:   line,
				Method: method.Name,
				Detail: "uses ordering verification (verifyOrder/verifySequence/InOrder)",
			})
			return // one per method
		}
	}
}

// containsVerifyOrdering checks if text contains ordering verification patterns.
func containsVerifyOrdering(text string) bool {
	if strings.Contains(text, "verifyOrder") || strings.Contains(text, "verifySequence") {
		return true
	}
	// Mockito InOrder — but not Kotest shouldContainInOrder
	if strings.Contains(text, "InOrder(") && !strings.Contains(text, "shouldContainInOrder") &&
		!strings.Contains(text, "ContainInOrder") {
		return true
	}
	return false
}

// detectArgumentCaptor flags tests that use slot/capture (MockK) or ArgumentCaptor (Mockito).
func detectArgumentCaptor(file *model.ParsedTestFile, method model.TestMethod, results *[]model.StructuralIndicator) {
	for _, stmt := range method.Statements {
		text := stmt.Text()
		if containsArgumentCaptor(text) {
			line := int(stmt.Node.StartPoint().Row) + 1
			*results = append(*results, model.StructuralIndicator{
				Type:   model.ArgumentCaptor,
				File:   file.Path,
				Line:   line,
				Method: method.Name,
				Detail: "uses argument capture (slot/capture/ArgumentCaptor)",
			})
			return // one per method
		}
	}
}

// containsArgumentCaptor checks if text contains argument capture patterns.
func containsArgumentCaptor(text string) bool {
	return strings.Contains(text, "slot<") ||
		strings.Contains(text, "capture(") ||
		strings.Contains(text, "ArgumentCaptor")
}

// detectStubAndVerify flags tests where the same (receiver, method) pair is both stubbed and verified.
func detectStubAndVerify(file *model.ParsedTestFile, method model.TestMethod, results *[]model.StructuralIndicator) {
	type callTarget struct {
		receiver string
		method   string
	}

	stubTargets := map[callTarget]bool{}
	verifyTargets := map[callTarget]bool{}

	for _, stmt := range method.Statements {
		if parse.IsStubCall(stmt) {
			recv, meth := parse.ExtractCallTarget(stmt)
			if recv != "" && meth != "" {
				stubTargets[callTarget{recv, meth}] = true
			}
		}
		if parse.IsVerifyCall(stmt) {
			recv, meth := parse.ExtractCallTarget(stmt)
			if recv != "" && meth != "" {
				verifyTargets[callTarget{recv, meth}] = true
			}
		}
	}

	// Find overlap — but skip if the verify call contains withArg assertions
	// (it's testing content, not just that the method was called)
	for target := range stubTargets {
		if verifyTargets[target] {
			// Check if the verify for this target has content assertions
			hasContentAssertion := false
			for _, stmt := range method.Statements {
				if parse.IsVerifyCall(stmt) {
					recv, meth := parse.ExtractCallTarget(stmt)
					if recv == target.receiver && meth == target.method {
						if parse.VerifyHasContentAssertion(stmt) {
							hasContentAssertion = true
							break
						}
					}
				}
			}
			if !hasContentAssertion {
				*results = append(*results, model.StructuralIndicator{
					Type:   model.StubAndVerify,
					File:   file.Path,
					Line:   method.LineStart,
					Method: method.Name,
					Detail: "stubs and verifies same target: " + target.receiver + "." + target.method,
				})
				return // one per method
			}
		}
	}
}

// detectPropertyAccessVerify flags verify calls that count property getter accesses.
// Pattern: verify(mock).someProperty.called(N) — no () after the property name.
// This is extremely fragile: any refactoring that reads the property a different
// number of times breaks the test, even if the behavior is unchanged.
// Detected via text: "verify(" + ".called(" but with a ".propertyName.called(" pattern
// where propertyName has no trailing "()" (indicating it's a getter, not a method call).
func detectPropertyAccessVerify(file *model.ParsedTestFile, method model.TestMethod, results *[]model.StructuralIndicator) {
	for _, stmt := range method.Statements {
		if !parse.IsVerifyCall(stmt) {
			continue
		}
		text := stmt.Text()
		if isPropertyAccessVerification(text) {
			line := int(stmt.Node.StartPoint().Row) + 1
			*results = append(*results, model.StructuralIndicator{
				Type:   model.PropertyAccessVerify,
				File:   file.Path,
				Line:   line,
				Method: method.Name,
				Detail: "verifies property getter access count — extremely fragile to refactoring",
			})
			return // one per method
		}
	}
}

// isPropertyAccessVerification detects verify calls that count property getter accesses.
// In Mockable: verify(mock).someProperty.called(.once)
// In MockK: verify { mock.someProperty }
// The key distinction: a property access has .name.called( without () before .called.
// A method call has .name().called( or .name(args).called(.
func isPropertyAccessVerification(text string) bool {
	// Look for the Mockable pattern: verify(...).property.called(
	// where "property" is followed by .called without ()
	calledIdx := strings.Index(text, ".called(")
	if calledIdx < 0 {
		return false
	}
	// Walk backwards from .called( to find the preceding segment
	prefix := text[:calledIdx]
	// If the character before .called is not ), it's a property access
	trimmed := strings.TrimRight(prefix, " \t\n")
	if len(trimmed) == 0 {
		return false
	}
	lastChar := trimmed[len(trimmed)-1]
	// Property access: ...propertyName.called(  (last char is a letter/digit)
	// Method call: ...methodName().called(  (last char is ')')
	// Method call: ...methodName(arg).called(  (last char is ')')
	if lastChar == ')' {
		return false // method call, not property access
	}
	// Verify it's actually a verify call (not some other .called usage)
	return strings.Contains(text, "verify(") || strings.Contains(text, "verify {") ||
		strings.Contains(text, "coVerify {") || strings.Contains(text, "coVerify(")
}

// detectVerifyOnlyTest flags tests that have verify calls but no output/exception assertions.
func detectVerifyOnlyTest(file *model.ParsedTestFile, method model.TestMethod, results *[]model.StructuralIndicator) {
	hasVerify := false
	hasAssertion := false
	hasStub := false

	for _, stmt := range method.Statements {
		if parse.IsVerifyCall(stmt) {
			hasVerify = true
		}
		if parse.IsAssertionCall(stmt) {
			hasAssertion = true
		}
		if parse.IsStubCall(stmt) {
			hasStub = true
		}
	}

	// A verify-only test has verify calls but no output assertions and no stubs.
	// Tests with stubs + verify are caught by StubAndVerify instead.
	// Exception: if ALL verify calls contain withArg assertions, the test is actually
	// checking content (e.g., analytics event fields) — don't flag it.
	if hasVerify && !hasAssertion && !hasStub {
		allVerifiesHaveContent := true
		for _, stmt := range method.Statements {
			if parse.IsVerifyCall(stmt) && !parse.VerifyHasContentAssertion(stmt) {
				allVerifiesHaveContent = false
				break
			}
		}
		if !allVerifiesHaveContent {
			*results = append(*results, model.StructuralIndicator{
				Type:   model.VerifyOnlyTest,
				File:   file.Path,
				Line:   method.LineStart,
				Method: method.Name,
				Detail: "test only uses verify calls with no output assertions",
			})
		}
	}
}
