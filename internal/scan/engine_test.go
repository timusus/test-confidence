package scan

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/timusus/test-confidence/internal/config"
	"github.com/timusus/test-confidence/internal/model"
)

func TestScanEngine(t *testing.T) {
	cfg := config.DefaultConfig()
	output, err := Scan("../../testdata/projects/android-simple", cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	result := output.Result

	if result.TotalTestFiles == 0 {
		t.Error("expected test files")
	}
	if result.Language != model.Kotlin {
		t.Error("expected Kotlin language")
	}
	if len(result.FileResults) == 0 {
		t.Error("expected per-file results")
	}
	if result.TotalTestMethods == 0 {
		t.Error("expected test methods counted")
	}

	// Verify analyzers actually ran
	hasAntiPattern := false
	hasAssertions := false
	hasDoubles := false
	for _, f := range result.FileResults {
		if len(f.AntiPatterns) > 0 {
			hasAntiPattern = true
		}
		if f.Assertions.TotalAssertions > 0 {
			hasAssertions = true
		}
		if len(f.Doubles.Doubles) > 0 {
			hasDoubles = true
		}
	}
	// At least some analyzer should have found something (the fixture has test content)
	if !hasAntiPattern && !hasAssertions && !hasDoubles {
		t.Error("expected at least one analyzer to produce results")
	}
}

func TestScanWithLanguageOverride(t *testing.T) {
	cfg := config.DefaultConfig()
	swift := model.Swift
	output, err := Scan("../../testdata/projects/android-simple", cfg, &swift)
	if err != nil {
		t.Fatal(err)
	}
	if output.Result.Language != model.Swift {
		t.Error("expected Swift language override")
	}
}

func TestScanWithUnparseableFile(t *testing.T) {
	// Create a temp dir with a valid platform marker and an unparseable test file
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "build.gradle.kts"), []byte(""), 0644)
	os.MkdirAll(filepath.Join(dir, "src/test/kotlin/com/example"), 0755)
	// Write binary garbage as a "test file"
	os.WriteFile(filepath.Join(dir, "src/test/kotlin/com/example/BrokenTest.kt"), []byte{0xFF, 0xFE, 0x00}, 0644)
	// Write a valid test file too
	os.WriteFile(filepath.Join(dir, "src/test/kotlin/com/example/GoodTest.kt"),
		[]byte("package com.example\nimport org.junit.Test\nclass GoodTest {\n@Test fun test() { assertEquals(1,1) }\n}"), 0644)

	cfg := config.DefaultConfig()
	output, err := Scan(dir, cfg, nil)
	if err != nil {
		t.Fatalf("scan should not fail on unparseable files: %v", err)
	}
	// The broken file might parse (tree-sitter is lenient) or fail. Either way, no crash.
	// If it does fail, UnparseableFiles should be > 0
	if output.Result.TotalTestFiles == 0 && output.Result.UnparseableFiles == 0 {
		t.Error("expected either parsed files or unparseable count")
	}
}
