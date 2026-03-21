package report

import (
	"bytes"
	"strings"
	"testing"

	"github.com/timusus/test-confidence/internal/model"
)

func buildTestScanResult() *model.ScanResult {
	return &model.ScanResult{
		Path:             "/project/app",
		Platform:         model.Android,
		TotalTestFiles:   14,
		TotalTestMethods: 89,
		UnparseableFiles: 0,
		FileResults: []model.FileAnalysis{
			{
				File: model.ParsedTestFile{
					Path:     "src/test/kotlin/com/example/UserServiceTest.kt",
					Language: model.Kotlin,
				},
				AntiPatterns: []model.AntiPattern{
					{Type: model.ThreadSleep, File: "UserServiceTest.kt", Line: 42, Method: "testUserLogin", Severity: model.Tier1},
					{Type: model.ThreadSleep, File: "UserServiceTest.kt", Line: 78, Method: "testUserLogout", Severity: model.Tier1},
					{Type: model.IgnoredTest, File: "UserServiceTest.kt", Line: 100, Method: "testUserDelete", Severity: model.Tier1},
				},
				Assertions: model.AssertionAnalysis{
					TotalAssertions:      12,
					StrengthDistribution: map[model.Strength]int{model.Strong: 8, model.Medium: 3, model.Weak: 1},
					TargetDistribution:   map[model.Target]int{model.Output: 8, model.Interaction: 4},
					ZeroAssertionMethods: []model.MethodRef{
						{File: "UserServiceTest.kt", Class: "UserServiceTest", Method: "testEmpty", Line: 55},
					},
					OutputInteractionRatio: 0.64,
				},
				Structural: []model.StructuralIndicator{
					{Type: model.VerifyOnlyTest, File: "UserServiceTest.kt", Line: 30, Method: "testVerifyOnly"},
					{Type: model.VerifyOrdering, File: "UserServiceTest.kt", Line: 60, Method: "testOrdered"},
					{Type: model.ArgumentCaptor, File: "UserServiceTest.kt", Line: 70, Method: "testCaptor"},
					{Type: model.StubAndVerify, File: "UserServiceTest.kt", Line: 80, Method: "testStubVerify"},
				},
				Tautologies: []model.TautologyFlag{
					{File: "UserServiceTest.kt", Line: 90, Method: "testTautology", StubIdentifier: "stubValue", AssertIdentifier: "stubValue"},
				},
				Doubles: model.DoublesAnalysis{
					Doubles: []model.TestDouble{
						{TypeName: "UserRepository", VariableName: "repo", Framework: model.FrameworkMockK, Usage: model.SetupOnly, Placement: model.Boundary},
						{TypeName: "ApiClient", VariableName: "api", Framework: model.FrameworkMockK, Usage: model.Verification, Placement: model.Internal},
						{TypeName: "Logger", VariableName: "logger", Framework: model.FrameworkMockK, Usage: model.SetupOnly, Placement: model.UnknownPlacement},
					},
					MockDensity:    0.75,
					SetupOnlyRatio: 0.67,
					FakeCount:      0,
					MostMockedTypes: []model.TypeFrequency{
						{TypeName: "UserRepository", Count: 34},
						{TypeName: "ApiClient", Count: 28},
					},
				},
				StructuralCouplingScore: 62.5,
			},
			{
				File: model.ParsedTestFile{
					Path:     "src/test/kotlin/com/example/OrderServiceTest.kt",
					Language: model.Kotlin,
				},
				AntiPatterns: []model.AntiPattern{
					{Type: model.ConditionalLogic, File: "OrderServiceTest.kt", Line: 15, Method: "testConditional", Severity: model.Tier1},
				},
				Assertions: model.AssertionAnalysis{
					TotalAssertions:      5,
					StrengthDistribution: map[model.Strength]int{model.Strong: 5},
					TargetDistribution:   map[model.Target]int{model.Output: 5},
					ZeroAssertionMethods: nil,
					OutputInteractionRatio: 1.0,
				},
				Structural: []model.StructuralIndicator{},
				Tautologies: []model.TautologyFlag{},
				Doubles: model.DoublesAnalysis{
					Doubles:         nil,
					MockDensity:     0,
					SetupOnlyRatio:  0,
					FakeCount:       0,
					MostMockedTypes: nil,
				},
			},
		},
	}
}

