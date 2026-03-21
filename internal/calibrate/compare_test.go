package calibrate

import (
	"testing"

	"github.com/timusus/test-confidence/internal/model"
)

func boolPtr(v bool) *bool {
	return &v
}

func strPtr(v string) *string {
	return &v
}

func TestClassifyAsBehavioral(t *testing.T) {
	tests := []struct {
		name     string
		fa       model.FileAnalysis
		expected bool
	}{
		{
			name: "output assertions only = behavioral",
			fa: model.FileAnalysis{
				Assertions: model.AssertionAnalysis{
					TotalAssertions:    3,
					TargetDistribution: map[model.Target]int{model.Output: 3},
				},
			},
			expected: true,
		},
		{
			name: "structural indicators = not behavioral",
			fa: model.FileAnalysis{
				Assertions: model.AssertionAnalysis{
					TotalAssertions:    3,
					TargetDistribution: map[model.Target]int{model.Output: 3},
				},
				Structural: []model.StructuralIndicator{
					{Type: model.VerifyOnlyTest},
				},
			},
			expected: false,
		},
		{
			name: "interaction only = not behavioral",
			fa: model.FileAnalysis{
				Assertions: model.AssertionAnalysis{
					TotalAssertions:    0,
					TargetDistribution: map[model.Target]int{model.Interaction: 2},
				},
			},
			expected: false,
		},
		{
			name: "more output than interaction = behavioral",
			fa: model.FileAnalysis{
				Assertions: model.AssertionAnalysis{
					TotalAssertions:    5,
					TargetDistribution: map[model.Target]int{model.Output: 4, model.Interaction: 1},
				},
			},
			expected: true,
		},
		{
			name: "no assertions = not behavioral",
			fa: model.FileAnalysis{
				Assertions: model.AssertionAnalysis{
					TotalAssertions:    0,
					TargetDistribution: map[model.Target]int{},
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyAsBehavioral(tt.fa)
			if got != tt.expected {
				t.Errorf("classifyAsBehavioral() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestClassifyCouplingLevel(t *testing.T) {
	tests := []struct {
		name     string
		fa       model.FileAnalysis
		expected CouplingLevel
	}{
		{
			name: "no structural indicators = low",
			fa: model.FileAnalysis{
				File: model.ParsedTestFile{
					Classes: []model.TestClass{
						{Methods: []model.TestMethod{{}, {}, {}}},
					},
				},
			},
			expected: CouplingLow,
		},
		{
			name: "verify ordering = high",
			fa: model.FileAnalysis{
				File: model.ParsedTestFile{
					Classes: []model.TestClass{
						{Methods: []model.TestMethod{{}, {}}},
					},
				},
				Structural: []model.StructuralIndicator{
					{Type: model.VerifyOrdering},
				},
			},
			expected: CouplingHigh,
		},
		{
			name: "multiple verify-only = high",
			fa: model.FileAnalysis{
				File: model.ParsedTestFile{
					Classes: []model.TestClass{
						{Methods: []model.TestMethod{{}, {}, {}}},
					},
				},
				Structural: []model.StructuralIndicator{
					{Type: model.VerifyOnlyTest},
					{Type: model.VerifyOnlyTest},
				},
			},
			expected: CouplingHigh,
		},
		{
			name: "single stub-and-verify = medium",
			fa: model.FileAnalysis{
				File: model.ParsedTestFile{
					Classes: []model.TestClass{
						{Methods: []model.TestMethod{{}, {}, {}}},
					},
				},
				Structural: []model.StructuralIndicator{
					{Type: model.StubAndVerify},
				},
			},
			expected: CouplingMedium,
		},
		{
			name: "no methods = low",
			fa: model.FileAnalysis{
				File: model.ParsedTestFile{
					Classes: []model.TestClass{},
				},
			},
			expected: CouplingLow,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyCouplingLevel(tt.fa)
			if got != tt.expected {
				t.Errorf("classifyCouplingLevel() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestClassifyAsFragile(t *testing.T) {
	tests := []struct {
		name     string
		fa       model.FileAnalysis
		expected bool
	}{
		{
			name: "structural indicators = fragile",
			fa: model.FileAnalysis{
				Structural: []model.StructuralIndicator{
					{Type: model.StubAndVerify},
				},
			},
			expected: true,
		},
		{
			name: "verification doubles = fragile",
			fa: model.FileAnalysis{
				Doubles: model.DoublesAnalysis{
					Doubles: []model.TestDouble{
						{Usage: model.Verification},
					},
				},
			},
			expected: true,
		},
		{
			name: "setup-only doubles = not fragile",
			fa: model.FileAnalysis{
				Doubles: model.DoublesAnalysis{
					Doubles: []model.TestDouble{
						{Usage: model.SetupOnly},
					},
				},
			},
			expected: false,
		},
		{
			name: "no indicators = not fragile",
			fa:       model.FileAnalysis{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyAsFragile(tt.fa)
			if got != tt.expected {
				t.Errorf("classifyAsFragile() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestClassifyOverallPlacement(t *testing.T) {
	tests := []struct {
		name     string
		fa       model.FileAnalysis
		expected string
	}{
		{
			name:     "no doubles = empty",
			fa:       model.FileAnalysis{},
			expected: "",
		},
		{
			name: "all boundary = boundary",
			fa: model.FileAnalysis{
				Doubles: model.DoublesAnalysis{
					Doubles: []model.TestDouble{
						{Placement: model.Boundary},
						{Placement: model.Boundary},
					},
				},
			},
			expected: "boundary",
		},
		{
			name: "all internal = internal",
			fa: model.FileAnalysis{
				Doubles: model.DoublesAnalysis{
					Doubles: []model.TestDouble{
						{Placement: model.Internal},
					},
				},
			},
			expected: "internal",
		},
		{
			name: "mixed = mixed",
			fa: model.FileAnalysis{
				Doubles: model.DoublesAnalysis{
					Doubles: []model.TestDouble{
						{Placement: model.Boundary},
						{Placement: model.Internal},
					},
				},
			},
			expected: "mixed",
		},
		{
			name: "unknown placement only = empty",
			fa: model.FileAnalysis{
				Doubles: model.DoublesAnalysis{
					Doubles: []model.TestDouble{
						{Placement: model.UnknownPlacement},
					},
				},
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyOverallPlacement(tt.fa)
			if got != tt.expected {
				t.Errorf("classifyOverallPlacement() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestCompare(t *testing.T) {
	result := &model.ScanResult{
		FileResults: []model.FileAnalysis{
			{
				File: model.ParsedTestFile{
					Path: "/project/src/test/FooTest.kt",
					Classes: []model.TestClass{
						{Methods: []model.TestMethod{{}, {}, {}}},
					},
				},
				Assertions: model.AssertionAnalysis{
					TotalAssertions:    5,
					TargetDistribution: map[model.Target]int{model.Output: 5},
				},
				AntiPatterns: []model.AntiPattern{},
				Structural:   []model.StructuralIndicator{},
				Tautologies:  []model.TautologyFlag{},
			},
			{
				File: model.ParsedTestFile{
					Path: "/project/src/test/BarTest.kt",
					Classes: []model.TestClass{
						{Methods: []model.TestMethod{{}, {}}},
					},
				},
				Assertions: model.AssertionAnalysis{
					TotalAssertions:    0,
					TargetDistribution: map[model.Target]int{model.Interaction: 3},
				},
				AntiPatterns: []model.AntiPattern{
					{Type: model.ThreadSleep},
				},
				Structural: []model.StructuralIndicator{
					{Type: model.VerifyOnlyTest},
					{Type: model.VerifyOnlyTest},
				},
				Tautologies: []model.TautologyFlag{},
			},
		},
	}

	gt := &GroundTruth{
		Annotations: []Annotation{
			{
				File:               "src/test/FooTest.kt",
				StructuralCoupling: CouplingLow,
				Behavioral:         boolPtr(true),
				Fragile:            boolPtr(false),
				HasAntiPatterns:    boolPtr(false),
				HasTautologies:     boolPtr(false),
			},
			{
				File:               "src/test/BarTest.kt",
				StructuralCoupling: CouplingHigh,
				Behavioral:         boolPtr(false),
				Fragile:            boolPtr(true),
				HasAntiPatterns:    boolPtr(true),
				HasTautologies:     boolPtr(false),
			},
			{
				File:               "src/test/MissingTest.kt",
				StructuralCoupling: CouplingLow,
				Behavioral:         boolPtr(true),
			},
		},
	}

	cal := Compare(result, gt, "/project")

	if cal.TotalAnnotated != 3 {
		t.Errorf("TotalAnnotated = %d, want 3", cal.TotalAnnotated)
	}
	if cal.Matched != 2 {
		t.Errorf("Matched = %d, want 2", cal.Matched)
	}
	if len(cal.Unmatched) != 1 {
		t.Errorf("Unmatched = %d, want 1", len(cal.Unmatched))
	}

	// Check that all signals agreed (our test data is constructed to agree)
	for _, s := range cal.Signals {
		if s.Agreed != s.Total {
			t.Errorf("Signal %q: agreed %d/%d, expected full agreement", s.Signal, s.Agreed, s.Total)
		}
	}

	// Check overall accuracy is 100% for this perfect-match scenario
	acc := cal.OverallAccuracy()
	if acc != 1.0 {
		t.Errorf("OverallAccuracy = %v, want 1.0", acc)
	}
}

func TestCompareWithMismatches(t *testing.T) {
	result := &model.ScanResult{
		FileResults: []model.FileAnalysis{
			{
				File: model.ParsedTestFile{
					Path: "/project/src/test/FooTest.kt",
					Classes: []model.TestClass{
						{Methods: []model.TestMethod{{}, {}}},
					},
				},
				Assertions: model.AssertionAnalysis{
					TotalAssertions:    0,
					TargetDistribution: map[model.Target]int{model.Interaction: 2},
				},
				Structural: []model.StructuralIndicator{
					{Type: model.VerifyOnlyTest},
				},
			},
		},
	}

	gt := &GroundTruth{
		Annotations: []Annotation{
			{
				File:               "src/test/FooTest.kt",
				StructuralCoupling: CouplingLow,
				Behavioral:         boolPtr(true),
				Fragile:            boolPtr(false),
			},
		},
	}

	cal := Compare(result, gt, "/project")

	// The tool should disagree on all signals here because the file is structural
	// but the annotation says low coupling, behavioral, not fragile
	totalMismatches := 0
	for _, s := range cal.Signals {
		totalMismatches += len(s.Misses)
	}
	if totalMismatches == 0 {
		t.Error("expected mismatches but got none")
	}

	// Overall accuracy should be less than 1.0
	if cal.OverallAccuracy() >= 1.0 {
		t.Errorf("expected accuracy < 1.0, got %v", cal.OverallAccuracy())
	}
}

func TestCalibrationResultTierAccuracy(t *testing.T) {
	cal := &CalibrationResult{
		Signals: []SignalResult{
			{Signal: "s1", Tier: model.Tier1, Agreed: 8, Total: 10},
			{Signal: "s2", Tier: model.Tier1, Agreed: 9, Total: 10},
			{Signal: "s3", Tier: model.Tier2, Agreed: 7, Total: 10},
			{Signal: "s4", Tier: model.Tier3, Agreed: 5, Total: 10},
		},
	}

	// Tier 1: 17/20
	agreed, total := cal.TierAccuracy(model.Tier1)
	if agreed != 17 || total != 20 {
		t.Errorf("Tier1: got %d/%d, want 17/20", agreed, total)
	}

	// Tier 2: 7/10
	agreed, total = cal.TierAccuracy(model.Tier2)
	if agreed != 7 || total != 10 {
		t.Errorf("Tier2: got %d/%d, want 7/10", agreed, total)
	}

	// Tier 3: 5/10
	agreed, total = cal.TierAccuracy(model.Tier3)
	if agreed != 5 || total != 10 {
		t.Errorf("Tier3: got %d/%d, want 5/10", agreed, total)
	}

	// Overall: 29/40
	acc := cal.OverallAccuracy()
	expected := 29.0 / 40.0
	if acc != expected {
		t.Errorf("OverallAccuracy = %v, want %v", acc, expected)
	}
}

func TestSignalResultAccuracy(t *testing.T) {
	s := SignalResult{Agreed: 0, Total: 0}
	if s.Accuracy() != 0 {
		t.Errorf("empty signal accuracy = %v, want 0", s.Accuracy())
	}

	s = SignalResult{Agreed: 7, Total: 10}
	if s.Accuracy() != 0.7 {
		t.Errorf("accuracy = %v, want 0.7", s.Accuracy())
	}
}
