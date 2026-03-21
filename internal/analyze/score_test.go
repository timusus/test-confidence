package analyze

import (
	"math"
	"testing"

	"github.com/timusus/test-confidence/internal/model"
)

func TestComputeStructuralCouplingScore_AllZero(t *testing.T) {
	input := ScoreInput{InternalMockRatio: -1} // no placement data
	score := ComputeStructuralCouplingScore(input)
	if score != 0 {
		t.Errorf("expected 0 for empty input, got %v", score)
	}
}

func TestComputeStructuralCouplingScore_AllMaximum(t *testing.T) {
	input := ScoreInput{
		InteractionAssertionRatio: 1.0,
		VerifyOrderingPresent:     true,
		ArgumentCaptorPresent:     true,
		StubAndVerifyOverlap:      true,
		SetupToAssertionRatio:     5.0, // above threshold, should clamp to 1.0
		InternalMockRatio:         1.0,
		TautologicalTestPresent:   true,
	}
	score := ComputeStructuralCouplingScore(input)
	if score != 100 {
		t.Errorf("expected 100 for maximum input, got %v", score)
	}
}

func TestComputeStructuralCouplingScore_InteractionOnly(t *testing.T) {
	input := ScoreInput{
		InteractionAssertionRatio: 1.0,
		InternalMockRatio:         -1,
	}
	score := ComputeStructuralCouplingScore(input)
	expected := 30.0 // 1.0 * 0.30 * 100
	if score != expected {
		t.Errorf("expected %.1f, got %.1f", expected, score)
	}
}

func TestComputeStructuralCouplingScore_HalfInteraction(t *testing.T) {
	input := ScoreInput{
		InteractionAssertionRatio: 0.5,
		InternalMockRatio:         -1,
	}
	score := ComputeStructuralCouplingScore(input)
	expected := 15.0 // 0.5 * 0.30 * 100
	if score != expected {
		t.Errorf("expected %.1f, got %.1f", expected, score)
	}
}

func TestComputeStructuralCouplingScore_BinarySignals(t *testing.T) {
	input := ScoreInput{
		VerifyOrderingPresent: true,
		ArgumentCaptorPresent: true,
		InternalMockRatio:     -1,
	}
	score := ComputeStructuralCouplingScore(input)
	expected := 25.0 // (0.15 + 0.10) * 100
	if score != expected {
		t.Errorf("expected %.1f, got %.1f", expected, score)
	}
}

func TestComputeStructuralCouplingScore_SetupRatioClamped(t *testing.T) {
	input := ScoreInput{
		SetupToAssertionRatio: 6.0, // 2x the threshold
		InternalMockRatio:     -1,
	}
	score := ComputeStructuralCouplingScore(input)
	expected := 10.0 // clamped to 1.0 * 0.10 * 100
	if score != expected {
		t.Errorf("expected %.1f, got %.1f", expected, score)
	}
}

func TestComputeStructuralCouplingScore_SetupRatioPartial(t *testing.T) {
	input := ScoreInput{
		SetupToAssertionRatio: 1.5, // half of threshold (3.0)
		InternalMockRatio:     -1,
	}
	score := ComputeStructuralCouplingScore(input)
	expected := 5.0 // (1.5/3.0) * 0.10 * 100
	if score != expected {
		t.Errorf("expected %.1f, got %.1f", expected, score)
	}
}

func TestComputeStructuralCouplingScore_InternalMockRatioIgnoredWhenUnknown(t *testing.T) {
	input := ScoreInput{
		InternalMockRatio: -1, // no data
	}
	score := ComputeStructuralCouplingScore(input)
	if score != 0 {
		t.Errorf("expected 0 when no placement data, got %v", score)
	}
}

func TestComputeStructuralCouplingScore_InternalMockRatioPartial(t *testing.T) {
	input := ScoreInput{
		InternalMockRatio: 0.5,
	}
	score := ComputeStructuralCouplingScore(input)
	expected := 7.5 // 0.5 * 0.15 * 100
	if score != expected {
		t.Errorf("expected %.1f, got %.1f", expected, score)
	}
}

func TestComputeStructuralCouplingScore_MixedSignals(t *testing.T) {
	// Simulate a file with moderate structural coupling:
	// 40% verify assertions, uses argument captor, 50% internal mocks
	input := ScoreInput{
		InteractionAssertionRatio: 0.4,
		ArgumentCaptorPresent:     true,
		InternalMockRatio:         0.5,
	}
	score := ComputeStructuralCouplingScore(input)
	// 0.4*0.30 + 0.10 + 0.5*0.15 = 0.12 + 0.10 + 0.075 = 0.295
	expected := math.Round(0.295*1000) / 10 // 29.5
	if score != expected {
		t.Errorf("expected %.1f, got %.1f", expected, score)
	}
}

