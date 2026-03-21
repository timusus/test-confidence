package analyze

import (
	"testing"

	"github.com/timusus/test-confidence/internal/model"
)

func TestAssertionAnalyzer(t *testing.T) {
	parsed := mustParseFixture(t, "kotlin_assertions.kt")
	defer parsed.Close()

	result := AnalyzeAssertions(parsed)

	// TotalAssertions = 3 strong + 2 medium + 1 weak + 1 exception = 7 (verify NOT counted)
	if result.TotalAssertions != 7 {
		t.Errorf("expected 7 total assertions (excluding verify), got %d", result.TotalAssertions)
	}

	// Strength distribution
	if result.StrengthDistribution[model.Strong] != 3 {
		t.Errorf("expected 3 strong, got %d", result.StrengthDistribution[model.Strong])
	}
	if result.StrengthDistribution[model.Medium] != 2 {
		t.Errorf("expected 2 medium, got %d", result.StrengthDistribution[model.Medium])
	}
	if result.StrengthDistribution[model.Weak] != 1 {
		t.Errorf("expected 1 weak, got %d", result.StrengthDistribution[model.Weak])
	}

	// Target distribution
	if result.TargetDistribution[model.Output] != 6 {
		t.Errorf("expected 6 output assertions, got %d", result.TargetDistribution[model.Output])
	}
	if result.TargetDistribution[model.Interaction] != 2 {
		t.Errorf("expected 2 interaction (verify), got %d", result.TargetDistribution[model.Interaction])
	}
	if result.TargetDistribution[model.Exception] != 1 {
		t.Errorf("expected 1 exception assertion, got %d", result.TargetDistribution[model.Exception])
	}

	// Zero-assertion methods
	if len(result.ZeroAssertionMethods) != 1 {
		t.Errorf("expected 1 zero-assertion method, got %d", len(result.ZeroAssertionMethods))
	}

	// OutputInteractionRatio = 7 output+exception / (7 + 2 interaction) = 7/9 ≈ 0.778
	if result.OutputInteractionRatio < 0.7 || result.OutputInteractionRatio > 0.85 {
		t.Errorf("expected ratio ~0.78, got %f", result.OutputInteractionRatio)
	}
}

func TestTurbineAssertionCounting(t *testing.T) {
	parsed := mustParseFixture(t, "kotlin_turbine_assertions.kt")
	defer parsed.Close()

	result := AnalyzeAssertions(parsed)

	// Method 1: Turbine .test { } with 3 assertThat() calls inside = 3 assertions
	// Method 2: Turbine .test { } with 2 shouldBe infix calls inside = 2 assertions
	// Method 3: 3 custom assertion helpers (assertColorSchemesEqual, verifySnapshot, validateResponse) = 3 assertions
	//   - assertColorSchemesEqual is caught by AST (prefix "assert"), counted as 1
	//   - verifySnapshot and validateResponse fall to text fallback custom prefix matching
	// Method 4: 1 AST-level assertThat + Turbine .test { } with 2 assertThat() = 3 assertions
	// Total: 3 + 2 + 3 + 3 = 11
	if result.TotalAssertions != 11 {
		t.Errorf("expected 11 total assertions, got %d", result.TotalAssertions)
	}

	// No zero-assertion methods — every method has at least 1 assertion
	if len(result.ZeroAssertionMethods) != 0 {
		t.Errorf("expected 0 zero-assertion methods, got %d", len(result.ZeroAssertionMethods))
	}
}

func TestCountAssertionMatches(t *testing.T) {
	tests := []struct {
		name string
		text string
		want int
	}{
		{
			name: "single assertThat in text",
			text: "assertThat(result).isEqualTo(42)",
			want: 1,
		},
		{
			name: "multiple assertThat in Turbine block",
			text: `flow.test {
            assertThat(awaitItem()).isEqualTo(first)
            assertThat(awaitItem()).isEqualTo(second)
            assertThat(awaitItem()).isEqualTo(third)
        }`,
			want: 3,
		},
		{
			name: "multiple shouldBe in Turbine block",
			text: `flow.test {
            awaitItem() shouldBe expected1
            awaitItem() shouldBe expected2
        }`,
			want: 2,
		},
		{
			name: "custom assertion helper with assert prefix",
			text: "assertColorSchemesEqual(expected, actual)",
			want: 1,
		},
		{
			name: "custom helper with verify prefix",
			text: "verifySnapshot(component)",
			want: 1,
		},
		{
			name: "custom helper with validate prefix",
			text: "validateResponse(response)",
			want: 1,
		},
		{
			name: "custom helper with check prefix",
			text: "checkConstraints(model)",
			want: 1,
		},
		{
			name: "no assertions",
			text: "sut.doSomething()",
			want: 0,
		},
		{
			name: "mixed known and custom not double-counted",
			text: "assertThat(result).isEqualTo(42)",
			// assertThat( is a known pattern, so custom prefix check is skipped
			want: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := countAssertionMatches(tt.text)
			if got != tt.want {
				t.Errorf("countAssertionMatches(%q) = %d, want %d", tt.text, got, tt.want)
			}
		})
	}
}

func TestContainsAssertionTextSwiftPatterns(t *testing.T) {
	tests := []struct {
		text string
		want bool
	}{
		// TCA TestStore patterns
		{"store.send(.buttonTapped) { $0.count = 1 }", true},
		{"await store.receive(.response(.success(\"ok\")))", true},
		// Custom assertion helpers
		{"XCTAssertThrows(try riskyOp())", true},
		{"XCTExpectFailure(\"Known issue\")", true},
		// Async expectations
		{"waitForExpectations(timeout: 5.0)", true},
		{"expectation.fulfill()", true},
		// Swift Testing
		{"#expect(result == 42)", true},
		{"#require(optionalValue)", true},
		// Non-assertion
		{"let x = store.state", false},
		{"print(result)", false},
	}

	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			got := containsAssertionText(tt.text)
			if got != tt.want {
				t.Errorf("containsAssertionText(%q) = %v, want %v", tt.text, got, tt.want)
			}
		})
	}
}

func TestTestExpectedAnnotation(t *testing.T) {
	parsed := mustParseFixture(t, "kotlin_test_expected.kt")
	defer parsed.Close()

	result := AnalyzeAssertions(parsed)

	// Method 1: @Test(expected = IllegalArgumentException::class) = 1 exception assertion
	// Method 2: @Test + assertThat().isEqualTo() = 1 strong output assertion
	// Method 3: @Test(expected = NullPointerException::class) = 1 exception assertion
	// Total: 3 assertions
	if result.TotalAssertions != 3 {
		t.Errorf("expected 3 total assertions, got %d", result.TotalAssertions)
	}

	// Exception target: 2 from @Test(expected=...)
	if result.TargetDistribution[model.Exception] != 2 {
		t.Errorf("expected 2 exception assertions, got %d", result.TargetDistribution[model.Exception])
	}

	// Output target: 1 from assertThat().isEqualTo()
	if result.TargetDistribution[model.Output] != 1 {
		t.Errorf("expected 1 output assertion, got %d", result.TargetDistribution[model.Output])
	}

	// No zero-assertion methods — all 3 have either @Test(expected) or an assertion
	if len(result.ZeroAssertionMethods) != 0 {
		t.Errorf("expected 0 zero-assertion methods, got %d", len(result.ZeroAssertionMethods))
	}
}
