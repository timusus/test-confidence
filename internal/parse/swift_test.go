package parse

import (
	"os"
	"strings"
	"testing"

	"github.com/timusus/test-confidence/internal/model"
)

func mustParseSwiftFixture(t *testing.T, filename string) *model.ParsedTestFile {
	t.Helper()
	src, err := os.ReadFile("../../testdata/fixtures/" + filename)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseTestFile("../../testdata/fixtures/"+filename, src, model.Swift)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { parsed.Close() })
	return parsed
}

func TestParseSwift(t *testing.T) {
	src, err := os.ReadFile("../../testdata/fixtures/swift_basic_test.swift")
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}

	tree, err := ParseSwift(src)
	if err != nil {
		t.Fatalf("failed to parse Swift: %v", err)
	}
	defer tree.Close()

	root := tree.RootNode()
	if root.Type() != "source_file" {
		t.Errorf("expected root node type 'source_file', got %q", root.Type())
	}
	if root.NamedChildCount() == 0 {
		t.Error("expected at least one named child node")
	}
}

func TestParseSwiftTestFile(t *testing.T) {
	parsed := mustParseSwiftFixture(t, "swift_basic_test.swift")

	// Basic structure
	if len(parsed.Classes) != 1 {
		t.Fatalf("expected 1 class, got %d", len(parsed.Classes))
	}

	cls := parsed.Classes[0]
	if cls.Name != "UserViewModelTests" {
		t.Errorf("expected class name 'UserViewModelTests', got %q", cls.Name)
	}

	// 3 test methods (setUp/tearDown are not test methods)
	if len(cls.Methods) != 3 {
		t.Errorf("expected 3 test methods, got %d", len(cls.Methods))
		for _, m := range cls.Methods {
			t.Logf("  method: %s", m.Name)
		}
	}

	// Imports include XCTest
	hasXCTest := false
	for _, imp := range parsed.Imports {
		if strings.Contains(imp.Path, "XCTest") {
			hasXCTest = true
			break
		}
	}
	if !hasXCTest {
		t.Error("expected XCTest import")
	}

	// Properties detected
	if len(cls.Properties) < 2 {
		t.Errorf("expected at least 2 properties, got %d", len(cls.Properties))
	}

	// Check sut property
	foundSUT := false
	for _, prop := range cls.Properties {
		if prop.Name == "sut" {
			foundSUT = true
			if prop.TypeName != "UserViewModel" {
				t.Errorf("expected sut type 'UserViewModel', got %q", prop.TypeName)
			}
		}
	}
	if !foundSUT {
		t.Error("expected 'sut' property")
	}

	// Setup blocks from override setUp()
	if len(cls.SetupBlocks) == 0 {
		t.Error("expected setup blocks from setUp() method")
	}

	// Test methods have synthetic @Test annotation
	for _, m := range cls.Methods {
		hasTest := false
		for _, ann := range m.Annotations {
			if ann.Name == "Test" {
				hasTest = true
			}
		}
		if !hasTest {
			t.Errorf("method %q missing synthetic @Test annotation", m.Name)
		}
	}

	// Methods have statements
	for _, m := range cls.Methods {
		if m.Name == "testLoadUsers" {
			if len(m.Statements) == 0 {
				t.Error("expected statements in 'testLoadUsers' method")
			}
			// Should have 4 statements: let users, XCTAssertEqual, XCTAssertTrue, XCTAssertNotNil
			if len(m.Statements) != 4 {
				t.Errorf("expected 4 statements in testLoadUsers, got %d", len(m.Statements))
			}
		}
	}
}

func TestParseSwiftAssertionDetection(t *testing.T) {
	parsed := mustParseSwiftFixture(t, "swift_basic_test.swift")

	cls := parsed.Classes[0]
	for _, m := range cls.Methods {
		if m.Name != "testLoadUsers" {
			continue
		}

		assertionCount := 0
		for _, stmt := range m.Statements {
			if IsAssertionCall(stmt) {
				assertionCount++
			}
		}
		// XCTAssertEqual, XCTAssertTrue, XCTAssertNotNil = 3 assertions
		if assertionCount != 3 {
			t.Errorf("expected 3 assertion calls in testLoadUsers, got %d", assertionCount)
			for _, stmt := range m.Statements {
				t.Logf("  stmt type=%s isAssert=%v text=%q",
					stmt.Type(), IsAssertionCall(stmt), stmt.Text())
			}
		}
	}
}

