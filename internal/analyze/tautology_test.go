package analyze

import (
	"os"
	"strings"
	"testing"

	"github.com/timusus/test-confidence/internal/model"
	"github.com/timusus/test-confidence/internal/parse"
)

func TestTautologyDetection(t *testing.T) {
	parsed := mustParseFixture(t, "kotlin_tautology.kt")
	defer parsed.Close()

	results := AnalyzeTautology(parsed, nil) // nil = no production file

	if len(results) != 2 {
		t.Errorf("expected 2 tautology flags, got %d", len(results))
		for _, r := range results {
			t.Logf("  flagged: %q stub=%q assert=%q", r.Method, r.StubIdentifier, r.AssertIdentifier)
		}
	}

	// Verify no false positives
	for _, r := range results {
		if strings.Contains(r.Method, "not tautological") {
			t.Errorf("false positive: flagged %q", r.Method)
		}
	}

	// Check identifiers are captured
	for _, r := range results {
		if r.StubIdentifier == "" || r.AssertIdentifier == "" {
			t.Error("expected identifiers to be captured")
		}
	}

	// SUTSimple should be nil (no production file provided)
	for _, r := range results {
		if r.SUTSimple != nil {
			t.Error("expected SUTSimple=nil without production file")
		}
	}
}

func TestParseProductionFile(t *testing.T) {
	src, err := os.ReadFile("../../testdata/fixtures/kotlin_passthrough_sut.kt")
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := parse.ParseProductionFile("test.kt", src, model.Kotlin)
	if err != nil {
		t.Fatal(err)
	}
	defer parsed.Close()

	if len(parsed.Methods) < 3 {
		t.Fatalf("expected at least 3 methods, got %d", len(parsed.Methods))
	}

	// getUser() is simple delegation
	var getUser *model.ProductionMethod
	for i, m := range parsed.Methods {
		if m.Name == "getUser" {
			getUser = &parsed.Methods[i]
		}
	}
	if getUser == nil {
		t.Fatal("getUser not found")
	}
	if !getUser.IsSingleExpr {
		t.Error("getUser should be single expression")
	}
	if getUser.BranchCount != 0 {
		t.Errorf("getUser should have 0 branches, got %d", getUser.BranchCount)
	}

	// computeSomething() has branches
	var compute *model.ProductionMethod
	for i, m := range parsed.Methods {
		if m.Name == "computeSomething" {
			compute = &parsed.Methods[i]
		}
	}
	if compute == nil {
		t.Fatal("computeSomething not found")
	}
	if compute.IsSingleExpr {
		t.Error("computeSomething should NOT be single expression")
	}
	if compute.BranchCount == 0 {
		t.Error("computeSomething should have branches")
	}
}

func TestTautologyWithSUTComplexity(t *testing.T) {
	testParsed := mustParseFixture(t, "kotlin_tautology.kt")
	defer testParsed.Close()

	prodSrc, err := os.ReadFile("../../testdata/fixtures/kotlin_passthrough_sut.kt")
	if err != nil {
		t.Fatal(err)
	}
	prodParsed, err := parse.ParseProductionFile("test.kt", prodSrc, model.Kotlin)
	if err != nil {
		t.Fatal(err)
	}
	defer prodParsed.Close()

	results := AnalyzeTautology(testParsed, prodParsed)

	for _, r := range results {
		if strings.Contains(r.Method, "direct match") {
			if r.SUTSimple == nil || !*r.SUTSimple {
				t.Error("expected SUTSimple=true for passthrough getUser()")
			}
		}
	}
}

func TestPassThroughDetection(t *testing.T) {
	parsed := mustParseFixture(t, "kotlin_passthrough.kt")
	defer parsed.Close()

	results := AnalyzeTautology(parsed, nil)

	if len(results) != 1 {
		t.Errorf("expected 1 pass-through flag, got %d", len(results))
		for _, r := range results {
			t.Logf("  flagged: %q stub=%q assert=%q", r.Method, r.StubIdentifier, r.AssertIdentifier)
		}
		return
	}

	r := results[0]
	if r.Method != "testGetUser" {
		t.Errorf("expected method testGetUser, got %q", r.Method)
	}
	if r.StubIdentifier != "expected" {
		t.Errorf("expected stub identifier 'expected', got %q", r.StubIdentifier)
	}
	if r.AssertIdentifier != "expected" {
		t.Errorf("expected assert identifier 'expected', got %q", r.AssertIdentifier)
	}
}
