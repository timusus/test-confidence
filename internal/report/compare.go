package report

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"sort"

	"github.com/timusus/test-confidence/internal/model"
	"github.com/timusus/test-confidence/internal/surface"
)

// Snapshot captures the key metrics from a scan for comparison.
type Snapshot struct {
	Label            string // human-readable label for trend charts (e.g. "3 months ago", "2025-01")
	TotalTestFiles   int
	TotalTestMethods int
	AntiPatterns     map[string]int // type → count
	OutputRatio      float64        // 0-1
	VerifyOnly       int
	ArgumentCaptor   int
	StubAndVerify    int
	VerifyOrdering   int
	TotalMocks       int // excluding fakes
	BoundaryMocks    int
	InternalMocks    int
	FakeCount        int
	SurfacesTested   int
	SurfacesUntested int
	SurfacesTotal    int

	// Style counts and percentages
	BehavioralFiles int     // absolute count of behavioral test methods
	StructuralFiles int     // absolute count of structural test methods
	BehavioralPct   float64 // % of methods that are behavioral
	StructuralPct   float64 // % of methods that are structural
	UnclassifiedPct float64 // % unclassified

	// Compose
	ComposeTestFiles int // files with Compose imports

	// Worst files (top 5 by structural indicator count)
	WorstFiles []WorstFile
}

// WorstFile captures a test file's structural coupling indicators.
type WorstFile struct {
	Name       string
	MockCount  int
	VerifyOnly int
	ArgCaptor  int
	StubVerify int
	Total      int
}

// LoadBaseline reads a previously saved JSON scan result and extracts a Snapshot.
func LoadBaseline(path string) (*Snapshot, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading baseline: %w", err)
	}

	var raw jsonScanResult
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing baseline JSON: %w", err)
	}

	s := &Snapshot{
		TotalTestFiles:   raw.TotalTestFiles,
		TotalTestMethods: raw.TotalTestMethods,
		AntiPatterns:     map[string]int{},
	}

	behavioral, structuralCount, unclassifiedCount := 0, 0, 0

	type fileCoupling struct {
		name       string
		mockCount  int
		verifyOnly int
		argCaptor  int
		stubVerify int
		total      int
	}
	var fileCouplings []fileCoupling

	for _, f := range raw.FileResults {
		for _, ap := range f.AntiPatterns {
			s.AntiPatterns[ap.Type]++
		}

		// Assertions
		out := f.Assertions.TargetDistribution["Output"] + f.Assertions.TargetDistribution["Exception"]
		inter := f.Assertions.TargetDistribution["Interaction"]

		fc := fileCoupling{name: shortPath(f.File.Path)}
		fc.verifyOnly = countStructuralType(f.Structural, "VerifyOnlyTest")
		fc.argCaptor = countStructuralType(f.Structural, "ArgumentCaptor")
		fc.stubVerify = countStructuralType(f.Structural, "StubAndVerify")

		s.VerifyOnly += fc.verifyOnly
		s.ArgumentCaptor += fc.argCaptor
		s.StubAndVerify += fc.stubVerify
		s.VerifyOrdering += countStructuralType(f.Structural, "VerifyOrdering")

		// Classify using per-method counts
		behavioral += f.Assertions.BehavioralMethods
		structuralCount += f.Assertions.StructuralMethods
		unclassifiedCount += f.Assertions.UnclassifiedMethods

		_ = out
		_ = inter

		// Doubles
		for _, d := range f.Doubles.Doubles {
			if d.Usage == "FakeUsage" {
				s.FakeCount++
				continue
			}
			s.TotalMocks++
			fc.mockCount++
			switch d.Placement {
			case "Boundary":
				s.BoundaryMocks++
			case "Internal":
				s.InternalMocks++
			}
		}

		fc.total = fc.verifyOnly + fc.argCaptor + fc.stubVerify
		if fc.total > 0 {
			fileCouplings = append(fileCouplings, fc)
		}
	}

	// Style counts and percentages (per-method)
	s.BehavioralFiles = behavioral
	s.StructuralFiles = structuralCount
	total := behavioral + structuralCount + unclassifiedCount
	if total > 0 {
		s.BehavioralPct = float64(behavioral) / float64(total) * 100
		s.StructuralPct = float64(structuralCount) / float64(total) * 100
		s.UnclassifiedPct = float64(unclassifiedCount) / float64(total) * 100
	}

	// Worst files (top 5)
	sort.Slice(fileCouplings, func(i, j int) bool {
		return fileCouplings[i].total > fileCouplings[j].total
	})
	limit := 5
	if len(fileCouplings) < limit {
		limit = len(fileCouplings)
	}
	for _, fc := range fileCouplings[:limit] {
		s.WorstFiles = append(s.WorstFiles, WorstFile{
			Name:       fc.name,
			MockCount:  fc.mockCount,
			VerifyOnly: fc.verifyOnly,
			ArgCaptor:  fc.argCaptor,
			StubVerify: fc.stubVerify,
			Total:      fc.total,
		})
	}

	// Compute output ratio
	totalOut := 0
	totalInter := 0
	for _, f := range raw.FileResults {
		totalOut += f.Assertions.TargetDistribution["Output"] + f.Assertions.TargetDistribution["Exception"]
		totalInter += f.Assertions.TargetDistribution["Interaction"]
	}
	if totalOut+totalInter > 0 {
		s.OutputRatio = float64(totalOut) / float64(totalOut+totalInter)
	}

	// Surfaces
	if raw.SurfaceAnalysis != nil {
		s.SurfacesTotal = len(raw.SurfaceAnalysis.Surfaces)
		s.SurfacesTested = raw.SurfaceAnalysis.TestedCount
		s.SurfacesUntested = raw.SurfaceAnalysis.UntestedCount
	}

	return s, nil
}