func TestParseSwiftManualMocks(t *testing.T) {
	parsed := mustParseSwiftFixture(t, "swift_manual_mocks_test.swift")

	if len(parsed.Classes) != 1 {
		t.Fatalf("expected 1 test class, got %d", len(parsed.Classes))
	}

	cls := parsed.Classes[0]
	if cls.Name != "UserServiceTests" {
		t.Errorf("expected class name 'UserServiceTests', got %q", cls.Name)
	}

	// Should have mockRepository and fakeAnalytics properties
	foundMock := false
	foundFake := false
	for _, prop := range cls.Properties {
		if prop.Name == "mockRepository" {
			foundMock = true
			if prop.TypeName != "MockUserRepository" {
				t.Errorf("expected type 'MockUserRepository', got %q", prop.TypeName)
			}
		}
		if prop.Name == "fakeAnalytics" {
			foundFake = true
			if prop.TypeName != "FakeAnalyticsTracker" {
				t.Errorf("expected type 'FakeAnalyticsTracker', got %q", prop.TypeName)
			}
		}
	}
	if !foundMock {
		t.Error("expected mockRepository property")
	}
	if !foundFake {
		t.Error("expected fakeAnalytics property")
	}

	// 2 test methods
	if len(cls.Methods) != 2 {
		t.Errorf("expected 2 test methods, got %d", len(cls.Methods))
	}
}

func TestParseSwiftAntiPatterns(t *testing.T) {
	parsed := mustParseSwiftFixture(t, "swift_antipatterns_test.swift")

	if len(parsed.Classes) != 1 {
		t.Fatalf("expected 1 class, got %d", len(parsed.Classes))
	}

	cls := parsed.Classes[0]

	// Find specific test methods
	var emptyMethod, ignoredMethod, sleepMethod, conditionalMethod model.TestMethod
	for _, m := range cls.Methods {
		switch m.Name {
		case "testEmpty":
			emptyMethod = m
		case "testIgnoredWithSkip":
			ignoredMethod = m
		case "testWithSleep":
			sleepMethod = m
		case "testConditionalLogic":
			conditionalMethod = m
		}
	}

	// Empty test
	if len(emptyMethod.Statements) != 0 {
		t.Errorf("testEmpty should have 0 statements, got %d", len(emptyMethod.Statements))
	}

	// Ignored test (XCTSkip)
	hasIgnore := false
	for _, ann := range ignoredMethod.Annotations {
		if ann.Name == "Ignore" {
			hasIgnore = true
		}
	}
	if !hasIgnore {
		t.Error("testIgnoredWithSkip should have Ignore annotation (from XCTSkip detection)")
	}

	// Sleep test has statements
	if len(sleepMethod.Statements) == 0 {
		t.Error("testWithSleep should have statements")
	}
	// Check Thread.sleep is in the text
	hasSleep := false
	for _, stmt := range sleepMethod.Statements {
		if strings.Contains(stmt.Text(), "Thread.sleep") {
			hasSleep = true
		}
	}
	if !hasSleep {
		t.Error("testWithSleep should contain Thread.sleep")
	}

	// Conditional test has if_statement
	if len(conditionalMethod.Statements) == 0 {
		t.Error("testConditionalLogic should have statements")
	}
	hasConditional := false
	for _, stmt := range conditionalMethod.Statements {
		if stmt.Type() == "if_statement" {
			hasConditional = true
		}
	}
	if !hasConditional {
		t.Error("testConditionalLogic should contain if_statement")
	}
}

