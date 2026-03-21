package report

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/timusus/test-confidence/internal/coverage"
	gitpkg "github.com/timusus/test-confidence/internal/git"
	"github.com/timusus/test-confidence/internal/model"
	"github.com/timusus/test-confidence/internal/surface"
)

// antiPatternLabel maps anti-pattern types to human-readable descriptions.
var antiPatternLabel = map[model.AntiPatternType]string{
	model.ThreadSleep:      "sleep/delay calls",
	model.UnsafeDelay:      "unsafe delay patterns",
	model.EmptyTest:        "empty test bodies",
	model.IgnoredTest:      "@Ignored",
	model.ConditionalLogic: "conditional logic",
	model.ReflectionUsage:  "reflection usage",
	model.GodTestClass:     "god test classes",
	model.AssertionRoulette: "assertion roulette",
}

// structuralLabel maps structural indicator types to human-readable descriptions.
var structuralLabel = map[model.StructuralType]string{
	model.VerifyOrdering: "verifyOrder/verifySequence",
	model.ArgumentCaptor: "ArgumentCaptor",
	model.StubAndVerify:  "stub and verify the same method",
	model.VerifyOnlyTest: "only verify calls (no output assertions)",
}

// SetupCoupledFile is imported from scan package for terminal reporting.
type SetupCoupledFile struct {
	TestFile      string
	CoChangeRate  float64
	ProdChanges   int
}

// CostlyTest mirrors scan.CostlyTest for terminal reporting.
type CostlyTest struct {
	TestFile           string
	UnnecessaryChanges int
	TotalProdChanges   int
	CoChangeRate       float64
}

// RiskHotspot mirrors scan.RiskHotspot for terminal reporting.
type RiskHotspot struct {
	ProductionFile string
	TestFile       string
	Commits        int
	RecentCommits  int
	CouplingScore  float64
	VerifyOnly     int
	SetupRatio     float64
	ZeroAssertion  int
	Tautologies    int
	RiskScore      float64
	FixHint        string
	CostHint       string
}

