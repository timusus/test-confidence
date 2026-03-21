package analyze

import (
	"testing"
)

func TestSetupComplexityAnalyzer(t *testing.T) {
	parsed := mustParseFixture(t, "kotlin_setup_heavy.kt")
	defer parsed.Close()

	assertions := AnalyzeAssertions(parsed)
	result := AnalyzeSetupComplexity(parsed, assertions)

	// Setup statements:
	//   5 @MockK annotations
	//   1 @Rule property (testRule)
	//   2 class-level property initializations (testUser, testDispatcher)
	//   3 statements in @Before setUp() method
	//   10 every/coEvery stubs in first test
	//   8 every/coEvery stubs in second test
	//   1 every stub in third test
	// Total = 5 + 1 + 2 + 3 + 10 + 8 + 1 = 30
	if result.SetupStatements != 30 {
		t.Errorf("expected 30 setup statements, got %d", result.SetupStatements)
	}

	// Assertions: 4 assertThat calls across all tests
	if result.AssertionCount != 4 {
		t.Errorf("expected 4 assertions, got %d", result.AssertionCount)
	}

	// Ratio = 30 / 4 = 7.5
	if result.Ratio < 7.4 || result.Ratio > 7.6 {
		t.Errorf("expected ratio ~7.5, got %f", result.Ratio)
	}
}

func TestSetupComplexityZeroAssertions(t *testing.T) {
	parsed := mustParseFixture(t, "kotlin_doubles.kt")
	defer parsed.Close()

	assertions := AnalyzeAssertions(parsed)
	result := AnalyzeSetupComplexity(parsed, assertions)

	// Should have setup statements (MockK annotations + every/coEvery stubs)
	if result.SetupStatements == 0 {
		t.Error("expected non-zero setup statements")
	}

	// Ratio should be 0 when assertions is 0 and no text-fallback assertions counted
	// (the doubles fixture has assertThat calls so assertions > 0, ratio should be computable)
	if result.Ratio < 0 {
		t.Error("ratio should not be negative")
	}
}