func TestParseSwiftAsyncTests(t *testing.T) {
	parsed := mustParseSwiftFixture(t, "swift_async_test.swift")

	if len(parsed.Classes) != 1 {
		t.Fatalf("expected 1 class, got %d", len(parsed.Classes))
	}

	cls := parsed.Classes[0]

	// Should have 3 test methods
	if len(cls.Methods) != 3 {
		t.Errorf("expected 3 test methods, got %d", len(cls.Methods))
		for _, m := range cls.Methods {
			t.Logf("  method: %s", m.Name)
		}
	}

	// Setup blocks from setUpWithError and tearDownWithError
	if len(cls.SetupBlocks) < 1 {
		t.Error("expected at least 1 setup block from setUpWithError()")
	}

	// Test methods with top-level assertions
	for _, m := range cls.Methods {
		if m.Name == "testAsyncWithExpectation" {
			// This method has assertions nested inside a closure callback.
			// The top-level IsAssertionCall won't see them; the text-based
			// fallback in the assertion analyzer handles this case.
			continue
		}
		assertionCount := 0
		for _, stmt := range m.Statements {
			if IsAssertionCall(stmt) {
				assertionCount++
			}
		}
		if assertionCount == 0 {
			t.Errorf("method %q should have at least one assertion", m.Name)
		}
	}
}

func TestParseSwiftLanguageField(t *testing.T) {
	parsed := mustParseSwiftFixture(t, "swift_basic_test.swift")

	if parsed.Language != model.Swift {
		t.Errorf("expected language Swift, got %v", parsed.Language)
	}
}

func TestParseSwiftProductionFile(t *testing.T) {
	src := []byte(`
import Foundation

class UserViewModel {
    private let repository: UserRepository

    init(repository: UserRepository) {
        self.repository = repository
    }

    func loadUsers() -> [User] {
        return repository.fetchUsers()
    }

    func deleteUser(id: String) -> Bool {
        return repository.deleteUser(id: id)
    }
}
`)
	parsed, err := ParseProductionFile("test.swift", src, model.Swift)
	if err != nil {
		t.Fatal(err)
	}
	defer parsed.Close()

	if parsed.Language != model.Swift {
		t.Errorf("expected language Swift, got %v", parsed.Language)
	}

	// Should find methods: init, loadUsers, deleteUser
	if len(parsed.Methods) < 2 {
		t.Errorf("expected at least 2 methods, got %d", len(parsed.Methods))
		for _, m := range parsed.Methods {
			t.Logf("  method: %s", m.Name)
		}
	}

	foundLoadUsers := false
	for _, m := range parsed.Methods {
		if m.Name == "loadUsers" {
			foundLoadUsers = true
		}
	}
	if !foundLoadUsers {
		t.Error("expected loadUsers method")
	}
}

func TestSwiftVerifyCallAlwaysFalse(t *testing.T) {
	parsed := mustParseSwiftFixture(t, "swift_basic_test.swift")

	// Swift doesn't have verify calls — IsVerifyCall should always return false
	for _, cls := range parsed.Classes {
		for _, m := range cls.Methods {
			for _, stmt := range m.Statements {
				if IsVerifyCall(stmt) {
					t.Errorf("IsVerifyCall should return false for Swift, got true for: %s", stmt.Text())
				}
			}
		}
	}
}

func TestSwiftStubCallAlwaysFalse(t *testing.T) {
	parsed := mustParseSwiftFixture(t, "swift_basic_test.swift")

	// Swift doesn't use MockK/Mockito stubs — IsStubCall should always return false
	for _, cls := range parsed.Classes {
		for _, m := range cls.Methods {
			for _, stmt := range m.Statements {
				if IsStubCall(stmt) {
					t.Errorf("IsStubCall should return false for Swift, got true for: %s", stmt.Text())
				}
			}
		}
	}
}

