package calibrate

import (
	"fmt"
	"io"
	"strings"

	"github.com/timusus/test-confidence/internal/model"
)

// WriteReport writes the calibration results to the given writer.
func WriteReport(w io.Writer, result *CalibrationResult) error {
	fmt.Fprintf(w, "\nCalibration Results (%d annotated files, %d matched)\n", result.TotalAnnotated, result.Matched)
	fmt.Fprintln(w, strings.Repeat("─", 60))

	if len(result.Unmatched) > 0 {
		fmt.Fprintf(w, "\n  Unmatched annotations (%d):\n", len(result.Unmatched))
		for _, f := range result.Unmatched {
			fmt.Fprintf(w, "    - %s\n", f)
		}
	}

	if len(result.Signals) == 0 {
		fmt.Fprintln(w, "\n  No signals to compare.")
		return nil
	}

	// Signal table
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  %-35s %12s %10s\n", "Signal", "Agreement", "Accuracy")
	fmt.Fprintf(w, "  %s\n", strings.Repeat("─", 57))

	for _, s := range result.Signals {
		pct := s.Accuracy() * 100
		fmt.Fprintf(w, "  %-35s %5d/%-5d %7.0f%%\n", s.Signal, s.Agreed, s.Total, pct)
	}

	// Per-tier breakdown
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  Per-Tier Accuracy")
	fmt.Fprintf(w, "  %s\n", strings.Repeat("─", 40))
	for _, tier := range []model.Tier{model.Tier1, model.Tier2, model.Tier3} {
		agreed, total := result.TierAccuracy(tier)
		if total == 0 {
			continue
		}
		pct := float64(agreed) / float64(total) * 100
		label := tierLabel(tier)
		fmt.Fprintf(w, "  %-30s %5d/%-5d %5.0f%%\n", label, agreed, total, pct)
	}

	// Overall
	fmt.Fprintln(w)
	pct := result.OverallAccuracy() * 100
	fmt.Fprintf(w, "  Overall weighted accuracy: %.0f%%\n", pct)

	// Mismatches detail
	hasMismatches := false
	for _, s := range result.Signals {
		if len(s.Misses) > 0 {
			hasMismatches = true
			break
		}
	}
	if hasMismatches {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "  Mismatches")
		fmt.Fprintf(w, "  %s\n", strings.Repeat("─", 57))
		for _, s := range result.Signals {
			for _, m := range s.Misses {
				shortFile := m.File
				parts := strings.Split(shortFile, "/")
				if len(parts) > 3 {
					shortFile = strings.Join(parts[len(parts)-3:], "/")
				}
				fmt.Fprintf(w, "  %-35s %s\n", s.Signal, shortFile)
				fmt.Fprintf(w, "    expected: %s, got: %s\n", m.Expected, m.Got)
			}
		}
	}

	fmt.Fprintln(w)
	return nil
}

func tierLabel(t model.Tier) string {
	switch t {
	case model.Tier1:
		return "Tier 1 (>90% target)"
	case model.Tier2:
		return "Tier 2 (70-90% target)"
	case model.Tier3:
		return "Tier 3 (50-70% target)"
	default:
		return "Unknown"
	}
}