// WriteTerminalReport writes a human-readable terminal report to w.
// gitAnalysis, surfaceAnalysis, setupCoupled, and coverageReport may be nil.
func WriteTerminalReport(w io.Writer, result *model.ScanResult, verbose bool, gitAnalysis *gitpkg.GitAnalysis, surfaceAnalysis *surface.SurfaceAnalysis, setupCoupled []SetupCoupledFile, costlyTests []CostlyTest, totalUnnecessary int, riskHotspots []RiskHotspot, coverageReport ...*coverage.CoverageReport) error {
	// Header
	platformLang := platformLanguage(result.Platform)
	fmt.Fprintf(w, "Confidence — %d test files, %d test methods (%s)\n",
		result.TotalTestFiles, result.TotalTestMethods, platformLang)

	if result.UnparseableFiles > 0 {
		fmt.Fprintf(w, "\n  Warning: %d files could not be parsed\n", result.UnparseableFiles)
	}

	// Codebase style summary — per-method classification
	if len(result.FileResults) > 0 {
		behavioral, structural, unclassified := 0, 0, 0
		totalFakes, totalMocksAll := 0, 0
		for _, fa := range result.FileResults {
			totalFakes += fa.Doubles.FakeCount
			for _, d := range fa.Doubles.Doubles {
				if d.Usage != model.FakeUsage {
					totalMocksAll++
				}
			}
			behavioral += fa.Assertions.BehavioralMethods
			structural += fa.Assertions.StructuralMethods
			unclassified += fa.Assertions.UnclassifiedMethods
		}
		total := behavioral + structural + unclassified
		if total > 0 {
			bPct := int(float64(behavioral) / float64(total) * 100)
			sPct := int(float64(structural) / float64(total) * 100)
			uPct := 100 - bPct - sPct
			styleLine := fmt.Sprintf("Test style: %d%% behavioral, %d%% structural, %d%% unclassified", bPct, sPct, uPct)
			fmt.Fprintf(w, "%s\n", styleLine)
		}
	}

	// Coverage summary in header
	var covReport *coverage.CoverageReport
	if len(coverageReport) > 0 {
		covReport = coverageReport[0]
	}
	if covReport != nil {
		fmt.Fprintf(w, "Line coverage: %.0f%% (%d/%d lines)\n",
			covReport.OverallLineCoverage()*100,
			covReport.TotalLinesCovered(),
			covReport.TotalLinesCovered()+covReport.TotalLinesMissed())
	}

	// Scope distribution
	sd := result.ScopeDistribution
	totalScope := sd.Local + sd.Device + sd.Snapshot
	if totalScope > 0 && (sd.Device > 0 || sd.Snapshot > 0) {
		parts := []string{}
		if sd.Local > 0 {
			parts = append(parts, fmt.Sprintf("%d local", sd.Local))
		}
		if sd.Device > 0 {
			parts = append(parts, fmt.Sprintf("%d device", sd.Device))
		}
		if sd.Snapshot > 0 {
			parts = append(parts, fmt.Sprintf("%d snapshot", sd.Snapshot))
		}
		fmt.Fprintf(w, "Test scope: %s\n", strings.Join(parts, ", "))
	}

	// Aggregate data across all file results
	antiPatternCounts := make(map[model.AntiPatternType]int)
	antiPatternDetails := make(map[model.AntiPatternType][]model.AntiPattern)
	var zeroAssertionMethods []model.MethodRef
	var totalOutput, totalInteraction int
	structuralCounts := make(map[model.StructuralType]int)
	structuralDetails := make(map[model.StructuralType][]model.StructuralIndicator)
	var allTautologies []model.TautologyFlag
	var allDoubles []model.TestDouble
	var mostMockedTypes []model.TypeFrequency
	var totalSetupOnly, totalVerification, totalBoundary, totalInternal, totalUnknown int

	for _, fa := range result.FileResults {
		for _, ap := range fa.AntiPatterns {
			antiPatternCounts[ap.Type]++
			antiPatternDetails[ap.Type] = append(antiPatternDetails[ap.Type], ap)
		}
		zeroAssertionMethods = append(zeroAssertionMethods, fa.Assertions.ZeroAssertionMethods...)
		totalOutput += fa.Assertions.TargetDistribution[model.Output]
		totalInteraction += fa.Assertions.TargetDistribution[model.Interaction]

		for _, si := range fa.Structural {
			structuralCounts[si.Type]++
			structuralDetails[si.Type] = append(structuralDetails[si.Type], si)
		}

		allTautologies = append(allTautologies, fa.Tautologies...)

		for _, d := range fa.Doubles.Doubles {
			allDoubles = append(allDoubles, d)
			// Exclude fakes from mock-specific metrics
			if d.Usage == model.FakeUsage {
				continue
			}
			switch d.Usage {
			case model.SetupOnly:
				totalSetupOnly++
			case model.Verification:
				totalVerification++
			case model.Both:
				totalSetupOnly++
				totalVerification++
			}
			switch d.Placement {
			case model.Boundary:
				totalBoundary++
			case model.Internal:
				totalInternal++
			case model.UnknownPlacement:
				totalUnknown++
			}
		}

		// Merge most-mocked types (take the longest list as approximation;
		// the scan engine should aggregate, but we handle per-file here)
		if len(fa.Doubles.MostMockedTypes) > len(mostMockedTypes) {
			mostMockedTypes = fa.Doubles.MostMockedTypes
		}
	}

	// Assertion Strength section (Tier 2 — Good confidence)
	if result.AggregateStrength != nil {
		strong := result.AggregateStrength[model.Strong]
		medium := result.AggregateStrength[model.Medium]
		weak := result.AggregateStrength[model.Weak]
		totalStrength := strong + medium + weak
		if totalStrength > 0 {
			fmt.Fprintf(w, "\n── Assertion Strength ────────────────────────────────────────\n\n")
			sPct := int(float64(strong) / float64(totalStrength) * 100)
			mPct := int(float64(medium) / float64(totalStrength) * 100)
			wPct := 100 - sPct - mPct
			fmt.Fprintf(w, "  Strong (equality, containment):  %d (%d%%)\n", strong, sPct)
			fmt.Fprintf(w, "  Medium (boolean, existence):     %d (%d%%)\n", medium, mPct)
			fmt.Fprintf(w, "  Weak (null checks):              %d (%d%%)\n", weak, wPct)
		}
	}

	// Findings section — only anti-patterns (high confidence, Tier 1)
	if len(antiPatternCounts) > 0 {
		fmt.Fprintf(w, "\n── Findings ──────────────────────────────────────────────────\n\n")

		orderedTypes := []model.AntiPatternType{
			model.ThreadSleep, model.UnsafeDelay, model.EmptyTest, model.IgnoredTest,
			model.ConditionalLogic, model.ReflectionUsage, model.GodTestClass, model.AssertionRoulette,
		}
		for _, apt := range orderedTypes {
			count := antiPatternCounts[apt]
			if count == 0 {
				continue
			}
			label := antiPatternLabel[apt]
			fmt.Fprintf(w, "  %d tests contain %s\n", count, label)
			if verbose {
				for _, ap := range antiPatternDetails[apt] {
					if ap.Class != "" {
						fmt.Fprintf(w, "    %s (%s)\n", ap.File, ap.Class)
					} else {
						fmt.Fprintf(w, "    %s:%d (%s)\n", ap.File, ap.Line, ap.Method)
					}
				}
			}
		}
	}

	// Structural Coupling section
	hasStructural := len(structuralCounts) > 0 || (totalOutput+totalInteraction) > 0 || len(zeroAssertionMethods) > 0
	if hasStructural {
		fmt.Fprintf(w, "\n── Structural Coupling Indicators ────────────────────────────\n\n")

		// Output-vs-interaction ratio
		totalOI := totalOutput + totalInteraction
		if totalOI > 0 {
			outputPct := float64(totalOutput) / float64(totalOI)
			interactionPct := float64(totalInteraction) / float64(totalOI)
			fmt.Fprintf(w, "  Output-vs-interaction ratio: %.2f (%d%% output, %d%% verify)\n",
				outputPct, int(outputPct*100), int(interactionPct*100))
		}

		orderedStructural := []model.StructuralType{
			model.VerifyOnlyTest, model.VerifyOrdering, model.ArgumentCaptor, model.StubAndVerify,
		}
		for _, st := range orderedStructural {
			count := structuralCounts[st]
			if count == 0 {
				continue
			}
			label := structuralLabel[st]
			fmt.Fprintf(w, "  %d test methods use %s\n", count, label)
			if verbose {
				for _, si := range structuralDetails[st] {
					fmt.Fprintf(w, "    %s:%d (%s)\n", si.File, si.Line, si.Method)
				}
			}
		}

		// Zero-assertion data is available in --json output for programmatic consumers
		// but not shown in terminal — the verify-only count above captures the actionable subset,
		// and the remainder mixes real issues with detection gaps (Compose patterns, custom helpers).

		// Note Compose test files if any were detected
		composeTestFiles := 0
		for _, fa := range result.FileResults {
			composeTestFiles += fa.Assertions.ComposeTestFiles
		}
		if composeTestFiles > 0 {
			fmt.Fprintf(w, "  (%d Compose UI test files detected — assertions not fully analyzed)\n", composeTestFiles)
		}

		// Note files with high unclassified statement ratios — these likely use
		// assertion patterns the tool doesn't recognize.
		unrecognizedFiles := 0
		for _, fa := range result.FileResults {
			if fa.Assertions.ComposeTestFiles > 0 {
				continue // Compose files already noted above
			}
			if fa.Assertions.TotalAssertions > 0 {
				continue // file has recognized assertions
			}
			if fa.Assertions.UnclassifiedStatements == 0 {
				continue
			}
			// Count total statements in this file's test methods
			totalStmts := 0
			for _, cls := range fa.File.Classes {
				for _, m := range cls.Methods {
					totalStmts += len(m.Statements)
				}
			}
			if totalStmts > 0 && float64(fa.Assertions.UnclassifiedStatements)/float64(totalStmts) > 0.5 {
				unrecognizedFiles++
			}
		}
		if unrecognizedFiles > 0 {
			fmt.Fprintf(w, "  (%d test files use assertion patterns not recognized by this tool)\n", unrecognizedFiles)
		}
	}

	// Worst Files section — rank by structural coupling density per file
	writeWorstFiles(w, result)

	// Doubles / Mock Placement section
	totalDoubles := len(allDoubles)
	if totalDoubles > 0 {
		// Setup-only vs verification
		totalUsage := totalSetupOnly + totalVerification
		if totalUsage > 0 {
			setupPct := int(float64(totalSetupOnly) / float64(totalUsage) * 100)
			verifyPct := 100 - setupPct
			fmt.Fprintf(w, "\n  Setup-only doubles: %d%%    Verification doubles: %d%%\n", setupPct, verifyPct)
		}

		fmt.Fprintf(w, "\n── Mock Placement (heuristic — configure via .confidence.yaml) ──\n\n")

		// Percentages are computed against non-fake doubles only
		mockTotal := totalBoundary + totalInternal + totalUnknown
		if mockTotal > 0 {
			boundaryPct := int(float64(totalBoundary) / float64(mockTotal) * 100)
			internalPct := int(float64(totalInternal) / float64(mockTotal) * 100)
			unknownPct := 100 - boundaryPct - internalPct
			fmt.Fprintf(w, "  Boundary mocks: %d%%    Internal mocks: %d%%\n", boundaryPct, internalPct)
			if unknownPct > 0 {
				fmt.Fprintf(w, "  Unclassified: %d%% — add type patterns to .confidence.yaml to improve\n", unknownPct)
			}
		}

		if len(mostMockedTypes) > 0 {
			// Build a placement lookup from all doubles
			typePlacement := map[string]model.Placement{}
			for _, d := range allDoubles {
				if d.Usage != model.FakeUsage {
					typePlacement[d.TypeName] = d.Placement
				}
			}

			// Show most-mocked with placement context
			fmt.Fprintf(w, "  Most-mocked types:\n")
			limit := 10
			if len(mostMockedTypes) < limit {
				limit = len(mostMockedTypes)
			}
			for _, mt := range mostMockedTypes[:limit] {
				placement := typePlacement[mt.TypeName]
				label := ""
				switch placement {
				case model.Internal:
					label = " [internal — consider using a fake]"
				case model.Boundary:
					label = " [boundary]"
				}
				fmt.Fprintf(w, "    %-30s (%dx)%s\n", mt.TypeName, mt.Count, label)
			}
		}

		if verbose {
			fmt.Fprintf(w, "\n  Details:\n")
			for _, d := range allDoubles {
				fmt.Fprintf(w, "    %s (%s) — %s, %s\n", d.TypeName, d.VariableName, d.Usage, d.Placement)
			}
		}
	}

	// Pass-through tests — assert the same value they stubbed
	if len(allTautologies) > 0 {
		fmt.Fprintf(w, "\n── Pass-Through Tests ────────────────────────────────────────\n\n")
		fmt.Fprintf(w, "  %d tests assert the same value they stubbed — no real logic tested\n", len(allTautologies))
		if verbose {
			for _, t := range allTautologies {
				fmt.Fprintf(w, "    %s:%d (%s) stub=%s assert=%s\n",
					t.File, t.Line, t.Method, t.StubIdentifier, t.AssertIdentifier)
			}
		}
	}

	// Surface Coverage section
	if surfaceAnalysis != nil && len(surfaceAnalysis.Surfaces) > 0 {
		writeSurfaceSection(w, surfaceAnalysis)
	}

	// Untested Complex Files section (non-surface business logic with churn)
	if surfaceAnalysis != nil && len(surfaceAnalysis.UntestedComplexFiles) > 0 {
		writeUntestedComplexSection(w, surfaceAnalysis.UntestedComplexFiles)
	}

	// Git Analysis section (Tier 3 — present with caveats)
	if gitAnalysis != nil && len(gitAnalysis.FilePairStats) > 0 {
		fmt.Fprintf(w, "\n── Git Signals (from %d commits, %s workflow) ──\n\n",
			gitAnalysis.AnalyzedCommits, gitAnalysis.WorkflowType)
		if gitAnalysis.FilteredCommits > 0 {
			fmt.Fprintf(w, "  %d bulk commits filtered (>50 files)\n", gitAnalysis.FilteredCommits)
		}

		// Find interesting pairs: high co-change, high structural coupling, high churn
		var highCoChange, highStructural, highChurn []gitpkg.FilePairStat
		for _, s := range gitAnalysis.FilePairStats {
			if s.ProdChanges < 3 {
				continue // not enough data
			}
			if s.CoChangeRatio > 0.7 {
				highCoChange = append(highCoChange, s)
			}
			if s.SmallProdChanges >= 2 && s.StructuralCouplingRate > 0.5 {
				highStructural = append(highStructural, s)
			}
			if s.TestChurnRatio > 1.5 {
				highChurn = append(highChurn, s)
			}
		}

		if len(highCoChange) > 0 {
			fmt.Fprintf(w, "  %d file pairs have high co-change rate (>70%% — test changes with most code changes)\n", len(highCoChange))
		}
		if len(highStructural) > 0 {
			fmt.Fprintf(w, "  %d file pairs show structural coupling signal (test changes on small code changes)\n", len(highStructural))
		}
		if len(highChurn) > 0 {
			fmt.Fprintf(w, "  %d test files change more often than their production counterparts\n", len(highChurn))
		}

		if verbose {
			for _, s := range gitAnalysis.FilePairStats {
				if s.ProdChanges < 3 {
					continue
				}
				fmt.Fprintf(w, "    %s → co-change: %.0f%% (%d/%d), structural: %.0f%%, churn: %.1fx\n",
					shortPath(s.TestFile),
					s.CoChangeRatio*100, s.CoChanges, s.ProdChanges,
					s.StructuralCouplingRate*100,
					s.TestChurnRatio)
			}
		}

		if len(highCoChange) == 0 && len(highStructural) == 0 && len(highChurn) == 0 {
			fmt.Fprintf(w, "  No significant co-change patterns detected\n")
		}
	}

	// Observed cost — test files that required the most unnecessary changes
	if totalUnnecessary > 0 {
		fmt.Fprintf(w, "\n── Observed Cost ─────────────────────────────────────────────\n\n")
		fmt.Fprintf(w, "  %d test-file changes were triggered by small code tweaks in the last %d commits.\n",
			totalUnnecessary, func() int { if gitAnalysis != nil { return gitAnalysis.AnalyzedCommits }; return 0 }())
		fmt.Fprintf(w, "  These are likely unnecessary — the test broke from refactoring, not behavior changes.\n\n")
		fmt.Fprintf(w, "  Costliest test files:\n")
		limit := 8
		if len(costlyTests) < limit {
			limit = len(costlyTests)
		}
		for _, ct := range costlyTests[:limit] {
			fmt.Fprintf(w, "    %-42s %d unnecessary changes (%.0f%% co-change)\n",
				shortPath(ct.TestFile), ct.UnnecessaryChanges, ct.CoChangeRate*100)
		}
		if len(setupCoupled) > 0 {
			fmt.Fprintf(w, "\n  %d of these are behavioral tests coupled through mock setup, not assertions.\n", len(setupCoupled))
		}
	}


	// Setup-Heavy Tests section — files with high setup-to-assertion ratio
	type setupHeavyFile struct {
		path            string
		setupStatements int
		assertionCount  int
		ratio           float64
	}
	var setupHeavyFiles []setupHeavyFile
	for _, fa := range result.FileResults {
		sc := fa.SetupComplexity
		if sc.Ratio > 5.0 && sc.SetupStatements > 0 && sc.AssertionCount > 0 {
			setupHeavyFiles = append(setupHeavyFiles, setupHeavyFile{
				path:            shortPath(fa.File.Path),
				setupStatements: sc.SetupStatements,
				assertionCount:  sc.AssertionCount,
				ratio:           sc.Ratio,
			})
		}
	}
	if len(setupHeavyFiles) > 0 {
		// Sort by ratio descending
		sort.Slice(setupHeavyFiles, func(i, j int) bool {
			return setupHeavyFiles[i].ratio > setupHeavyFiles[j].ratio
		})
		fmt.Fprintf(w, "\n── Setup-Heavy Tests ─────────────────────────────────────────\n\n")
		limit := 10
		if len(setupHeavyFiles) < limit {
			limit = len(setupHeavyFiles)
		}
		for _, f := range setupHeavyFiles[:limit] {
			fmt.Fprintf(w, "  %-40s %d setup lines, %d assertions (%.0f:1)\n",
				f.path, f.setupStatements, f.assertionCount, f.ratio)
		}
	}

	// Risk Hotspots section — high-churn code with fragile tests (Tier 2)
	if len(riskHotspots) > 0 {
		fmt.Fprintf(w, "\n── Risk Hotspots — high-churn code with fragile tests ────────\n\n")
		limit := 8
		if len(riskHotspots) < limit {
			limit = len(riskHotspots)
		}
		for _, h := range riskHotspots[:limit] {
			// File name and churn
			name := shortPath(h.ProductionFile)
			parts := []string{}
			if h.VerifyOnly > 0 {
				parts = append(parts, fmt.Sprintf("%d verify-only", h.VerifyOnly))
			}
			if h.Tautologies > 0 {
				parts = append(parts, fmt.Sprintf("%d pass-through", h.Tautologies))
			}
			if h.SetupRatio > 5.0 {
				parts = append(parts, fmt.Sprintf("setup %.0f:1", h.SetupRatio))
			}
			if h.ZeroAssertion > 0 {
				parts = append(parts, fmt.Sprintf("%d zero-assertion", h.ZeroAssertion))
			}
			detail := ""
			if len(parts) > 0 {
				detail = " — " + strings.Join(parts, ", ")
			}
			fmt.Fprintf(w, "  %-40s %d commits, score %.0f%s\n", name, h.Commits, h.CouplingScore, detail)
			fmt.Fprintf(w, "    Cost: %s\n", h.CostHint)
			fmt.Fprintf(w, "    Fix:  %s\n\n", h.FixHint)
		}
		if len(riskHotspots) > limit {
			fmt.Fprintf(w, "  ... and %d more (use --json for full list)\n", len(riskHotspots)-limit)
		}
	}

	// Recommendations section — prioritized with cost and fix context
	totalMocks := totalBoundary + totalInternal + totalUnknown
	recs := buildTopActions(
		antiPatternCounts, structuralCounts,
		totalBoundary, totalInternal, totalMocks,
		surfaceAnalysis, gitAnalysis,
		result.TotalTestMethods,
		len(setupHeavyFiles),
	)
	if len(recs) > 0 {
		fmt.Fprintf(w, "\n── Where To Start ────────────────────────────────────────────\n\n")
		limit := 5
		if len(recs) < limit {
			limit = len(recs)
		}
		for i, r := range recs[:limit] {
			fmt.Fprintf(w, "  %d. %s\n\n", i+1, r.text)
		}
	}

	// Footer
	if !verbose {
		fmt.Fprintf(w, "\n  Use --verbose for per-file detail.\n")
	}
	fmt.Fprintf(w, "  Use --json for machine-readable output.\n")

	return nil
}