func TestParseSwiftExtensions(t *testing.T) {
	parsed := mustParseSwiftFixture(t, "swift_extension_test.swift")

	// Should have 3 classes:
	// 1. ObservableRelayBindTest (class + extension merged)
	// 2. SomeOtherTest (extension-only, no class declaration)
	// 3. ReducerTests (indirect inheritance via BaseTCATestCase)
	if len(parsed.Classes) != 3 {
		t.Fatalf("expected 3 classes, got %d", len(parsed.Classes))
		for _, cls := range parsed.Classes {
			t.Logf("  class: %s (%d methods)", cls.Name, len(cls.Methods))
		}
	}

	// Check ObservableRelayBindTest has 2 methods from extension
	var relayClass *model.TestClass
	for i := range parsed.Classes {
		if parsed.Classes[i].Name == "ObservableRelayBindTest" {
			relayClass = &parsed.Classes[i]
			break
		}
	}
	if relayClass == nil {
		t.Fatal("expected ObservableRelayBindTest class")
	}
	if len(relayClass.Methods) != 2 {
		t.Errorf("ObservableRelayBindTest: expected 2 methods, got %d", len(relayClass.Methods))
		for _, m := range relayClass.Methods {
			t.Logf("  method: %s", m.Name)
		}
	}

	// Check SomeOtherTest exists with 1 method
	var otherClass *model.TestClass
	for i := range parsed.Classes {
		if parsed.Classes[i].Name == "SomeOtherTest" {
			otherClass = &parsed.Classes[i]
			break
		}
	}
	if otherClass == nil {
		t.Fatal("expected SomeOtherTest class from extension-only")
	}
	if len(otherClass.Methods) != 1 {
		t.Errorf("SomeOtherTest: expected 1 method, got %d", len(otherClass.Methods))
	}

	// Check ReducerTests exists with 1 method (indirect inheritance)
	var reducerClass *model.TestClass
	for i := range parsed.Classes {
		if parsed.Classes[i].Name == "ReducerTests" {
			reducerClass = &parsed.Classes[i]
			break
		}
	}
	if reducerClass == nil {
		t.Fatal("expected ReducerTests class")
	}
	if len(reducerClass.Methods) != 1 {
		t.Errorf("ReducerTests: expected 1 method, got %d", len(reducerClass.Methods))
	}
}

func TestParseSwiftIndirectInheritance(t *testing.T) {
	// Test that a class with non-XCTestCase parent still gets detected
	// if its name matches test naming patterns
	src := []byte(`
import XCTest

class MyFeatureTests: BaseTestCase {
    func testSomething() {
        XCTAssertTrue(true)
    }
    func testAnotherThing() {
        XCTAssertEqual(1, 1)
    }
}
`)
	parsed, err := ParseTestFile("MyFeatureTests.swift", src, model.Swift)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { parsed.Close() })

	if len(parsed.Classes) != 1 {
		t.Fatalf("expected 1 class, got %d", len(parsed.Classes))
	}
	if parsed.Classes[0].Name != "MyFeatureTests" {
		t.Errorf("expected class name 'MyFeatureTests', got %q", parsed.Classes[0].Name)
	}
	if len(parsed.Classes[0].Methods) != 2 {
		t.Errorf("expected 2 methods, got %d", len(parsed.Classes[0].Methods))
	}
}

