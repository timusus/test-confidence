package analyze

import (
	"testing"

	"github.com/timusus/test-confidence/internal/model"
)

func TestStructuralAnalyzer(t *testing.T) {
	parsed := mustParseFixture(t, "kotlin_structural.kt")
	defer parsed.Close()

	results := AnalyzeStructural(parsed)

	types := map[model.StructuralType]int{}
	for _, r := range results {
		types[r.Type]++
	}

	if types[model.VerifyOrdering] != 1 {
		t.Errorf("expected 1 VerifyOrdering, got %d", types[model.VerifyOrdering])
	}
	if types[model.ArgumentCaptor] != 1 {
		t.Errorf("expected 1 ArgumentCaptor, got %d", types[model.ArgumentCaptor])
	}
	if types[model.StubAndVerify] != 1 {
		t.Errorf("expected 1 StubAndVerify, got %d", types[model.StubAndVerify])
	}
	if types[model.VerifyOnlyTest] != 1 {
		t.Errorf("expected 1 VerifyOnlyTest, got %d", types[model.VerifyOnlyTest])
	}

	// Total should be exactly 4
	if len(results) != 4 {
		t.Errorf("expected 4 indicators, got %d", len(results))
	}

	// All should have line numbers and method names
	for _, r := range results {
		if r.Line == 0 {
			t.Errorf("indicator %v has no line number", r.Type)
		}
		if r.Method == "" {
			t.Errorf("indicator %v has no method name", r.Type)
		}
	}
}
