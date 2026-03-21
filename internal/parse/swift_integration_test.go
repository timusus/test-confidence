package parse

import (
	"testing"

	"github.com/timusus/test-confidence/internal/model"
)

// TestSwiftFullPipelineBasic verifies that a Swift test file parsed through
// ParseTestFile produces data compatible with the analyzer pipeline.
// This is an integration-level test in the parse package.
func TestSwiftFullPipelineBasic(t *testing.T) {
	parsed := mustParseSwiftFixture(t, "swift_basic_test.swift")

	// Verify the data shape matches what analyzers expect
	if parsed.Language != model.Swift {
		t.Fatalf("wrong language: %v", parsed.Language)
	}

	cls := parsed.Classes[0]

	// Methods should have line numbers
	for _, m := range cls.Methods {
		if m.LineStart == 0 || m.LineEnd == 0 {
			t.Errorf("method %q has zero line numbers", m.Name)
		}
		if m.LineEnd < m.LineStart {
			t.Errorf("method %q has end before start: %d < %d", m.Name, m.LineEnd, m.LineStart)
		}
	}

	// Properties should have line numbers
	for _, p := range cls.Properties {
		if p.Line == 0 {
			t.Errorf("property %q has zero line number", p.Name)
		}
	}

	// Imports should have line numbers
	for _, imp := range parsed.Imports {
		if imp.Line == 0 {
			t.Errorf("import %q has zero line number", imp.Path)
		}
	}
}

// TestSwiftExceptionAssertions verifies XCTAssertThrowsError and XCTAssertNoThrow
// are detected as assertions.
func TestSwiftExceptionAssertions(t *testing.T) {
	parsed := mustParseSwiftFixture(t, "swift_antipatterns_test.swift")

	cls := parsed.Classes[0]

	for _, m := range cls.Methods {
		if m.Name == "testWithThrows" {
			assertionCount := 0
			for _, stmt := range m.Statements {
				if IsAssertionCall(stmt) {
					assertionCount++
				}
			}
			if assertionCount == 0 {
				t.Error("testWithThrows should have assertion calls (XCTAssertThrowsError)")
			}
		}
		if m.Name == "testWithNoThrow" {
			assertionCount := 0
			for _, stmt := range m.Statements {
				if IsAssertionCall(stmt) {
					assertionCount++
				}
			}
			if assertionCount != 1 {
				t.Errorf("testWithNoThrow should have 1 assertion call, got %d", assertionCount)
			}
		}
	}
}

// TestSwiftStatementsAreASTNodes verifies that statement ASTNodes have valid
// tree-sitter node references (required for analyzers that walk the AST).
func TestSwiftStatementsAreASTNodes(t *testing.T) {
	parsed := mustParseSwiftFixture(t, "swift_basic_test.swift")

	for _, cls := range parsed.Classes {
		for _, m := range cls.Methods {
			for _, stmt := range m.Statements {
				if stmt.Node == nil {
					t.Errorf("method %q has statement with nil Node", m.Name)
				}
				if stmt.Source == nil {
					t.Errorf("method %q has statement with nil Source", m.Name)
				}
				if stmt.Text() == "" {
					t.Errorf("method %q has statement with empty text", m.Name)
				}
			}
		}
	}
}