// recommendation is a prioritized action item with context about cost and fix.
type recommendation struct {
	priority int // lower = more important
	text     string
}

// buildTopActions generates prioritized action items with cost context and fix suggestions.
func buildTopActions(
	antiPatterns map[model.AntiPatternType]int,
	structural map[model.StructuralType]int,
	totalBoundary, totalInternal, mockTotal int,
	sa *surface.SurfaceAnalysis,
	ga *gitpkg.GitAnalysis,
	totalMethods int,
	setupHeavyCount int,
) []recommendation {
	var recs []recommendation

	// 1. Untested high-churn surfaces
	if sa != nil && sa.UntestedCount > 0 {
		// Find the highest-risk untested surface (by recent churn, then total, then size)
		var top *surface.SurfaceChurn
		for i := range sa.UntestedByChurn {
			sc := &sa.UntestedByChurn[i]
			if top == nil || sc.RecentChurn > top.RecentChurn ||
				(sc.RecentChurn == top.RecentChurn && sc.Surface.Lines > top.Surface.Lines) {
				top = sc
			}
		}

		r := recommendation{priority: 1}
		if top != nil && (top.RecentChurn > 0 || top.Surface.Lines > 100) {
			detail := top.Surface.Name
			if top.Surface.Lines > 0 {
				detail += fmt.Sprintf(" (%d lines, %d functions", top.Surface.Lines, top.Surface.FunctionCount)
				if top.RecentChurn > 0 {
					detail += fmt.Sprintf(", %d commits in 3 months", top.RecentChurn)
				}
				detail += ")"
			}
			r.text = fmt.Sprintf("%d surfaces have no tests. Highest risk: %s.\n"+
				"     Cost: bugs in untested surfaces are caught in production, not CI.\n"+
				"     Fix:  start with a behavioral test — assert on observable state, not implementation.\n"+
				"           For ViewModels: provide fakes, call a method, assert on the resulting StateFlow value.\n"+
				"           For Screens: use Robolectric + ComposeTestRule, assert on displayed content.",
				sa.UntestedCount, detail)
		} else {
			r.text = fmt.Sprintf("%d surfaces have no tests.\n"+
				"     Cost: changes to these surfaces have no automated safety net.\n"+
				"     Fix:  see Surface Coverage above for the full list.",
				sa.UntestedCount)
		}
		recs = append(recs, r)
	}

	// 2. Stub-and-verify overlap
	if c := structural[model.StubAndVerify]; c > 5 {
		r := recommendation{priority: 2}
		r.text = fmt.Sprintf("%d tests stub and verify the same method.\n"+
			"     Cost: these tests break on every refactor (even safe ones) because they assert on wiring.\n"+
			"           Every internal restructure requires updating these tests, slowing refactoring.\n"+
			"     Fix:  replace `verify { repo.save(x) }` with an assertion on the result of saving.\n"+
			"           Use a Fake that captures state: `assertThat(fakeRepo.saved).contains(x)`.\n"+
			"           Stub-only (no verify) is fine — the test checks the output, not the call.",
			c)
		recs = append(recs, r)
	}

	// 3. High internal mock ratio
	if mockTotal > 0 {
		internalPct := float64(totalInternal) / float64(mockTotal) * 100
		if internalPct > 50 {
			r := recommendation{priority: 3}
			r.text = fmt.Sprintf("%.0f%% of mocks are on internal types (use cases, domain objects).\n"+
				"     Cost: mocking internals couples tests to implementation structure. When you refactor\n"+
				"           how components collaborate, these tests break even though behavior is unchanged.\n"+
				"     Fix:  replace mockk<GetUser>() with a FakeGetUser that returns test data.\n"+
				"           Fakes survive refactoring because they implement the interface, not record calls.\n"+
				"           Only mock at boundaries: network clients, databases, platform APIs.",
				internalPct)
			recs = append(recs, r)
		}
	}

	// 4. Structurally-coupled file pairs from git
	if ga != nil {
		highStructural := 0
		var worstPair gitpkg.FilePairStat
		for _, s := range ga.FilePairStats {
			if s.ProdChanges >= 3 && s.SmallProdChanges >= 2 && s.StructuralCouplingRate > 0.5 {
				highStructural++
				if s.StructuralCouplingRate > worstPair.StructuralCouplingRate {
					worstPair = s
				}
			}
		}
		if highStructural > 3 {
			r := recommendation{priority: 4}
			worstName := shortPath(worstPair.TestFile)
			r.text = fmt.Sprintf("%d test files change even on small code tweaks (e.g., %s at %.0f%%).\n"+
				"     Cost: every minor refactor forces test updates, creating friction against improving code.\n"+
				"           Engineers avoid refactoring because \"it'll break the tests.\"\n"+
				"     Fix:  these tests likely use verify calls or mirror the implementation structure.\n"+
				"           Rewrite to assert on outputs: given input X, expect output Y.\n"+
				"           Use --verbose under Git Signals to see the full list.",
				highStructural, worstName, worstPair.StructuralCouplingRate*100)
			recs = append(recs, r)
		}
	}

	// 5. ArgumentCaptor heavy usage
	if c := structural[model.ArgumentCaptor]; c > 20 {
		perMethod := float64(c) / float64(maxInt(totalMethods, 1)) * 100
		r := recommendation{priority: 5}
		r.text = fmt.Sprintf("%d tests use ArgumentCaptor (%.1f%% of methods).\n"+
			"     Cost: capturing arguments to verify what was passed to collaborators tests implementation\n"+
			"           wiring, not behavior. These tests are fragile to parameter changes.\n"+
			"     Fix:  instead of capturing args passed to a mock, use a Fake that records what it received.\n"+
			"           Or better: assert on the end result, not the intermediate calls.",
			c, perMethod)
		recs = append(recs, r)
	}

	// 6. Ignored tests (quick win)
	if c := antiPatterns[model.IgnoredTest]; c > 3 {
		r := recommendation{priority: 6}
		r.text = fmt.Sprintf("%d tests are @Ignored.\n"+
			"     Cost: ignored tests give false confidence — the suite looks bigger than it is.\n"+
			"           They also rot: the longer they're ignored, the harder they are to fix.\n"+
			"     Fix:  for each one, decide: fix it (if the test is valid) or delete it (if it's stale).\n"+
			"           This is a quick cleanup — 30 minutes of decision-making.",
			c)
		recs = append(recs, r)
	}

	// 7. Setup-heavy tests
	if setupHeavyCount > 0 {
		r := recommendation{priority: 7}
		r.text = fmt.Sprintf("%d test files have setup-to-assertion ratios above 5:1.\n"+
			"     Cost: excessive mock setup obscures test intent and makes tests fragile to refactoring.\n"+
			"           Each stub is a coupling point — when the production code changes, the stubs break.\n"+
			"     Fix:  replace fine-grained stubs with Fakes that provide sensible defaults.\n"+
			"           A FakeRepository that returns test data eliminates 10 every{} lines with one object.",
			setupHeavyCount)
		recs = append(recs, r)
	}

	// 8. Verify-only tests
	if c := structural[model.VerifyOnlyTest]; c > 20 {
		r := recommendation{priority: 8}
		r.text = fmt.Sprintf("%d test methods contain only verify calls with no output assertions.\n"+
			"     Cost: these tests check that code was called, not that it worked correctly.\n"+
			"           A passing test doesn't mean the feature works — only that the wiring exists.\n"+
			"     Fix:  add assertions on the observable result. If the method has side effects,\n"+
			"           use a Fake boundary that captures the effect and assert on it.",
			c)
		recs = append(recs, r)
	}

	return recs
}