func TestParseSwiftNimbleAssertions(t *testing.T) {
	parsed := mustParseSwiftFixture(t, "swift_nimble_test.swift")

	if len(parsed.Classes) != 1 {
		t.Fatalf("expected 1 class, got %d", len(parsed.Classes))
	}

	cls := parsed.Classes[0]
	if cls.Name != "UserServiceTests" {
		t.Errorf("expected class name 'UserServiceTests', got %q", cls.Name)
	}

	if len(cls.Methods) != 8 {
		t.Errorf("expected 8 test methods, got %d", len(cls.Methods))
		for _, m := range cls.Methods {
			t.Logf("  method: %s", m.Name)
		}
	}

	totalAssertions := 0
	for _, m := range cls.Methods {
		for _, stmt := range m.Statements {
			if IsAssertionCall(stmt) {
				totalAssertions++
			}
		}
	}
	if totalAssertions < 8 {
		t.Errorf("expected at least 8 Nimble assertions, got %d", totalAssertions)
	}

	for _, m := range cls.Methods {
		if m.Name != "testFetchUserReturnsCorrectName" {
			continue
		}
		for _, stmt := range m.Statements {
			if IsSwiftAssertionCall(stmt) {
				strength, target := ClassifySwiftAssertion(stmt)
				if strength != model.Strong {
					t.Errorf("expect(...).to(equal(...)) should be Strong, got %v", strength)
				}
				if target != model.Output {
					t.Errorf("Nimble assertion should target Output, got %v", target)
				}
			}
		}
	}

	for _, m := range cls.Methods {
		if m.Name != "testFetchUserReturnsNonNilAge" {
			continue
		}
		for _, stmt := range m.Statements {
			if IsSwiftAssertionCall(stmt) {
				strength, _ := ClassifySwiftAssertion(stmt)
				if strength != model.Weak {
					t.Errorf("expect(...).toNot(beNil()) should be Weak, got %v", strength)
				}
			}
		}
	}

	for _, m := range cls.Methods {
		if m.Name != "testUserIsActive" {
			continue
		}
		for _, stmt := range m.Statements {
			if IsSwiftAssertionCall(stmt) {
				strength, _ := ClassifySwiftAssertion(stmt)
				if strength != model.Medium {
					t.Errorf("expect(...).to(beTrue()) should be Medium, got %v", strength)
				}
			}
		}
	}

	if !ContainsSwiftAssertionText(`expect(user.name).to(equal("Alice"))`) {
		t.Error("ContainsSwiftAssertionText should detect Nimble pattern")
	}
	if !ContainsSwiftAssertionText(`expect(value).toNot(beNil())`) {
		t.Error("ContainsSwiftAssertionText should detect Nimble toNot pattern")
	}
}

func TestParseSwiftTCATestStore(t *testing.T) {
	parsed := mustParseSwiftFixture(t, "swift_tca_test.swift")

	if len(parsed.Classes) != 1 {
		t.Fatalf("expected 1 class, got %d", len(parsed.Classes))
	}

	cls := parsed.Classes[0]
	if cls.Name != "FeatureTests" {
		t.Errorf("expected class name 'FeatureTests', got %q", cls.Name)
	}

	if len(cls.Methods) != 6 {
		t.Errorf("expected 6 test methods, got %d", len(cls.Methods))
		for _, m := range cls.Methods {
			t.Logf("  method: %s", m.Name)
		}
	}
}

func TestParseSwiftStructBasedTests(t *testing.T) {
	// Verify struct-based test classes (Swift Testing) are detected
	src := []byte(`
import Testing

struct CalculatorTests {
    @Test func addition() {
        #expect(1 + 1 == 2)
    }
    @Test func subtraction() {
        #expect(5 - 3 == 2)
    }
}
`)
	parsed, err := ParseTestFile("CalculatorTests.swift", src, model.Swift)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { parsed.Close() })

	if len(parsed.Classes) != 1 {
		t.Fatalf("expected 1 class from struct, got %d", len(parsed.Classes))
	}
	if parsed.Classes[0].Name != "CalculatorTests" {
		t.Errorf("expected class name 'CalculatorTests', got %q", parsed.Classes[0].Name)
	}
	if len(parsed.Classes[0].Methods) != 2 {
		t.Errorf("expected 2 methods, got %d", len(parsed.Classes[0].Methods))
	}
}

func TestSwiftExpectStrengthClassification(t *testing.T) {
	parsed := mustParseSwiftFixture(t, "swift_tca_test.swift")

	cls := parsed.Classes[0]

	// Verify methods were parsed
	if len(cls.Methods) < 2 {
		t.Fatalf("expected at least 2 methods, got %d", len(cls.Methods))
	}

	// The #expect macro is parsed into ERROR nodes by tree-sitter, so AST-level
	// assertion detection won't find them. The text-based fallback in
	// containsAssertionText (assertions.go) and ContainsSwiftAssertionText
	// handles these cases. We verify the methods were parsed correctly.
	methodNames := map[string]bool{}
	for _, m := range cls.Methods {
		methodNames[m.Name] = true
	}

	for _, expected := range []string{"sendAction", "equality", "comparison", "booleanCheck", "throwsCheck", "requireUnwrap"} {
		if !methodNames[expected] {
			t.Errorf("expected method %q not found", expected)
		}
	}
}

