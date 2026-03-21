package analyze

import (
	"math"

	"github.com/timusus/test-confidence/internal/model"
)

// Structural coupling score weights.
// These are initial estimates — not yet calibrated against PIT/Descartes results.
// See PLAN.md Phase 8.1 and Phase 9.1 for the calibration plan.
const (
	weightInteractionRatio  = 0.30
	weightVerifyOrdering    = 0.15
	weightArgumentCaptor    = 0.10
	weightStubAndVerify     = 0.10
	weightSetupToAssertion  = 0.10
	weightInternalMockRatio = 0.15
	weightTautologicalTest  = 0.10
)

// setupAssertionRatioThreshold is the setup-to-assertion ratio above which the
// signal is considered fully active. A file with this ratio or above gets 1.0
// for the setup_to_assertion component. This value was chosen based on the
// observation that files with >3x more setup lines than assertion lines are
// typically over-specified.
const setupAssertionRatioThreshold = 3.0

// ScoreInput contains the per-file signals needed to compute the structural
// coupling score. All fields are derived from existing analysis results.
type ScoreInput struct {
	// InteractionAssertionRatio is the fraction of assertions that are verify
	// calls (0.0–1.0). Computed as interaction / (output + exception + interaction).
	// A value of 1.0 means all assertions are verify calls.
	InteractionAssertionRatio float64

	// VerifyOrderingPresent is true if any test method in the file uses
	// verifyOrder, verifySequence, or InOrder.
	VerifyOrderingPresent bool

	// ArgumentCaptorPresent is true if any test method uses slot/capture/ArgumentCaptor.
	ArgumentCaptorPresent bool

	// StubAndVerifyOverlap is true if any test method stubs and verifies the
	// same (receiver, method) pair.
	StubAndVerifyOverlap bool

	// SetupToAssertionRatio is the ratio of stub/mock setup statements to
	// assertion statements across the file. Higher values indicate more setup
	// relative to actual checking. 0 if no assertions.
	SetupToAssertionRatio float64

	// InternalMockRatio is the fraction of non-fake doubles placed at the
	// internal layer (0.0–1.0). -1 if placement is unknown for all doubles
	// (meaning we have no data to score this component).
	InternalMockRatio float64

	// TautologicalTestPresent is true if any test in the file was flagged as
	// tautological (asserting the same value that was stubbed).
	TautologicalTestPresent bool
}

// ComputeStructuralCouplingScore computes a 0–100 score indicating the degree
// of structural coupling in a test file. Higher = more structurally coupled.
//
// The score is a weighted sum of individual binary and continuous signals.
// Git signals are deliberately excluded because their accuracy is
// workflow-dependent (see DESIGN.md).
//
// Weights are initial estimates — not yet calibrated against mutation testing
// results. See PLAN.md Phase 9.1 for the calibration plan.
func ComputeStructuralCouplingScore(input ScoreInput) float64 {
	score := 0.0

	// 1. Interaction-to-assertion ratio (continuous, 0.0–1.0)
	score += input.InteractionAssertionRatio * weightInteractionRatio

	// 2. Verify ordering (binary)
	if input.VerifyOrderingPresent {
		score += weightVerifyOrdering
	}

	// 3. Argument captor (binary)
	if input.ArgumentCaptorPresent {
		score += weightArgumentCaptor
	}

	// 4. Stub-and-verify overlap (binary)
	if input.StubAndVerifyOverlap {
		score += weightStubAndVerify
	}

	// 5. Setup-to-assertion ratio (continuous, normalized to 0.0–1.0)
	// Clamp at the threshold: anything above the threshold gets 1.0.
	if input.SetupToAssertionRatio > 0 {
		normalized := input.SetupToAssertionRatio / setupAssertionRatioThreshold
		if normalized > 1.0 {
			normalized = 1.0
		}
		score += normalized * weightSetupToAssertion
	}

	// 6. Internal mock ratio (continuous, 0.0–1.0)
	// Skip if we have no placement data (-1 sentinel).
	if input.InternalMockRatio >= 0 {
		score += input.InternalMockRatio * weightInternalMockRatio
	}

	// 7. Tautological test flag (binary)
	if input.TautologicalTestPresent {
		score += weightTautologicalTest
	}

	// Scale to 0–100 and round to one decimal place.
	return math.Round(score*1000) / 10
}

// BuildScoreInput extracts a ScoreInput from the per-file analysis results.
// This bridges the existing analysis data model to the scoring function.
func BuildScoreInput(fa model.FileAnalysis) ScoreInput {
	input := ScoreInput{}

	// 1. Interaction-to-assertion ratio
	output := fa.Assertions.TargetDistribution[model.Output]
	exception := fa.Assertions.TargetDistribution[model.Exception]
	interaction := fa.Assertions.TargetDistribution[model.Interaction]
	total := output + exception + interaction
	if total > 0 {
		input.InteractionAssertionRatio = float64(interaction) / float64(total)
	}

	// 2–4. Binary structural indicators
	for _, si := range fa.Structural {
		switch si.Type {
		case model.VerifyOrdering:
			input.VerifyOrderingPresent = true
		case model.ArgumentCaptor:
			input.ArgumentCaptorPresent = true
		case model.StubAndVerify:
			input.StubAndVerifyOverlap = true
		}
	}

	// 5. Setup-to-assertion ratio
	assertions := fa.Assertions.TotalAssertions
	if assertions > 0 {
		stubCount := countStubStatements(fa)
		input.SetupToAssertionRatio = float64(stubCount) / float64(assertions)
	}

	// 6. Internal mock ratio
	var internal, classified int
	for _, d := range fa.Doubles.Doubles {
		if d.Usage == model.FakeUsage {
			continue
		}
		switch d.Placement {
		case model.Internal:
			internal++
			classified++
		case model.Boundary:
			classified++
		}
	}
	if classified > 0 {
		input.InternalMockRatio = float64(internal) / float64(classified)
	} else {
		input.InternalMockRatio = -1 // no data
	}

	// 7. Tautological test flag
	input.TautologicalTestPresent = len(fa.Tautologies) > 0

	return input
}

// countStubStatements counts the number of stub setup statements across all
// test methods in a file. This is used for the setup-to-assertion ratio.
func countStubStatements(fa model.FileAnalysis) int {
	count := 0
	for _, cls := range fa.File.Classes {
		for _, method := range cls.Methods {
			for _, stmt := range method.Statements {
				if isStubLike(stmt) {
					count++
				}
			}
		}
	}
	return count
}

// isStubLike checks if a statement looks like a stub setup call.
// Uses text-based matching on common stub patterns.
func isStubLike(node model.ASTNode) bool {
	text := node.Text()
	// MockK stubs
	if containsAny(text, "every {", "every{", "coEvery {", "coEvery{") {
		return true
	}
	// Mockito stubs
	if containsAny(text, "whenever(", ".thenReturn(", ".thenAnswer(") {
		return true
	}
	return false
}

// containsAny returns true if s contains any of the given substrings.
func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		for i := 0; i <= len(s)-len(sub); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
	}
	return false
}
