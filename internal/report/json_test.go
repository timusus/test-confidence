package report

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/timusus/test-confidence/internal/model"
)

func TestJSONReport(t *testing.T) {
	result := buildTestScanResult()
	var buf bytes.Buffer
	err := WriteJSONReport(&buf, result, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	for _, key := range []string{"path", "language", "totalTestFiles", "fileResults"} {
		if _, ok := parsed[key]; !ok {
			t.Errorf("missing key: %s", key)
		}
	}
}

func TestJSONReportEmpty(t *testing.T) {
	result := &model.ScanResult{Path: ".", Language: model.Kotlin}
	var buf bytes.Buffer
	err := WriteJSONReport(&buf, result, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if parsed["path"] != "." {
		t.Error("expected path to be '.'")
	}
}

func TestJSONReportAntiPatternTier(t *testing.T) {
	result := buildTestScanResult()
	var buf bytes.Buffer
	err := WriteJSONReport(&buf, result, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	fileResults, ok := parsed["fileResults"].([]interface{})
	if !ok || len(fileResults) == 0 {
		t.Fatal("expected fileResults array")
	}

	first := fileResults[0].(map[string]interface{})
	antiPatterns, ok := first["antiPatterns"].([]interface{})
	if !ok || len(antiPatterns) == 0 {
		t.Fatal("expected antiPatterns array")
	}

	ap := antiPatterns[0].(map[string]interface{})
	if _, ok := ap["tier"]; !ok {
		t.Error("expected tier field on anti-pattern")
	}
}

func TestJSONReportAggregateStrength(t *testing.T) {
	result := buildTestScanResult()
	result.AggregateStrength = map[model.Strength]int{
		model.Strong: 13,
		model.Medium: 3,
		model.Weak:   1,
	}
	var buf bytes.Buffer
	err := WriteJSONReport(&buf, result, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	agg, ok := parsed["aggregateStrength"].(map[string]interface{})
	if !ok {
		t.Fatal("expected aggregateStrength object in JSON output")
	}

	if v, ok := agg["Strong"].(float64); !ok || int(v) != 13 {
		t.Errorf("expected Strong=13, got %v", agg["Strong"])
	}
	if v, ok := agg["Medium"].(float64); !ok || int(v) != 3 {
		t.Errorf("expected Medium=3, got %v", agg["Medium"])
	}
	if v, ok := agg["Weak"].(float64); !ok || int(v) != 1 {
		t.Errorf("expected Weak=1, got %v", agg["Weak"])
	}
}

func TestJSONReportAggregateStrengthOmittedWhenEmpty(t *testing.T) {
	result := &model.ScanResult{Path: ".", Language: model.Kotlin}
	var buf bytes.Buffer
	err := WriteJSONReport(&buf, result, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if _, ok := parsed["aggregateStrength"]; ok {
		t.Error("aggregateStrength should be omitted when empty")
	}
}