func TestTerminalReport(t *testing.T) {
	result := buildTestScanResult()
	var buf bytes.Buffer
	err := WriteTerminalReport(&buf, result, false, nil, nil, nil, nil, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	output := buf.String()
	if !strings.Contains(output, "Findings") {
		t.Error("expected Findings section")
	}
	if !strings.Contains(output, "sleep/delay") {
		t.Error("expected Thread.sleep in output")
	}
	if !strings.Contains(output, "Structural Coupling") {
		t.Error("expected Structural Coupling section")
	}
}

func TestTerminalReportVerbose(t *testing.T) {
	result := buildTestScanResult()
	var buf bytes.Buffer
	err := WriteTerminalReport(&buf, result, true, nil, nil, nil, nil, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	output := buf.String()
	if !strings.Contains(output, ".kt") {
		t.Error("verbose should include file names")
	}
}

func TestTerminalReportEmpty(t *testing.T) {
	result := &model.ScanResult{Path: ".", Platform: model.Android}
	var buf bytes.Buffer
	err := WriteTerminalReport(&buf, result, false, nil, nil, nil, nil, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if buf.Len() == 0 {
		t.Error("expected some output even for empty results")
	}
}

func TestTerminalReportUnparseableWarning(t *testing.T) {
	result := &model.ScanResult{
		Path:             ".",
		Platform:         model.Android,
		TotalTestFiles:   5,
		UnparseableFiles: 2,
	}
	var buf bytes.Buffer
	err := WriteTerminalReport(&buf, result, false, nil, nil, nil, nil, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	output := buf.String()
	if !strings.Contains(output, "unparseable") && !strings.Contains(output, "could not be parsed") {
		t.Error("expected warning about unparseable files")
	}
}

func TestTerminalReportTautologySection(t *testing.T) {
	result := buildTestScanResult()
	var buf bytes.Buffer
	err := WriteTerminalReport(&buf, result, false, nil, nil, nil, nil, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	output := buf.String()
	if !strings.Contains(output, "Pass-Through Tests") {
		t.Error("expected Pass-Through Tests section")
	}
}

func TestTerminalReportAssertionStrength(t *testing.T) {
	result := buildTestScanResult()
	result.AggregateStrength = map[model.Strength]int{
		model.Strong: 13,
		model.Medium: 3,
		model.Weak:   1,
	}
	var buf bytes.Buffer
	err := WriteTerminalReport(&buf, result, false, nil, nil, nil, nil, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	output := buf.String()
	if !strings.Contains(output, "Assertion Strength") {
		t.Error("expected Assertion Strength section")
	}
	if !strings.Contains(output, "Strong (equality, containment)") {
		t.Error("expected Strong label")
	}
	if !strings.Contains(output, "Medium (boolean, existence)") {
		t.Error("expected Medium label")
	}
	if !strings.Contains(output, "Weak (null checks)") {
		t.Error("expected Weak label")
	}
	if !strings.Contains(output, "13") {
		t.Error("expected strong count 13 in output")
	}
}

func TestTerminalReportAssertionStrengthOmittedWhenEmpty(t *testing.T) {
	result := &model.ScanResult{Path: ".", Platform: model.Android}
	var buf bytes.Buffer
	err := WriteTerminalReport(&buf, result, false, nil, nil, nil, nil, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	output := buf.String()
	if strings.Contains(output, "Assertion Strength") {
		t.Error("should not show Assertion Strength section when no data")
	}
}

func TestTerminalReportWorstFilesWithScore(t *testing.T) {
	result := buildTestScanResult()
	var buf bytes.Buffer
	err := WriteTerminalReport(&buf, result, false, nil, nil, nil, nil, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	output := buf.String()
	if !strings.Contains(output, "Worst Files") {
		t.Error("expected Worst Files section")
	}
	if !strings.Contains(output, "score:") {
		t.Error("expected score in Worst Files output")
	}
	if !strings.Contains(output, "Score distribution") {
		t.Error("expected Score distribution summary")
	}
}