func countStructuralType(structural []jsonStructural, typ string) int {
	c := 0
	for _, s := range structural {
		if s.Type == typ {
			c++
		}
	}
	return c
}

// SnapshotFromCurrent builds a Snapshot from live scan results.
func SnapshotFromCurrent(result *model.ScanResult, sa *surface.SurfaceAnalysis) *Snapshot {
	s := &Snapshot{
		TotalTestFiles:   result.TotalTestFiles,
		TotalTestMethods: result.TotalTestMethods,
		AntiPatterns:     map[string]int{},
	}

	totalOut := 0
	totalInter := 0
	behavioral, structural, unclassified := 0, 0, 0

	// Per-file structural coupling for worst-files ranking
	type fileCoupling struct {
		name       string
		mockCount  int
		verifyOnly int
		argCaptor  int
		stubVerify int
		total      int
	}
	var fileCouplings []fileCoupling

	for _, f := range result.FileResults {
		for _, ap := range f.AntiPatterns {
			s.AntiPatterns[ap.Type.String()]++
		}

		totalOut += f.Assertions.TargetDistribution[model.Output] + f.Assertions.TargetDistribution[model.Exception]
		totalInter += f.Assertions.TargetDistribution[model.Interaction]

		s.ComposeTestFiles += f.Assertions.ComposeTestFiles

		// Classify using per-method counts
		behavioral += f.Assertions.BehavioralMethods
		structural += f.Assertions.StructuralMethods
		unclassified += f.Assertions.UnclassifiedMethods

		fc := fileCoupling{name: shortPath(f.File.Path)}
		for _, si := range f.Structural {
			switch si.Type {
			case model.VerifyOnlyTest:
				s.VerifyOnly++
				fc.verifyOnly++
			case model.ArgumentCaptor:
				s.ArgumentCaptor++
				fc.argCaptor++
			case model.StubAndVerify:
				s.StubAndVerify++
				fc.stubVerify++
			case model.VerifyOrdering:
				s.VerifyOrdering++
			}
		}

		for _, d := range f.Doubles.Doubles {
			if d.Usage == model.FakeUsage {
				s.FakeCount++
				continue
			}
			s.TotalMocks++
			fc.mockCount++
			switch d.Placement {
			case model.Boundary:
				s.BoundaryMocks++
			case model.Internal:
				s.InternalMocks++
			}
		}

		fc.total = fc.verifyOnly + fc.argCaptor + fc.stubVerify
		if fc.total > 0 {
			fileCouplings = append(fileCouplings, fc)
		}
	}

	// Style counts and percentages (per-method)
	s.BehavioralFiles = behavioral
	s.StructuralFiles = structural
	total := behavioral + structural + unclassified
	if total > 0 {
		s.BehavioralPct = float64(behavioral) / float64(total) * 100
		s.StructuralPct = float64(structural) / float64(total) * 100
		s.UnclassifiedPct = float64(unclassified) / float64(total) * 100
	}

	// Worst files (top 5)
	sort.Slice(fileCouplings, func(i, j int) bool {
		return fileCouplings[i].total > fileCouplings[j].total
	})
	limit := 5
	if len(fileCouplings) < limit {
		limit = len(fileCouplings)
	}
	for _, fc := range fileCouplings[:limit] {
		s.WorstFiles = append(s.WorstFiles, WorstFile{
			Name:       fc.name,
			MockCount:  fc.mockCount,
			VerifyOnly: fc.verifyOnly,
			ArgCaptor:  fc.argCaptor,
			StubVerify: fc.stubVerify,
			Total:      fc.total,
		})
	}

	if totalOut+totalInter > 0 {
		s.OutputRatio = float64(totalOut) / float64(totalOut+totalInter)
	}

	if sa != nil {
		s.SurfacesTotal = len(sa.Surfaces)
		s.SurfacesTested = sa.TestedCount
		s.SurfacesUntested = sa.UntestedCount
	}

	return s
}

