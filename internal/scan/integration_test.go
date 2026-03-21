package scan

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/timusus/test-confidence/internal/config"
	"github.com/timusus/test-confidence/internal/report"
)

func TestFullScan(t *testing.T) {
	cfg := config.DefaultConfig()
	output, err := Scan("../../testdata/projects/android-simple", cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	result := output.Result

	// Anti-patterns should be found
	totalAntiPatterns := 0
	for _, f := range result.FileResults {
		totalAntiPatterns += len(f.AntiPatterns)
	}
	if totalAntiPatterns == 0 {
		t.Error("expected anti-patterns (Thread.sleep, @Ignore, etc.)")
	}

	// Assertions should be analyzed
	totalAssertions := 0
	for _, f := range result.FileResults {
		totalAssertions += f.Assertions.TotalAssertions
	}
	if totalAssertions == 0 {
		t.Error("expected assertions to be counted")
	}

	// Doubles should be detected
	totalDoubles := 0
	for _, f := range result.FileResults {
		totalDoubles += len(f.Doubles.Doubles)
	}
	if totalDoubles == 0 {
		t.Error("expected doubles to be detected")
	}

	// Terminal output renders without error
	var buf bytes.Buffer
	if err := report.WriteTerminalReport(&buf, result, false, nil, output.SurfaceAnalysis, nil, nil, 0, nil); err != nil {
		t.Errorf("terminal report failed: %v", err)
	}
	if buf.Len() == 0 {
		t.Error("expected non-empty terminal output")
	}

	// JSON output is valid
	var jsonBuf bytes.Buffer
	if err := report.WriteJSONReport(&jsonBuf, result, nil, output.SurfaceAnalysis, nil); err != nil {
		t.Errorf("JSON report failed: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(jsonBuf.Bytes(), &parsed); err != nil {
		t.Errorf("invalid JSON: %v", err)
	}
}
