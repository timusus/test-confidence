package calibrate

import (
	"bytes"
	"strings"
	"testing"

	"github.com/timusus/test-confidence/internal/model"
)

func TestWriteReport(t *testing.T) {
	result := &CalibrationResult{
		TotalAnnotated: 5,
		Matched:        4,
		Unmatched:      []string{"missing/File.kt"},
		Signals: []SignalResult{
			{Signal: "Behavioral classification", Tier: model.Tier2, Agreed: 3, Total: 4},
			{Signal: "Anti-pattern detection", Tier: model.Tier1, Agreed: 4, Total: 4},
			{Signal: "Mock placement", Tier: model.Tier3, Agreed: 2, Total: 3,
				Misses: []Mismatch{{File: "src/test/Foo.kt", Expected: "boundary", Got: "internal"}}},
		},
	}

	var buf bytes.Buffer
	err := WriteReport(&buf, result)
	if err != nil {
		t.Fatalf("WriteReport() error: %v", err)
	}

	output := buf.String()

	// Check key sections are present
	checks := []string{
		"Calibration Results (5 annotated files, 4 matched)",
		"Unmatched annotations (1)",
		"missing/File.kt",
		"Behavioral classification",
		"Anti-pattern detection",
		"Mock placement",
		"Per-Tier Accuracy",
		"Tier 1",
		"Tier 2",
		"Tier 3",
		"Overall weighted accuracy",
		"Mismatches",
		"expected: boundary, got: internal",
	}

	for _, check := range checks {
		if !strings.Contains(output, check) {
			t.Errorf("output missing %q", check)
		}
	}
}

func TestWriteReportNoSignals(t *testing.T) {
	result := &CalibrationResult{
		TotalAnnotated: 2,
		Matched:        0,
		Unmatched:      []string{"a.kt", "b.kt"},
	}

	var buf bytes.Buffer
	err := WriteReport(&buf, result)
	if err != nil {
		t.Fatalf("WriteReport() error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "No signals to compare") {
		t.Error("expected 'No signals to compare' message")
	}
}

func TestWriteReportNoMismatches(t *testing.T) {
	result := &CalibrationResult{
		TotalAnnotated: 2,
		Matched:        2,
		Signals: []SignalResult{
			{Signal: "Test signal", Tier: model.Tier1, Agreed: 2, Total: 2},
		},
	}

	var buf bytes.Buffer
	err := WriteReport(&buf, result)
	if err != nil {
		t.Fatalf("WriteReport() error: %v", err)
	}

	output := buf.String()
	if strings.Contains(output, "Mismatches") {
		t.Error("should not show Mismatches section when there are none")
	}
}