// WriteComparisonReport writes a narrative comparison between old and new snapshots.
func WriteComparisonReport(w io.Writer, old, now *Snapshot) error {
	fmt.Fprintf(w, "Confidence — Comparison\n\n")

	// Scale
	fileDelta := now.TotalTestFiles - old.TotalTestFiles
	methodDelta := now.TotalTestMethods - old.TotalTestMethods
	fmt.Fprintf(w, "── Scale ─────────────────────────────────────────────────────\n\n")
	fmt.Fprintf(w, "  Test files:    %d → %d  (%s)\n", old.TotalTestFiles, now.TotalTestFiles, signedInt(fileDelta))
	fmt.Fprintf(w, "  Test methods:  %d → %d  (%s)\n", old.TotalTestMethods, now.TotalTestMethods, signedInt(methodDelta))

	// Test style delta
	if old.BehavioralPct > 0 || now.BehavioralPct > 0 {
		bDelta := now.BehavioralPct - old.BehavioralPct
		fmt.Fprintf(w, "  Test style:    %.0f%% → %.0f%% behavioral  %s\n",
			old.BehavioralPct, now.BehavioralPct,
			trend(bDelta > 1, bDelta < -1, true))
	}
	fmt.Fprintf(w, "\n")

	// Structural coupling — normalized per 100 methods
	fmt.Fprintf(w, "── Structural Coupling (per 100 test methods) ────────────────\n\n")

	oldM := max(old.TotalTestMethods, 1)
	nowM := max(now.TotalTestMethods, 1)

	outputDelta := now.OutputRatio - old.OutputRatio
	fmt.Fprintf(w, "  Output ratio:    %4.0f%% → %4.0f%%  %s\n",
		old.OutputRatio*100, now.OutputRatio*100, trend(outputDelta > 0.005, outputDelta < -0.005, true))

	writeRateRow(w, "Verify-only", old.VerifyOnly, oldM, now.VerifyOnly, nowM, false)
	writeRateRow(w, "ArgumentCaptor", old.ArgumentCaptor, oldM, now.ArgumentCaptor, nowM, false)
	writeRateRow(w, "Stub+verify", old.StubAndVerify, oldM, now.StubAndVerify, nowM, false)

	// Absolute structural counts
	fmt.Fprintf(w, "\n  Absolutes:\n")
	fmt.Fprintf(w, "  Behavioral methods: %d → %d  (%s)\n", old.BehavioralFiles, now.BehavioralFiles, signedInt(now.BehavioralFiles-old.BehavioralFiles))
	fmt.Fprintf(w, "  Structural methods: %d → %d  (%s)\n", old.StructuralFiles, now.StructuralFiles, signedInt(now.StructuralFiles-old.StructuralFiles))
	fmt.Fprintf(w, "  Stub+verify:      %d → %d  (%s)\n", old.StubAndVerify, now.StubAndVerify, signedInt(now.StubAndVerify-old.StubAndVerify))
	fmt.Fprintf(w, "\n")

	// Mock strategy
	fmt.Fprintf(w, "── Test Doubles Strategy ─────────────────────────────────────\n\n")

	fmt.Fprintf(w, "  Fakes:           %4d → %4d  (%s)\n", old.FakeCount, now.FakeCount, signedInt(now.FakeCount-old.FakeCount))
	fmt.Fprintf(w, "  Mocks:           %4d → %4d  (%s)\n", old.TotalMocks, now.TotalMocks, signedInt(now.TotalMocks-old.TotalMocks))
	oldInternalPct := pct(old.InternalMocks, old.TotalMocks)
	nowInternalPct := pct(now.InternalMocks, now.TotalMocks)
	fmt.Fprintf(w, "  Internal mock %%: %3.0f%% → %3.0f%%  %s\n",
		oldInternalPct, nowInternalPct, trend(nowInternalPct < oldInternalPct-1, nowInternalPct > oldInternalPct+1, true))
	fmt.Fprintf(w, "\n")

	// Surface coverage
	if old.SurfacesTotal > 0 || now.SurfacesTotal > 0 {
		fmt.Fprintf(w, "── Surface Coverage ──────────────────────────────────────────\n\n")
		oldPct := pct(old.SurfacesTested, old.SurfacesTotal)
		nowPct := pct(now.SurfacesTested, now.SurfacesTotal)
		fmt.Fprintf(w, "  Tested:    %d/%d (%2.0f%%) → %d/%d (%2.0f%%)  %s\n",
			old.SurfacesTested, old.SurfacesTotal, oldPct,
			now.SurfacesTested, now.SurfacesTotal, nowPct,
			trend(nowPct > oldPct+1, nowPct < oldPct-1, true))
		fmt.Fprintf(w, "  Untested:  %d → %d  (%s)\n",
			old.SurfacesUntested, now.SurfacesUntested, signedInt(now.SurfacesUntested-old.SurfacesUntested))
		fmt.Fprintf(w, "\n")
	}

	// Worst files comparison
	if len(old.WorstFiles) > 0 || len(now.WorstFiles) > 0 {
		writeWorstFilesComparison(w, old, now)
	}

	// Narrative summary
	fmt.Fprintf(w, "── Summary ───────────────────────────────────────────────────\n\n")
	writeNarrative(w, old, now)

	return nil
}

