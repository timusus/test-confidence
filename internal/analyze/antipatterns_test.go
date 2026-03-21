package analyze

import (
	"os"
	"testing"

	"github.com/timusus/test-confidence/internal/config"
	"github.com/timusus/test-confidence/internal/model"
	"github.com/timusus/test-confidence/internal/parse"
)

func TestAntiPatternAnalyzer(t *testing.T) {
	parsed := mustParseFixture(t, "kotlin_antipatterns.kt")
	defer parsed.Close()

	cfg := config.DefaultConfig()
	results := AnalyzeAntiPatterns(parsed, cfg)

	counts := map[model.AntiPatternType]int{}
	for _, r := range results {
		counts[r.Type]++
	}

	assertCount(t, counts, model.ThreadSleep, 1)
	assertCount(t, counts, model.IgnoredTest, 1)
	assertCount(t, counts, model.EmptyTest, 1)
	assertCount(t, counts, model.ConditionalLogic, 1)
	assertCount(t, counts, model.ReflectionUsage, 1)

	// Verify ALL are Tier1
	for _, r := range results {
		if r.Severity != model.Tier1 {
			t.Errorf("anti-pattern %v should be Tier1, got %v", r.Type, r.Severity)
		}
	}

	// Verify line numbers are set
	for _, r := range results {
		if r.Line == 0 {
			t.Errorf("anti-pattern %v has no line number", r.Type)
		}
	}

	// Total should be exactly 5 (no false positives on "normal test")
	if len(results) != 5 {
		t.Errorf("expected exactly 5 anti-patterns, got %d", len(results))
	}
}

func assertCount(t *testing.T, counts map[model.AntiPatternType]int, typ model.AntiPatternType, expected int) {
	t.Helper()
	if counts[typ] != expected {
		t.Errorf("expected %d %v, got %d", expected, typ, counts[typ])
	}
}

func mustParseFixture(t *testing.T, filename string) *model.ParsedTestFile {
	t.Helper()
	src, err := os.ReadFile("../../testdata/fixtures/" + filename)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := parse.ParseTestFile("../../testdata/fixtures/"+filename, src, model.Kotlin)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}

func mustParseSwiftFixture(t *testing.T, filename string) *model.ParsedTestFile {
	t.Helper()
	src, err := os.ReadFile("../../testdata/fixtures/" + filename)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := parse.ParseTestFile("../../testdata/fixtures/"+filename, src, model.Swift)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}

func TestGodTestClassPerClass(t *testing.T) {
	parsed := mustParseSwiftFixture(t, "swift_god_class_test.swift")
	defer parsed.Close()

	cfg := config.DefaultConfig()
	results := AnalyzeAntiPatterns(parsed, cfg)

	// Collect god test class findings
	var godClassResults []model.AntiPattern
	for _, r := range results {
		if r.Type == model.GodTestClass {
			godClassResults = append(godClassResults, r)
		}
	}

	// Only GodTestClassExample (25 methods) should be flagged, not SmallTestClass (5 methods)
	if len(godClassResults) != 1 {
		t.Fatalf("expected exactly 1 god test class finding, got %d", len(godClassResults))
	}

	if godClassResults[0].Class != "LargeFeatureTests" {
		t.Errorf("expected god class name %q, got %q", "LargeFeatureTests", godClassResults[0].Class)
	}

	// Verify class name is set (not file-level detection)
	if godClassResults[0].Class == "" {
		t.Error("god test class finding should include the class name")
	}
}

func TestSwiftSleepAntiPatterns(t *testing.T) {
	parsed := mustParseSwiftFixture(t, "swift_sleep_patterns_test.swift")
	defer parsed.Close()

	cfg := config.DefaultConfig()
	results := AnalyzeAntiPatterns(parsed, cfg)

	sleepCount := 0
	for _, r := range results {
		if r.Type == model.ThreadSleep {
			sleepCount++
			t.Logf("sleep detected in method %q at line %d", r.Method, r.Line)
		}
	}

	// Should detect: Task.sleep, usleep, sleep, DispatchQueue.main.asyncAfter = 4 sleep patterns
	if sleepCount != 4 {
		t.Errorf("expected 4 sleep anti-patterns, got %d", sleepCount)
		for _, r := range results {
			t.Logf("  anti-pattern: %v method=%s line=%d", r.Type, r.Method, r.Line)
		}
	}
}
