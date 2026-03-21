package calibrate

import (
	"github.com/timusus/test-confidence/internal/model"
)

// SignalResult tracks agreement for a single signal comparison.
type SignalResult struct {
	Signal    string
	Tier      model.Tier
	Agreed    int
	Total     int
	Misses    []Mismatch // individual disagreements for debugging
}

// Accuracy returns the agreement rate as a float64 (0.0 to 1.0).
func (s SignalResult) Accuracy() float64 {
	if s.Total == 0 {
		return 0
	}
	return float64(s.Agreed) / float64(s.Total)
}

// Mismatch describes a single disagreement between tool and human.
type Mismatch struct {
	File     string
	Expected string
	Got      string
}

// CalibrationResult holds all signal comparison results.
type CalibrationResult struct {
	TotalAnnotated int
	Matched        int // annotations that matched a scanned file
	Unmatched      []string // annotation files not found in scan results
	Signals        []SignalResult
}

// OverallAccuracy computes a weighted accuracy across all signals,
// weighted by the number of comparisons per signal.
func (c CalibrationResult) OverallAccuracy() float64 {
	totalAgreed := 0
	totalComparisons := 0
	for _, s := range c.Signals {
		totalAgreed += s.Agreed
		totalComparisons += s.Total
	}
	if totalComparisons == 0 {
		return 0
	}
	return float64(totalAgreed) / float64(totalComparisons)
}

// TierAccuracy returns (agreed, total) counts for a specific tier.
func (c CalibrationResult) TierAccuracy(tier model.Tier) (int, int) {
	agreed := 0
	total := 0
	for _, s := range c.Signals {
		if s.Tier == tier {
			agreed += s.Agreed
			total += s.Total
		}
	}
	return agreed, total
}

// Compare runs the calibration comparison between scan results and ground truth.
func Compare(result *model.ScanResult, gt *GroundTruth, scanPath string) *CalibrationResult {
	cal := &CalibrationResult{
		TotalAnnotated: len(gt.Annotations),
	}

	// Build a lookup from annotation file to its matched FileAnalysis
	type matchedPair struct {
		annotation Annotation
		analysis   model.FileAnalysis
	}
	var pairs []matchedPair

	for _, ann := range gt.Annotations {
		resolved := ResolveAnnotationPath(ann.File, scanPath)
		found := false
		for _, fa := range result.FileResults {
			if MatchesFile(fa.File.Path, resolved) {
				pairs = append(pairs, matchedPair{annotation: ann, analysis: fa})
				found = true
				break
			}
		}
		if !found {
			cal.Unmatched = append(cal.Unmatched, ann.File)
		}
	}
	cal.Matched = len(pairs)

	// Compare each signal
	behavioral := SignalResult{Signal: "Behavioral classification", Tier: model.Tier2}
	couplingHigh := SignalResult{Signal: "Structural coupling (high)", Tier: model.Tier2}
	couplingLow := SignalResult{Signal: "Structural coupling (low)", Tier: model.Tier2}
	antiPattern := SignalResult{Signal: "Anti-pattern detection", Tier: model.Tier1}
	tautological := SignalResult{Signal: "Tautological detection", Tier: model.Tier1}
	mockPlacement := SignalResult{Signal: "Mock placement", Tier: model.Tier3}
	fragile := SignalResult{Signal: "Fragility assessment", Tier: model.Tier2}

	for _, p := range pairs {
		ann := p.annotation
		fa := p.analysis

		// 1. Behavioral classification (Tier 2)
		if ann.Behavioral != nil {
			behavioral.Total++
			toolBehavioral := classifyAsBehavioral(fa)
			if toolBehavioral == *ann.Behavioral {
				behavioral.Agreed++
			} else {
				behavioral.Misses = append(behavioral.Misses, Mismatch{
					File:     ann.File,
					Expected: boolLabel(*ann.Behavioral, "behavioral", "structural"),
					Got:      boolLabel(toolBehavioral, "behavioral", "structural"),
				})
			}
		}

		// 2. Structural coupling level (Tier 2)
		if ann.StructuralCoupling != "" {
			toolCoupling := classifyCouplingLevel(fa)
			if ann.StructuralCoupling == CouplingHigh {
				couplingHigh.Total++
				if toolCoupling == CouplingHigh {
					couplingHigh.Agreed++
				} else {
					couplingHigh.Misses = append(couplingHigh.Misses, Mismatch{
						File:     ann.File,
						Expected: string(ann.StructuralCoupling),
						Got:      string(toolCoupling),
					})
				}
			}
			if ann.StructuralCoupling == CouplingLow {
				couplingLow.Total++
				if toolCoupling == CouplingLow {
					couplingLow.Agreed++
				} else {
					couplingLow.Misses = append(couplingLow.Misses, Mismatch{
						File:     ann.File,
						Expected: string(ann.StructuralCoupling),
						Got:      string(toolCoupling),
					})
				}
			}
		}

		// 3. Anti-pattern detection (Tier 1)
		if ann.HasAntiPatterns != nil {
			antiPattern.Total++
			toolHasAntiPatterns := len(fa.AntiPatterns) > 0
			if toolHasAntiPatterns == *ann.HasAntiPatterns {
				antiPattern.Agreed++
			} else {
				antiPattern.Misses = append(antiPattern.Misses, Mismatch{
					File:     ann.File,
					Expected: boolLabel(*ann.HasAntiPatterns, "has anti-patterns", "no anti-patterns"),
					Got:      boolLabel(toolHasAntiPatterns, "has anti-patterns", "no anti-patterns"),
				})
			}
		}

		// 4. Tautological detection (Tier 1)
		if ann.HasTautologies != nil {
			tautological.Total++
			toolHasTautologies := len(fa.Tautologies) > 0
			if toolHasTautologies == *ann.HasTautologies {
				tautological.Agreed++
			} else {
				tautological.Misses = append(tautological.Misses, Mismatch{
					File:     ann.File,
					Expected: boolLabel(*ann.HasTautologies, "has tautologies", "no tautologies"),
					Got:      boolLabel(toolHasTautologies, "has tautologies", "no tautologies"),
				})
			}
		}

		// 5. Mock placement (Tier 3)
		if ann.MockPlacement != nil {
			toolPlacement := classifyOverallPlacement(fa)
			if toolPlacement != "" {
				mockPlacement.Total++
				if toolPlacement == *ann.MockPlacement {
					mockPlacement.Agreed++
				} else {
					mockPlacement.Misses = append(mockPlacement.Misses, Mismatch{
						File:     ann.File,
						Expected: *ann.MockPlacement,
						Got:      toolPlacement,
					})
				}
			}
		}

		// 6. Fragility assessment (Tier 2)
		if ann.Fragile != nil {
			fragile.Total++
			toolFragile := classifyAsFragile(fa)
			if toolFragile == *ann.Fragile {
				fragile.Agreed++
			} else {
				fragile.Misses = append(fragile.Misses, Mismatch{
					File:     ann.File,
					Expected: boolLabel(*ann.Fragile, "fragile", "not fragile"),
					Got:      boolLabel(toolFragile, "fragile", "not fragile"),
				})
			}
		}
	}

	// Only include signals that had comparisons
	for _, s := range []SignalResult{behavioral, couplingHigh, couplingLow, antiPattern, tautological, mockPlacement, fragile} {
		if s.Total > 0 {
			cal.Signals = append(cal.Signals, s)
		}
	}

	return cal
}