func writeRateRow(w io.Writer, label string, oldCount, oldTotal, nowCount, nowTotal int, higherIsBetter bool) {
	oldRate := float64(oldCount) / float64(oldTotal) * 100
	nowRate := float64(nowCount) / float64(nowTotal) * 100
	delta := nowRate - oldRate
	improved := delta < -0.3
	regressed := delta > 0.3
	if higherIsBetter {
		improved, regressed = regressed, improved
	}
	fmt.Fprintf(w, "  %-16s %4.1f → %4.1f  %s\n", label+":", oldRate, nowRate, trend(improved, regressed, higherIsBetter))
}

func writeNarrative(w io.Writer, old, now *Snapshot) {
	growth := float64(now.TotalTestMethods-old.TotalTestMethods) / float64(max(old.TotalTestMethods, 1)) * 100

	if growth > 20 {
		fmt.Fprintf(w, "  The test suite grew %.0f%% (%d → %d methods).\n", growth, old.TotalTestMethods, now.TotalTestMethods)
	}

	// Structural coupling trend
	oldSVRate := float64(old.StubAndVerify) / float64(max(old.TotalTestMethods, 1)) * 100
	nowSVRate := float64(now.StubAndVerify) / float64(max(now.TotalTestMethods, 1)) * 100
	oldACRate := float64(old.ArgumentCaptor) / float64(max(old.TotalTestMethods, 1)) * 100
	nowACRate := float64(now.ArgumentCaptor) / float64(max(now.TotalTestMethods, 1)) * 100

	if nowSVRate < oldSVRate-0.3 && nowACRate < oldACRate-0.3 {
		fmt.Fprintf(w, "  Structural coupling is declining — new tests are more behavioral.\n")
	} else if nowSVRate > oldSVRate+0.3 && nowACRate > oldACRate+0.3 {
		fmt.Fprintf(w, "  Structural coupling is increasing — new tests are more implementation-coupled.\n")
	}

	// Fake adoption
	fakeDelta := now.FakeCount - old.FakeCount
	if fakeDelta > 10 {
		fmt.Fprintf(w, "  Fake adoption is growing (+%d fakes) — a shift from mocking to behavioral testing.\n", fakeDelta)
	}

	// Surface coverage
	if now.SurfacesUntested < old.SurfacesUntested {
		covered := old.SurfacesUntested - now.SurfacesUntested
		fmt.Fprintf(w, "  %d previously-untested surfaces now have tests.\n", covered)
	} else if now.SurfacesUntested > old.SurfacesUntested {
		added := now.SurfacesUntested - old.SurfacesUntested
		fmt.Fprintf(w, "  %d new surfaces were added without tests.\n", added)
	}

	// Internal mock ratio
	oldInternalPct := pct(old.InternalMocks, old.TotalMocks)
	nowInternalPct := pct(now.InternalMocks, now.TotalMocks)
	if nowInternalPct > oldInternalPct+3 {
		fmt.Fprintf(w, "  Internal mock ratio increased (%.0f%% → %.0f%%) — new code is still mocking domain types.\n",
			oldInternalPct, nowInternalPct)
	} else if nowInternalPct < oldInternalPct-3 {
		fmt.Fprintf(w, "  Internal mock ratio decreased (%.0f%% → %.0f%%) — less mocking of domain types.\n",
			oldInternalPct, nowInternalPct)
	}
}