// writeWorstFiles ranks test files by structural coupling score and shows the top offenders.
func writeWorstFiles(w io.Writer, result *model.ScanResult) {
	type fileCoupling struct {
		path          string
		score         float64
		verifyOnly    int
		argCaptor     int
		stubAndVerify int
		verifyOrder   int
		mockCount     int // number of mock dependencies (excluding fakes)
	}

	var files []*fileCoupling
	for _, fa := range result.FileResults {
		if fa.StructuralCouplingScore == 0 {
			continue
		}
		fc := &fileCoupling{
			path:  shortPath(fa.File.Path),
			score: fa.StructuralCouplingScore,
		}
		for _, si := range fa.Structural {
			switch si.Type {
			case model.VerifyOnlyTest:
				fc.verifyOnly++
			case model.ArgumentCaptor:
				fc.argCaptor++
			case model.StubAndVerify:
				fc.stubAndVerify++
			case model.VerifyOrdering:
				fc.verifyOrder++
			}
		}
		for _, d := range fa.Doubles.Doubles {
			if d.Usage != model.FakeUsage {
				fc.mockCount++
			}
		}
		files = append(files, fc)
	}

	if len(files) == 0 {
		return
	}

	// Sort by score descending
	sort.Slice(files, func(i, j int) bool {
		return files[i].score > files[j].score
	})

	// Only show if the top file has a meaningful score
	if files[0].score < 10 {
		return
	}

	fmt.Fprintf(w, "\n── Worst Files (by structural coupling score) ────────────────\n\n")
	limit := 5
	if len(files) < limit {
		limit = len(files)
	}
	for _, fc := range files[:limit] {
		parts := []string{}
		if fc.mockCount > 5 {
			parts = append(parts, fmt.Sprintf("%d mocks", fc.mockCount))
		}
		if fc.verifyOnly > 0 {
			parts = append(parts, fmt.Sprintf("%d verify-only", fc.verifyOnly))
		}
		if fc.argCaptor > 0 {
			parts = append(parts, fmt.Sprintf("%d argument captors", fc.argCaptor))
		}
		if fc.stubAndVerify > 0 {
			parts = append(parts, fmt.Sprintf("%d stub-and-verify", fc.stubAndVerify))
		}
		if fc.verifyOrder > 0 {
			parts = append(parts, fmt.Sprintf("%d verify ordering", fc.verifyOrder))
		}
		fmt.Fprintf(w, "  %-40s score: %.0f  %s\n", fc.path, fc.score, strings.Join(parts, ", "))
	}

	// Score distribution summary
	writeScoreDistribution(w, result)
}

