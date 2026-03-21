package analyze

import (
	"testing"

	"github.com/timusus/test-confidence/internal/model"
)

func TestDoublesAnalyzer(t *testing.T) {
	parsed := mustParseFixture(t, "kotlin_doubles.kt")
	defer parsed.Close()

	result := AnalyzeDoubles(parsed)

	// 4 class-level + 2 inline = 7 doubles total:
	//   repo (@MockK), api (@MockK), fakeDb (Fake), testRepository (Test),
	//   stubAuth (Stub), logger (inline mockk), testClock (inline fake)
	if len(result.Doubles) < 7 {
		for _, d := range result.Doubles {
			t.Logf("double: var=%s type=%s framework=%v usage=%v", d.VariableName, d.TypeName, d.Framework, d.Usage)
		}
		t.Fatalf("expected at least 7 doubles, got %d", len(result.Doubles))
	}

	usages := map[string]model.Usage{}
	frameworks := map[string]model.Framework{}
	for _, d := range result.Doubles {
		usages[d.VariableName] = d.Usage
		frameworks[d.VariableName] = d.Framework
	}

	// Usage classification
	if usages["repo"] != model.SetupOnly {
		t.Errorf("repo: expected SetupOnly, got %v", usages["repo"])
	}
	if usages["api"] != model.Both {
		t.Errorf("api: expected Both, got %v", usages["api"])
	}
	if usages["fakeDb"] != model.FakeUsage {
		t.Errorf("fakeDb: expected FakeUsage, got %v", usages["fakeDb"])
	}
	if usages["testRepository"] != model.FakeUsage {
		t.Errorf("testRepository: expected FakeUsage, got %v", usages["testRepository"])
	}
	if usages["stubAuth"] != model.FakeUsage {
		t.Errorf("stubAuth: expected FakeUsage, got %v", usages["stubAuth"])
	}
	if usages["testClock"] != model.FakeUsage {
		t.Errorf("testClock: expected FakeUsage, got %v", usages["testClock"])
	}
	if usages["logger"] != model.Verification {
		t.Errorf("logger: expected Verification, got %v", usages["logger"])
	}

	// Framework detection
	if frameworks["repo"] != model.FrameworkMockK {
		t.Errorf("repo: expected MockK framework, got %v", frameworks["repo"])
	}
	if frameworks["fakeDb"] != model.FrameworkFake {
		t.Errorf("fakeDb: expected Fake framework, got %v", frameworks["fakeDb"])
	}
	if frameworks["testRepository"] != model.FrameworkFake {
		t.Errorf("testRepository: expected Fake framework, got %v", frameworks["testRepository"])
	}
	if frameworks["stubAuth"] != model.FrameworkFake {
		t.Errorf("stubAuth: expected Fake framework, got %v", frameworks["stubAuth"])
	}
	if frameworks["testClock"] != model.FrameworkFake {
		t.Errorf("testClock: expected Fake framework, got %v", frameworks["testClock"])
	}

	// Mock density > 0
	if result.MockDensity <= 0 || result.MockDensity > 1 {
		t.Errorf("expected MockDensity between 0 and 1, got %f", result.MockDensity)
	}

	// SetupOnlyRatio: 1 setup-only / 3 non-fake doubles = 0.33
	if result.SetupOnlyRatio < 0.2 || result.SetupOnlyRatio > 0.5 {
		t.Errorf("expected SetupOnlyRatio ~0.33, got %f", result.SetupOnlyRatio)
	}

	// FakeCount: fakeDb + testRepository + stubAuth + testClock = 4
	if result.FakeCount != 4 {
		t.Errorf("expected 4 fakes, got %d", result.FakeCount)
	}

	// MostMockedTypes should have entries
	if len(result.MostMockedTypes) == 0 {
		t.Error("expected most-mocked types")
	}
}

func TestIsFakeFunction(t *testing.T) {
	tests := []struct {
		varName  string
		typeName string
		want     bool
	}{
		// Fake prefix
		{"fakeDb", "FakeDatabase", true},
		{"fakeDb", "", true},
		{"db", "FakeDatabase", true},
		// Test prefix
		{"testRepository", "TestUserRepository", true},
		{"testRepo", "", true},
		{"repo", "TestUserRepository", true},
		// Stub prefix
		{"stubAuth", "StubAuthenticator", true},
		{"stubAuth", "", true},
		{"auth", "StubAuthenticator", true},
		// Not fakes
		{"repository", "UserRepository", false},
		{"testing", "", false},     // "test" prefix but lowercase next char
		{"stubborn", "", false},    // "stub" prefix but lowercase next char
		{"fakeable", "", false},    // "fake" prefix but lowercase next char
		{"sut", "ViewModel", false},
		{"mock", "MockService", false}, // Mock prefix is not a fake
	}

	for _, tc := range tests {
		got := isFake(tc.varName, tc.typeName)
		if got != tc.want {
			t.Errorf("isFake(%q, %q) = %v, want %v", tc.varName, tc.typeName, got, tc.want)
		}
	}
}
