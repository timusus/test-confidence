package analyze

import (
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/timusus/test-confidence/internal/config"
	"github.com/timusus/test-confidence/internal/model"
	"github.com/timusus/test-confidence/internal/parse"
)

// AnalyzeAntiPatterns detects common test anti-patterns in a parsed test file.
func AnalyzeAntiPatterns(file *model.ParsedTestFile, cfg config.Config) []model.AntiPattern {
	var results []model.AntiPattern

	// Import-level: reflection usage
	for _, imp := range file.Imports {
		if strings.HasPrefix(imp.Path, "java.lang.reflect") {
			results = append(results, model.AntiPattern{
				Type:     model.ReflectionUsage,
				File:     file.Path,
				Line:     imp.Line,
				Severity: model.Tier1,
			})
		}
	}

	// File-level: relaxed mock policy — global or per-mock relaxed configuration
	// silently returns defaults for unstubbed calls, masking missing interactions.
	// Tier 2: useful signal but common in legacy codebases transitioning to stricter mocking.
	if file.Source != nil {
		src := string(file.Source)
		// Swift: Mockable framework global relaxed policy
		if strings.Contains(src, "MockerPolicy.default") && strings.Contains(src, "relaxed") {
			results = append(results, model.AntiPattern{
				Type:     model.RelaxedMockPolicy,
				File:     file.Path,
				Severity: model.Tier2,
			})
		}
		// Kotlin: MockK relaxed mocks — mockk(relaxed = true) or @MockK(relaxed = true)
		if strings.Contains(src, "relaxed = true") || strings.Contains(src, "relaxed=true") {
			results = append(results, model.AntiPattern{
				Type:     model.RelaxedMockPolicy,
				File:     file.Path,
				Severity: model.Tier2,
			})
		}
		// Mockito: lenient strictness
		if strings.Contains(src, "strictness") && strings.Contains(src, "LENIENT") {
			results = append(results, model.AntiPattern{
				Type:     model.RelaxedMockPolicy,
				File:     file.Path,
				Severity: model.Tier2,
			})
		}
	}

	for _, cls := range file.Classes {
		// God test class: too many test methods in a single class
		if len(cls.Methods) > cfg.Thresholds.GodTestClass {
			results = append(results, model.AntiPattern{
				Type:     model.GodTestClass,
				File:     file.Path,
				Class:    cls.Name,
				Severity: model.Tier1,
			})
		}

		for _, method := range cls.Methods {
			// Check if method is ignored
			isIgnored := false
			for _, ann := range method.Annotations {
				if ann.Name == "Ignore" || ann.Name == "Disabled" {
					isIgnored = true
					results = append(results, model.AntiPattern{
						Type:     model.IgnoredTest,
						File:     file.Path,
						Line:     ann.Line,
						Method:   method.Name,
						Severity: model.Tier1,
					})
				}
			}

			// Empty test: has @Test but zero statements (and not already flagged as ignored)
			if len(method.Statements) == 0 && !isIgnored {
				results = append(results, model.AntiPattern{
					Type:     model.EmptyTest,
					File:     file.Path,
					Line:     method.LineStart,
					Method:   method.Name,
					Severity: model.Tier1,
				})
			}

			// Walk statements for Thread.sleep, conditional logic
			for _, stmt := range method.Statements {
				walkForAntiPatterns(stmt, file.Path, method.Name, &results)
			}

			// Assertion roulette: too many assertions in one method
			assertionCount := 0
			for _, stmt := range method.Statements {
				if parse.IsAssertionCall(stmt) {
					assertionCount++
				}
			}
			if assertionCount > cfg.Thresholds.AssertionRoulette {
				results = append(results, model.AntiPattern{
					Type:     model.AssertionRoulette,
					File:     file.Path,
					Line:     method.LineStart,
					Method:   method.Name,
					Severity: model.Tier1,
				})
			}
		}
	}

	return results
}

// walkForAntiPatterns recursively walks AST nodes looking for anti-patterns.
func walkForAntiPatterns(node model.ASTNode, file, method string, results *[]model.AntiPattern) {
	// Check for Thread.sleep() / SystemClock.sleep()
	if node.Type() == "call_expression" {
		text := node.Text()
		if isSleepCall(text) {
			line := int(node.Node.StartPoint().Row) + 1
			*results = append(*results, model.AntiPattern{
				Type:     model.ThreadSleep,
				File:     file,
				Line:     line,
				Method:   method,
				Severity: model.Tier1,
			})
			return // no need to recurse into this node
		}
	}

	// Check for conditional logic: if_expression/when_expression (Kotlin) or if_statement/switch_statement (Swift)
	// Only flag if not inside a lambda/closure (which is likely test setup / fake behavior, not conditional assertions)
	if node.Type() == "if_expression" || node.Type() == "when_expression" ||
		node.Type() == "if_statement" || node.Type() == "switch_statement" {
		if !hasLambdaAncestor(node.Node) {
			// Suppress for switch/when statements that contain assertions in their branches —
			// this is exhaustive enum pattern matching used as an assertion pattern, not conditional logic.
			// e.g., switch result { case .success: #expect(...); case .failure: Issue.record(...) }
			if (node.Type() == "switch_statement" || node.Type() == "when_expression") &&
				switchContainsAssertions(node) {
				// Recurse into children but don't flag the switch itself
			} else {
				line := int(node.Node.StartPoint().Row) + 1
				*results = append(*results, model.AntiPattern{
					Type:     model.ConditionalLogic,
					File:     file,
					Line:     line,
					Method:   method,
					Severity: model.Tier1,
				})
				return // don't recurse to avoid double-counting nested conditionals
			}
		}
	}

	// Recurse into children
	for _, child := range node.Children() {
		walkForAntiPatterns(child, file, method, results)
	}
}

// switchContainsAssertions checks if a switch/when statement's body contains assertion calls.
// Used to distinguish exhaustive enum pattern-matching assertions from conditional test logic.
func switchContainsAssertions(node model.ASTNode) bool {
	text := node.Text()
	return containsAssertionText(text)
}

// hasLambdaAncestor checks if a node is nested inside a lambda_literal, annotated_lambda,
// closure_expression (Swift), or object_literal (Kotlin anonymous class).
// Conditional logic inside these contexts is typically fake/stub behavior, not test logic.
func hasLambdaAncestor(node *sitter.Node) bool {
	current := node.Parent()
	for current != nil {
		switch current.Type() {
		case "lambda_literal", "annotated_lambda", "closure_expression",
			"object_literal", "anonymous_function":
			return true
		}
		current = current.Parent()
	}
	return false
}

// isSleepCall checks if a call_expression text represents a sleep/delay call.
// Detects Thread.sleep, SystemClock.sleep, Task.sleep, usleep, sleep (standalone),
// and DispatchQueue delayed dispatch.
func isSleepCall(text string) bool {
	// Kotlin/Java sleep
	if strings.Contains(text, "Thread.sleep") || strings.Contains(text, "SystemClock.sleep") {
		return true
	}
	// Swift concurrency sleep
	if strings.Contains(text, "Task.sleep") {
		return true
	}
	// POSIX sleep
	if strings.HasPrefix(text, "usleep(") {
		return true
	}
	// Standalone sleep() — but not Task.sleep() or Thread.sleep() which are already caught above
	if strings.HasPrefix(text, "sleep(") {
		return true
	}
	// Delayed dispatch
	if strings.Contains(text, "DispatchQueue") && strings.Contains(text, "asyncAfter") {
		return true
	}
	return false
}
