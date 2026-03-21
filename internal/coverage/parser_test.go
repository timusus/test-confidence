package coverage

import (
	"os"
	"path/filepath"
	"testing"
)

const testXML = `<?xml version="1.0" ?>
<report name="test">
  <package name="com/example/app/main">
    <sourcefile name="MainViewModel.kt">
      <counter type="LINE" missed="10" covered="40"/>
      <counter type="BRANCH" missed="4" covered="12"/>
    </sourcefile>
    <sourcefile name="Utils.kt">
      <counter type="LINE" missed="0" covered="20"/>
      <counter type="BRANCH" missed="0" covered="0"/>
    </sourcefile>
  </package>
  <package name="com/example/app/detail">
    <sourcefile name="DetailScreen.kt">
      <counter type="LINE" missed="30" covered="10"/>
      <counter type="BRANCH" missed="8" covered="2"/>
    </sourcefile>
  </package>
</report>`

func TestParseJacocoXML(t *testing.T) {
	tmpDir := t.TempDir()
	xmlPath := filepath.Join(tmpDir, "report.xml")
	if err := os.WriteFile(xmlPath, []byte(testXML), 0644); err != nil {
		t.Fatal(err)
	}

	report, err := ParseJacocoXML(xmlPath)
	if err != nil {
		t.Fatal(err)
	}

	if len(report.Files) != 3 {
		t.Fatalf("expected 3 files, got %d", len(report.Files))
	}

	// Check MainViewModel.kt
	mv, ok := report.Files["MainViewModel.kt"]
	if !ok {
		t.Fatal("MainViewModel.kt not found")
	}
	if mv.LinesCovered != 40 || mv.LinesMissed != 10 {
		t.Errorf("MainViewModel lines: covered=%d missed=%d", mv.LinesCovered, mv.LinesMissed)
	}
	if mv.LineCoverage < 0.79 || mv.LineCoverage > 0.81 {
		t.Errorf("MainViewModel line coverage: %.2f", mv.LineCoverage)
	}
	if mv.Package != "com/example/app/main" {
		t.Errorf("MainViewModel package: %s", mv.Package)
	}

	// Check DetailScreen.kt (low coverage)
	ds := report.Files["DetailScreen.kt"]
	if ds.LineCoverage < 0.24 || ds.LineCoverage > 0.26 {
		t.Errorf("DetailScreen line coverage: %.2f", ds.LineCoverage)
	}

	// Aggregates
	if report.TotalLinesCovered() != 70 {
		t.Errorf("total covered: %d", report.TotalLinesCovered())
	}
	if report.TotalLinesMissed() != 40 {
		t.Errorf("total missed: %d", report.TotalLinesMissed())
	}
	overall := report.OverallLineCoverage()
	// 70/110 = ~0.636
	if overall < 0.63 || overall > 0.64 {
		t.Errorf("overall coverage: %.3f", overall)
	}
}

func TestLookupCoverage(t *testing.T) {
	report := &CoverageReport{
		Files: map[string]FileCoverage{
			"MainViewModel.kt": {SourceFile: "MainViewModel.kt", LineCoverage: 0.8},
		},
	}

	// Exact match
	fc := LookupCoverage(report, "/some/path/to/MainViewModel.kt")
	if fc == nil {
		t.Fatal("expected match for MainViewModel.kt")
	}
	if fc.LineCoverage != 0.8 {
		t.Errorf("expected 0.8, got %.2f", fc.LineCoverage)
	}

	// No match
	fc = LookupCoverage(report, "/some/path/to/Unknown.kt")
	if fc != nil {
		t.Error("expected nil for Unknown.kt")
	}

	// Nil report
	fc = LookupCoverage(nil, "/some/path/to/MainViewModel.kt")
	if fc != nil {
		t.Error("expected nil for nil report")
	}
}

func TestParseJacocoXML_FileNotFound(t *testing.T) {
	_, err := ParseJacocoXML("/nonexistent/path.xml")
	if err == nil {
		t.Error("expected error for missing file")
	}
}