func writeWorstFilesComparison(w io.Writer, old, now *Snapshot) {
	if len(now.WorstFiles) == 0 {
		return
	}

	fmt.Fprintf(w, "── Worst Files ───────────────────────────────────────────────\n\n")

	// Build old lookup
	oldLookup := map[string]WorstFile{}
	for _, wf := range old.WorstFiles {
		oldLookup[wf.Name] = wf
	}

	seen := map[string]bool{}
	for _, wf := range now.WorstFiles {
		if seen[wf.Name] {
			continue
		}
		seen[wf.Name] = true
		delta := ""
		if owf, found := oldLookup[wf.Name]; found {
			d := wf.Total - owf.Total
			if d != 0 {
				delta = fmt.Sprintf("  (%s)", signedInt(d))
			} else {
				delta = "  (unchanged)"
			}
		} else {
			delta = "  (new)"
		}
		fmt.Fprintf(w, "  %-40s %d indicators%s\n", wf.Name, wf.Total, delta)
	}

	// Check if any old worst files dropped off (improved)
	nowLookup := map[string]bool{}
	for _, wf := range now.WorstFiles {
		nowLookup[wf.Name] = true
	}
	improved := 0
	for _, owf := range old.WorstFiles {
		if !nowLookup[owf.Name] {
			improved++
		}
	}
	if improved > 0 {
		fmt.Fprintf(w, "  %d previously worst files no longer in top 5\n", improved)
	}
	fmt.Fprintf(w, "\n")
}

func trend(improved, regressed, higherIsBetter bool) string {
	if improved {
		return "↑ improved"
	}
	if regressed {
		return "↓ regressed"
	}
	return "→ stable"
}

func signedInt(n int) string {
	if n > 0 {
		return fmt.Sprintf("+%d", n)
	}
	return fmt.Sprintf("%d", n)
}

func pct(n, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(n) / float64(total) * 100
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// round to 1 decimal place — unused but available
func round1(f float64) float64 {
	return math.Round(f*10) / 10
}