// classifyAsBehavioral determines if the tool considers a file primarily behavioral.
// A file is behavioral if it has output assertions and no structural indicators.
func classifyAsBehavioral(fa model.FileAnalysis) bool {
	hasStructural := len(fa.Structural) > 0
	hasOutputAssertions := fa.Assertions.TargetDistribution[model.Output] > 0
	hasInteractionAssertions := fa.Assertions.TargetDistribution[model.Interaction] > 0

	// If there are structural indicators or interaction-heavy assertions, it's not behavioral
	if hasStructural {
		return false
	}
	// If it has output assertions and no/few interaction assertions, it's behavioral
	if hasOutputAssertions && !hasInteractionAssertions {
		return true
	}
	// If it has output assertions that outnumber interaction assertions, lean behavioral
	outputCount := fa.Assertions.TargetDistribution[model.Output]
	interactionCount := fa.Assertions.TargetDistribution[model.Interaction]
	if outputCount > interactionCount {
		return true
	}
	return false
}

// classifyCouplingLevel maps tool signals to a coupling level.
func classifyCouplingLevel(fa model.FileAnalysis) CouplingLevel {
	structuralCount := len(fa.Structural)
	methodCount := 0
	for _, cls := range fa.File.Classes {
		methodCount += len(cls.Methods)
	}

	if methodCount == 0 {
		return CouplingLow
	}

	// Count specific high-signal indicators
	verifyOnly := 0
	captors := 0
	stubAndVerify := 0
	for _, s := range fa.Structural {
		switch s.Type {
		case model.VerifyOnlyTest:
			verifyOnly++
		case model.ArgumentCaptor:
			captors++
		case model.StubAndVerify:
			stubAndVerify++
		case model.VerifyOrdering:
			// VerifyOrdering is a strong signal of high coupling
			return CouplingHigh
		}
	}

	// Density: structural indicators per test method
	density := float64(structuralCount) / float64(methodCount)

	if density >= 0.5 || verifyOnly >= 2 || captors >= 2 {
		return CouplingHigh
	}
	if density > 0 || stubAndVerify > 0 {
		return CouplingMedium
	}
	return CouplingLow
}

// classifyAsFragile determines if the tool signals suggest a fragile test file.
// Fragile = would break on refactoring without behavior change.
func classifyAsFragile(fa model.FileAnalysis) bool {
	// Structural indicators are the primary fragility signal
	if len(fa.Structural) > 0 {
		return true
	}
	// High mock density with verification usage suggests fragility
	verificationDoubles := 0
	for _, d := range fa.Doubles.Doubles {
		if d.Usage == model.Verification || d.Usage == model.Both {
			verificationDoubles++
		}
	}
	if verificationDoubles > 0 {
		return true
	}
	return false
}

// classifyOverallPlacement determines the dominant mock placement for a file.
// Returns "boundary", "internal", "mixed", or "" if no mocks.
func classifyOverallPlacement(fa model.FileAnalysis) string {
	boundary := 0
	internal := 0
	for _, d := range fa.Doubles.Doubles {
		switch d.Placement {
		case model.Boundary:
			boundary++
		case model.Internal:
			internal++
		}
	}
	total := boundary + internal
	if total == 0 {
		return ""
	}
	if boundary > 0 && internal == 0 {
		return "boundary"
	}
	if internal > 0 && boundary == 0 {
		return "internal"
	}
	return "mixed"
}

func boolLabel(v bool, trueLabel, falseLabel string) string {
	if v {
		return trueLabel
	}
	return falseLabel
}
