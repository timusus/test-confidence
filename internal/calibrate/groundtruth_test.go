package calibrate

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadGroundTruth(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "calibration.yaml")

	content := `annotations:
  - file: "src/test/FooTest.kt"
    structural_coupling: high
    behavioral: true
    fragile: true
    has_anti_patterns: false
    has_tautologies: true
    mock_placement: "boundary"
    notes: "Test notes"
  - file: "src/test/BarTest.kt"
    structural_coupling: low
    behavioral: false
`
	os.WriteFile(path, []byte(content), 0644)

	gt, err := LoadGroundTruth(path)
	if err != nil {
		t.Fatalf("LoadGroundTruth() error: %v", err)
	}

	if len(gt.Annotations) != 2 {
		t.Fatalf("got %d annotations, want 2", len(gt.Annotations))
	}

	a := gt.Annotations[0]
	if a.File != "src/test/FooTest.kt" {
		t.Errorf("file = %q, want %q", a.File, "src/test/FooTest.kt")
	}
	if a.StructuralCoupling != CouplingHigh {
		t.Errorf("structural_coupling = %q, want %q", a.StructuralCoupling, CouplingHigh)
	}
	if a.Behavioral == nil || !*a.Behavioral {
		t.Error("behavioral should be true")
	}
	if a.Fragile == nil || !*a.Fragile {
		t.Error("fragile should be true")
	}
	if a.HasAntiPatterns == nil || *a.HasAntiPatterns {
		t.Error("has_anti_patterns should be false")
	}
	if a.HasTautologies == nil || !*a.HasTautologies {
		t.Error("has_tautologies should be true")
	}
	if a.MockPlacement == nil || *a.MockPlacement != "boundary" {
		t.Errorf("mock_placement = %v, want boundary", a.MockPlacement)
	}
	if a.Notes != "Test notes" {
		t.Errorf("notes = %q, want %q", a.Notes, "Test notes")
	}
}

func TestLoadGroundTruthInvalidCoupling(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "calibration.yaml")

	content := `annotations:
  - file: "FooTest.kt"
    structural_coupling: extreme
`
	os.WriteFile(path, []byte(content), 0644)

	_, err := LoadGroundTruth(path)
	if err == nil {
		t.Error("expected error for invalid structural_coupling")
	}
}

func TestLoadGroundTruthInvalidPlacement(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "calibration.yaml")

	content := `annotations:
  - file: "FooTest.kt"
    mock_placement: "wrong"
`
	os.WriteFile(path, []byte(content), 0644)

	_, err := LoadGroundTruth(path)
	if err == nil {
		t.Error("expected error for invalid mock_placement")
	}
}

func TestLoadGroundTruthEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "calibration.yaml")

	content := `annotations: []`
	os.WriteFile(path, []byte(content), 0644)

	_, err := LoadGroundTruth(path)
	if err == nil {
		t.Error("expected error for empty annotations")
	}
}

func TestLoadGroundTruthMissingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "calibration.yaml")

	content := `annotations:
  - structural_coupling: low
`
	os.WriteFile(path, []byte(content), 0644)

	_, err := LoadGroundTruth(path)
	if err == nil {
		t.Error("expected error for missing file field")
	}
}

func TestLoadGroundTruthFileNotFound(t *testing.T) {
	_, err := LoadGroundTruth("/nonexistent/path.yaml")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestFindCalibrationFile(t *testing.T) {
	dir := t.TempDir()

	// No file exists
	if got := FindCalibrationFile(dir); got != "" {
		t.Errorf("expected empty, got %q", got)
	}

	// Create the file
	path := filepath.Join(dir, ".confidence-calibration.yaml")
	os.WriteFile(path, []byte("annotations:\n  - file: test\n"), 0644)

	if got := FindCalibrationFile(dir); got != path {
		t.Errorf("expected %q, got %q", path, got)
	}
}

func TestMatchesFile(t *testing.T) {
	tests := []struct {
		analysis   string
		annotation string
		expected   bool
	}{
		{"/abs/path/src/test/FooTest.kt", "/abs/path/src/test/FooTest.kt", true},
		{"/abs/path/src/test/FooTest.kt", "src/test/FooTest.kt", true},
		{"src/test/FooTest.kt", "/abs/path/src/test/FooTest.kt", true},
		{"/abs/path/src/test/FooTest.kt", "src/test/BarTest.kt", false},
		{"/abs/path/FooTest.kt", "different/FooTest.kt", false},
	}

	for _, tt := range tests {
		got := MatchesFile(tt.analysis, tt.annotation)
		if got != tt.expected {
			t.Errorf("MatchesFile(%q, %q) = %v, want %v", tt.analysis, tt.annotation, got, tt.expected)
		}
	}
}

func TestResolveAnnotationPath(t *testing.T) {
	got := ResolveAnnotationPath("src/test/Foo.kt", "/project")
	want := filepath.Clean("/project/src/test/Foo.kt")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	got = ResolveAnnotationPath("/abs/Foo.kt", "/project")
	want = filepath.Clean("/abs/Foo.kt")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