func TestBuildScoreInput_PurelyBehavioral(t *testing.T) {
	fa := model.FileAnalysis{
		Assertions: model.AssertionAnalysis{
			TotalAssertions:    10,
			TargetDistribution: map[model.Target]int{model.Output: 10},
		},
		Doubles: model.DoublesAnalysis{
			Doubles: []model.TestDouble{
				{TypeName: "Repo", Usage: model.SetupOnly, Placement: model.Boundary},
			},
		},
	}
	input := BuildScoreInput(fa)

	if input.InteractionAssertionRatio != 0 {
		t.Errorf("expected 0 interaction ratio, got %v", input.InteractionAssertionRatio)
	}
	if input.VerifyOrderingPresent {
		t.Error("expected no verify ordering")
	}
	if input.ArgumentCaptorPresent {
		t.Error("expected no argument captor")
	}
	if input.StubAndVerifyOverlap {
		t.Error("expected no stub-and-verify")
	}
	if input.TautologicalTestPresent {
		t.Error("expected no tautological tests")
	}
	if input.InternalMockRatio != 0 {
		t.Errorf("expected 0 internal mock ratio, got %v", input.InternalMockRatio)
	}
}

func TestBuildScoreInput_StructurallyCoupled(t *testing.T) {
	fa := model.FileAnalysis{
		Assertions: model.AssertionAnalysis{
			TotalAssertions:    5,
			TargetDistribution: map[model.Target]int{model.Output: 3, model.Interaction: 7},
		},
		Structural: []model.StructuralIndicator{
			{Type: model.VerifyOrdering},
			{Type: model.ArgumentCaptor},
			{Type: model.StubAndVerify},
		},
		Tautologies: []model.TautologyFlag{
			{File: "test.kt", Method: "test1"},
		},
		Doubles: model.DoublesAnalysis{
			Doubles: []model.TestDouble{
				{TypeName: "UseCase", Usage: model.Verification, Placement: model.Internal},
				{TypeName: "Api", Usage: model.SetupOnly, Placement: model.Boundary},
			},
		},
	}
	input := BuildScoreInput(fa)

	expectedRatio := 7.0 / 10.0
	if math.Abs(input.InteractionAssertionRatio-expectedRatio) > 0.001 {
		t.Errorf("expected interaction ratio %.3f, got %.3f", expectedRatio, input.InteractionAssertionRatio)
	}
	if !input.VerifyOrderingPresent {
		t.Error("expected verify ordering")
	}
	if !input.ArgumentCaptorPresent {
		t.Error("expected argument captor")
	}
	if !input.StubAndVerifyOverlap {
		t.Error("expected stub-and-verify")
	}
	if !input.TautologicalTestPresent {
		t.Error("expected tautological test")
	}
	if input.InternalMockRatio != 0.5 {
		t.Errorf("expected 0.5 internal mock ratio, got %v", input.InternalMockRatio)
	}
}

func TestBuildScoreInput_NoDoublesPlacement(t *testing.T) {
	fa := model.FileAnalysis{
		Assertions: model.AssertionAnalysis{
			TotalAssertions:    5,
			TargetDistribution: map[model.Target]int{model.Output: 5},
		},
		Doubles: model.DoublesAnalysis{
			Doubles: []model.TestDouble{
				{TypeName: "Thing", Usage: model.SetupOnly, Placement: model.UnknownPlacement},
			},
		},
	}
	input := BuildScoreInput(fa)

	if input.InternalMockRatio != -1 {
		t.Errorf("expected -1 (no placement data), got %v", input.InternalMockRatio)
	}
}

func TestBuildScoreInput_FakesExcluded(t *testing.T) {
	fa := model.FileAnalysis{
		Assertions: model.AssertionAnalysis{
			TotalAssertions:    5,
			TargetDistribution: map[model.Target]int{model.Output: 5},
		},
		Doubles: model.DoublesAnalysis{
			Doubles: []model.TestDouble{
				{TypeName: "FakeRepo", Usage: model.FakeUsage, Placement: model.Boundary},
				{TypeName: "Api", Usage: model.Verification, Placement: model.Internal},
			},
		},
	}
	input := BuildScoreInput(fa)

	// Fake should be excluded; only the Internal double counts
	if input.InternalMockRatio != 1.0 {
		t.Errorf("expected 1.0 (fake excluded), got %v", input.InternalMockRatio)
	}
}

func TestBuildScoreInput_NoAssertions(t *testing.T) {
	fa := model.FileAnalysis{
		Assertions: model.AssertionAnalysis{
			TotalAssertions:    0,
			TargetDistribution: map[model.Target]int{},
		},
	}
	input := BuildScoreInput(fa)

	if input.InteractionAssertionRatio != 0 {
		t.Errorf("expected 0 interaction ratio with no assertions, got %v", input.InteractionAssertionRatio)
	}
	if input.SetupToAssertionRatio != 0 {
		t.Errorf("expected 0 setup ratio with no assertions, got %v", input.SetupToAssertionRatio)
	}
}