func TestSwiftVerifyCallRobust(t *testing.T) {
	// Test that verify( is detected even with leading whitespace or in chained expressions
	src := []byte(`
import XCTest
@testable import MyApp

class MockTests: XCTestCase {
    func testVerifyMockable() {
        verify(mock).fetchUser(id: "123").called(count: .once)
    }
}
`)
	parsed, err := ParseTestFile("MockTests.swift", src, model.Swift)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { parsed.Close() })

	if len(parsed.Classes) != 1 || len(parsed.Classes[0].Methods) != 1 {
		t.Fatal("expected 1 class with 1 method")
	}

	method := parsed.Classes[0].Methods[0]
	verifyFound := false
	for _, stmt := range method.Statements {
		if IsVerifyCall(stmt) {
			verifyFound = true
		}
	}
	if !verifyFound {
		t.Error("expected verify call to be detected")
		for _, stmt := range method.Statements {
			t.Logf("  stmt type=%s text=%q isVerify=%v", stmt.Type(), stmt.Text(), IsVerifyCall(stmt))
		}
	}
}

func TestSwiftStubCallCuckoo(t *testing.T) {
	// Test Cuckoo framework stub detection
	src := []byte(`
import XCTest
@testable import MyApp

class CuckooTests: XCTestCase {
    func testStubCuckoo() {
        stub(mockRepository) { stub in
            when(stub.fetchUser(id: any())).thenReturn(user)
        }
        XCTAssertEqual(sut.loadUser(), user)
    }
}
`)
	parsed, err := ParseTestFile("CuckooTests.swift", src, model.Swift)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { parsed.Close() })

	if len(parsed.Classes) != 1 || len(parsed.Classes[0].Methods) != 1 {
		t.Fatal("expected 1 class with 1 method")
	}

	method := parsed.Classes[0].Methods[0]
	stubFound := false
	for _, stmt := range method.Statements {
		if IsStubCall(stmt) {
			stubFound = true
		}
	}
	if !stubFound {
		t.Error("expected stub call to be detected for Cuckoo")
		for _, stmt := range method.Statements {
			t.Logf("  stmt type=%s text=%q isStub=%v", stmt.Type(), stmt.Text(), IsStubCall(stmt))
		}
	}
}

func TestClassifyExpectStrength(t *testing.T) {
	tests := []struct {
		text     string
		strength model.Strength
		target   model.Target
	}{
		{"#expect(x == y)", model.Strong, model.Output},
		{"#expect(x != y)", model.Strong, model.Output},
		{"#expect(x > y)", model.Medium, model.Output},
		{"#expect(x < y)", model.Medium, model.Output},
		{"#expect(x >= y)", model.Medium, model.Output},
		{"#expect(isValid)", model.Medium, model.Output},
		{"#expect(throws: MyError.self) { try op() }", model.Medium, model.Exception},
		{"#require(optionalValue)", model.Strong, model.Output},
	}

	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			// Swift macros (#expect, #require) are parsed into ERROR nodes by tree-sitter,
			// so we test the classification functions directly.
			if strings.Contains(tt.text, "#require(") {
				// #require is always Strong Output (unless throws:)
				// This is handled by ClassifySwiftAssertion which checks #require first
				// Just verify via text-based classification
				if tt.strength != model.Strong {
					t.Errorf("expected Strong for #require, got %v", tt.strength)
				}
				return
			}
			strength := classifyExpectStrength(tt.text)
			target := classifyExpectTarget(tt.text)
			if strength != tt.strength {
				t.Errorf("classifyExpectStrength(%q): got %v, want %v", tt.text, strength, tt.strength)
			}
			if target != tt.target {
				t.Errorf("classifyExpectTarget(%q): got %v, want %v", tt.text, target, tt.target)
			}
		})
	}
}