// writeScoreDistribution shows a summary of score distribution across all files.
func writeScoreDistribution(w io.Writer, result *model.ScanResult) {
	if len(result.FileResults) == 0 {
		return
	}

	var scores []float64
	for _, fa := range result.FileResults {
		scores = append(scores, fa.StructuralCouplingScore)
	}
	sort.Float64s(scores)

	n := len(scores)
	mean := 0.0
	for _, s := range scores {
		mean += s
	}
	mean /= float64(n)

	median := scores[n/2]
	if n%2 == 0 && n > 1 {
		median = (scores[n/2-1] + scores[n/2]) / 2
	}

	p90Idx := int(float64(n) * 0.9)
	if p90Idx >= n {
		p90Idx = n - 1
	}
	p90 := scores[p90Idx]

	fmt.Fprintf(w, "\n  Score distribution: mean=%.0f  median=%.0f  p90=%.0f  (0=behavioral, 100=structural)\n", mean, median, p90)
	fmt.Fprintf(w, "  Weights are initial estimates — not yet calibrated (see PLAN.md Phase 9.1).\n")
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func shortPath(path string) string {
	// Show last 3 path segments for readability
	parts := strings.Split(path, "/")
	if len(parts) > 3 {
		return strings.Join(parts[len(parts)-3:], "/")
	}
	return path
}

func writeSurfaceSection(w io.Writer, sa *surface.SurfaceAnalysis) {
	// Count by type
	typeCounts := map[surface.SurfaceType]int{}
	for _, s := range sa.Surfaces {
		typeCounts[s.Type]++
	}

	total := len(sa.Surfaces)
	var parts []string
	for _, st := range []surface.SurfaceType{surface.Screen, surface.ViewModel, surface.Activity, surface.Fragment, surface.View, surface.ViewController} {
		if c := typeCounts[st]; c > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", c, pluralizeSurfaceType(st, c)))
		}
	}

	fmt.Fprintf(w, "\n── Surface Coverage ──────────────────────────────────────────\n\n")
	fmt.Fprintf(w, "  Found %d surfaces (%s)\n", total, strings.Join(parts, ", "))

	if total > 0 {
		testedPct := int(float64(sa.TestedCount) / float64(total) * 100)
		fmt.Fprintf(w, "  Tested: %d (%d%%)    Untested: %d\n", sa.TestedCount, testedPct, sa.UntestedCount)
	}

	if len(sa.UntestedByChurn) > 0 {
		fmt.Fprintf(w, "\n  Untested surfaces (highest risk first):\n")
		for _, sc := range sa.UntestedByChurn {
			s := sc.Surface
			sizeInfo := ""
			if s.Lines > 0 {
				sizeInfo = fmt.Sprintf("%d lines, %d fns", s.Lines, s.FunctionCount)
			}
			churnInfo := ""
			if sc.RecentChurn > 0 {
				churnInfo = fmt.Sprintf("%d commits (3mo)", sc.RecentChurn)
			} else if sc.GitChurn > 0 {
				churnInfo = fmt.Sprintf("%d commits", sc.GitChurn)
			}
			covInfo := ""
			if s.HasCoverageData {
				covInfo = fmt.Sprintf("%.0f%% covered", s.LineCoverage*100)
			}

			parts := []string{}
			if sizeInfo != "" {
				parts = append(parts, sizeInfo)
			}
			if churnInfo != "" {
				parts = append(parts, churnInfo)
			}
			if covInfo != "" {
				parts = append(parts, covInfo)
			}

			detail := strings.Join(parts, ", ")
			if detail != "" {
				fmt.Fprintf(w, "    %-28s %s — no tests\n", sc.Surface.Name, detail)
			} else {
				fmt.Fprintf(w, "    %-28s no tests\n", sc.Surface.Name)
			}
		}
	} else if sa.UntestedCount > 0 {
		fmt.Fprintf(w, "\n  Untested surfaces:\n")
		for _, s := range sa.Surfaces {
			if !s.HasTest {
				fmt.Fprintf(w, "    %s\n", s.Name)
			}
		}
	}
}

