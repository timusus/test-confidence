package parse

import (
	"os"
	"testing"

	"github.com/timusus/test-confidence/internal/model"
)

func mustParseFixture(t *testing.T, filename string) *model.ParsedTestFile {
	t.Helper()
	src, err := os.ReadFile("../../testdata/fixtures/" + filename)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseTestFile("../../testdata/fixtures/"+filename, src, model.Kotlin)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { parsed.Close() })
	return parsed
}

func findMethod(t *testing.T, parsed *model.ParsedTestFile, name string) model.TestMethod {
	t.Helper()
	for _, cls := range parsed.Classes {
		for _, m := range cls.Methods {
			if m.Name == name {
				return m
			}
		}
	}
	t.Fatalf("method %q not found", name)
	return model.TestMethod{}
}

func TestIsVerifyCall(t *testing.T) {
	parsed := mustParseFixture(t, "kotlin_helpers_test.kt")
	method := findMethod(t, parsed, "verify calls")

	verifyCount := 0
	for _, stmt := range method.Statements {
		if IsVerifyCall(stmt) {
			verifyCount++
		}
	}
	if verifyCount != 2 {
		t.Errorf("expected 2 verify calls, got %d", verifyCount)
	}
}

func TestIsStubCall(t *testing.T) {
	parsed := mustParseFixture(t, "kotlin_helpers_test.kt")
	method := findMethod(t, parsed, "stub calls")

	stubCount := 0
	for _, stmt := range method.Statements {
		if IsStubCall(stmt) {
			stubCount++
		}
	}
	if stubCount != 2 {
		t.Errorf("expected 2 stub calls, got %d", stubCount)
	}
}

func TestIsAssertionCall(t *testing.T) {
	parsed := mustParseFixture(t, "kotlin_helpers_test.kt")
	method := findMethod(t, parsed, "assertion calls")

	assertionCount := 0
	for _, stmt := range method.Statements {
		if IsAssertionCall(stmt) {
			assertionCount++
		}
	}
	// assertThat, assertEquals, assertTrue, assertNotNull, assertThrows = 5
	if assertionCount != 5 {
		t.Errorf("expected 5 assertion calls, got %d", assertionCount)
	}
}

func TestIsNotCrossClassified(t *testing.T) {
	parsed := mustParseFixture(t, "kotlin_helpers_test.kt")
	method := findMethod(t, parsed, "mixed calls")

	for _, stmt := range method.Statements {
		isVerify := IsVerifyCall(stmt)
		isStub := IsStubCall(stmt)
		isAssert := IsAssertionCall(stmt)
		trueCount := 0
		if isVerify {
			trueCount++
		}
		if isStub {
			trueCount++
		}
		if isAssert {
			trueCount++
		}
		if trueCount > 1 {
			t.Errorf("statement classified as multiple types: verify=%v stub=%v assert=%v text=%q",
				isVerify, isStub, isAssert, stmt.Text())
		}
	}
}

func TestExtractCallTarget(t *testing.T) {
	parsed := mustParseFixture(t, "kotlin_helpers_test.kt")
	method := findMethod(t, parsed, "verify calls")

	for _, stmt := range method.Statements {
		if IsVerifyCall(stmt) {
			receiver, methodName := ExtractCallTarget(stmt)
			if receiver != "repo" {
				t.Errorf("expected receiver 'repo', got %q", receiver)
			}
			if methodName != "save" && methodName != "saveAsync" {
				t.Errorf("expected method 'save' or 'saveAsync', got %q", methodName)
			}
			break
		}
	}
}

func TestExtractCallTargetFromStub(t *testing.T) {
	parsed := mustParseFixture(t, "kotlin_helpers_test.kt")
	method := findMethod(t, parsed, "stub calls")

	for _, stmt := range method.Statements {
		if IsStubCall(stmt) {
			receiver, methodName := ExtractCallTarget(stmt)
			if receiver != "repo" {
				t.Errorf("expected receiver 'repo', got %q", receiver)
			}
			if methodName != "getUser" && methodName != "getUserAsync" {
				t.Errorf("expected method 'getUser' or 'getUserAsync', got %q", methodName)
			}
			break
		}
	}
}

func TestExtractReturnValue(t *testing.T) {
	parsed := mustParseFixture(t, "kotlin_helpers_test.kt")
	method := findMethod(t, parsed, "stub calls")

	for _, stmt := range method.Statements {
		if IsStubCall(stmt) {
			rv := ExtractReturnValue(stmt)
			if rv != "user" {
				t.Errorf("expected return value 'user', got %q", rv)
			}
			break
		}
	}
}

func TestExtractAssertedValues(t *testing.T) {
	parsed := mustParseFixture(t, "kotlin_helpers_test.kt")
	method := findMethod(t, parsed, "assertion calls")

	for _, stmt := range method.Statements {
		text := stmt.Text()
		if text == "assertThat(result).isEqualTo(expected)" {
			actual, expected := ExtractAssertedValues(stmt)
			if actual != "result" {
				t.Errorf("assertThat: expected actual 'result', got %q", actual)
			}
			if expected != "expected" {
				t.Errorf("assertThat: expected expected 'expected', got %q", expected)
			}
		}
		if text == "assertEquals(expected, actual)" {
			actual, expected := ExtractAssertedValues(stmt)
			if actual != "actual" {
				t.Errorf("assertEquals: expected actual 'actual', got %q", actual)
			}
			if expected != "expected" {
				t.Errorf("assertEquals: expected expected 'expected', got %q", expected)
			}
		}
	}
}

func TestHasParentOfType(t *testing.T) {
	parsed := mustParseFixture(t, "kotlin_helpers_test.kt")
	method := findMethod(t, parsed, "delay in runTest")

	// The top-level statement is `runTest { ... }` — it doesn't have a parent of type runTest.
	// We need to look inside the runTest block for `delay(100)`.
	// But our statements are top-level only. HasParentOfType is designed for
	// nodes that are already inside a block — we need to walk into the lambda.

	// The top-level statement is the runTest call_expression itself.
	// Let's verify that the delay call inside it has runTest as parent.
	if len(method.Statements) != 1 {
		t.Fatalf("expected 1 top-level statement, got %d", len(method.Statements))
	}

	runTestStmt := method.Statements[0]
	// Walk into runTest's lambda to find delay
	found := false
	walkChildren(runTestStmt, func(child model.ASTNode) bool {
		if child.Type() == "call_expression" {
			for i := 0; i < int(child.Node.NamedChildCount()); i++ {
				c := child.Node.NamedChild(i)
				if c.Type() == "simple_identifier" && nodeText(c, child.Source) == "delay" {
					if HasParentOfType(child, "runTest") {
						found = true
						return false
					}
				}
			}
		}
		return true
	})
	if !found {
		t.Error("expected delay() to have parent runTest")
	}
}

// walkChildren recursively walks all named children of an ASTNode.
// The callback returns false to stop walking.
func walkChildren(node model.ASTNode, fn func(model.ASTNode) bool) bool {
	for _, child := range node.Children() {
		if !fn(child) {
			return false
		}
		if !walkChildren(child, fn) {
			return false
		}
	}
	return true
}