func writeUntestedComplexSection(w io.Writer, files []surface.UntestedComplexFile) {
	fmt.Fprintf(w, "\n── Untested Business Logic ───────────────────────────────────\n\n")
	fmt.Fprintf(w, "  %d production files with significant complexity and churn have no tests.\n", len(files))
	fmt.Fprintf(w, "  These are not UI surfaces — they are repositories, managers, services,\n")
	fmt.Fprintf(w, "  and other business logic that carries risk when untested.\n\n")

	limit := 15
	if len(files) < limit {
		limit = len(files)
	}
	for _, f := range files[:limit] {
		churnInfo := ""
		if f.RecentChurn > 0 {
			churnInfo = fmt.Sprintf("%d commits (3mo)", f.RecentChurn)
		} else {
			churnInfo = fmt.Sprintf("%d commits", f.TotalChurn)
		}
		fmt.Fprintf(w, "    %-40s %d lines, %d fns, %s\n", f.Name, f.Lines, f.FunctionCount, churnInfo)
	}
	if len(files) > limit {
		fmt.Fprintf(w, "    ... and %d more (use --json for full list)\n", len(files)-limit)
	}
}

func pluralizeSurfaceType(st surface.SurfaceType, count int) string {
	name := st.String()
	if count == 1 {
		return name
	}
	switch st {
	case surface.Activity:
		return "Activities"
	default:
		return name + "s"
	}
}

func platformLanguage(p model.Platform) string {
	switch p {
	case model.Android:
		return "Android/Kotlin"
	case model.IOS:
		return "iOS/Swift"
	default:
		return "Unknown"
	}
}
