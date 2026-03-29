package scan

import (
	"fmt"
	"math"
	"sort"
	"os"
	"runtime"
	"sync"

	"github.com/timusus/test-confidence/internal/analyze"
	"github.com/timusus/test-confidence/internal/config"
	"github.com/timusus/test-confidence/internal/coverage"
	"github.com/timusus/test-confidence/internal/discover"
	gitpkg "github.com/timusus/test-confidence/internal/git"
	"github.com/timusus/test-confidence/internal/model"
	"github.com/timusus/test-confidence/internal/parse"
	"github.com/timusus/test-confidence/internal/surface"
)

// SetupCoupledFile is a test file classified as behavioral (output assertions, no verify)
// but with high git co-change — indicating coupling through mock setup, not assertions.
type SetupCoupledFile struct {
	TestFile      string
	CoChangeRate  float64
	ProdChanges   int
	StructuralRate float64
}

// CostlyTest is a test file ranked by observed cost — how many times it changed
// due to small production tweaks (likely refactoring, not behavior changes).
type CostlyTest struct {
	TestFile        string
	UnnecessaryChanges int // test changes from small prod changes
	TotalProdChanges   int
	CoChangeRate       float64
}

// ScanOutput holds both static analysis results and optional git analysis.
type ScanOutput struct {
	Result          *model.ScanResult
	GitAnalysis     *gitpkg.GitAnalysis         // nil if git analysis was skipped or failed
	SurfaceAnalysis *surface.SurfaceAnalysis     // nil if surface discovery found nothing
	SetupCoupled    []SetupCoupledFile           // behavioral files with high co-change
	CostlyTests     []CostlyTest                // test files ranked by observed maintenance cost
	TotalUnnecessaryChanges int                  // sum of all unnecessary test changes
	RiskHotspots    []RiskHotspot              // high-churn files with fragile tests
	Coverage        *coverage.CoverageReport   // nil if no coverage report provided
}

// Scan orchestrates the full analysis pipeline: discovery, parsing, and analysis.
// If langOverride is non-nil, it is used instead of auto-detection.
func Scan(path string, cfg config.Config, langOverride *model.Language) (*ScanOutput, error) {
	// 1. Detect language
	lang := discover.DetectLanguage(path)
	if langOverride != nil {
		lang = *langOverride
	}

	// 1b. Apply language-specific defaults for placement patterns
	cfg.ApplyLanguageDefaults(lang)

	// 3. Find test files
	testFiles, err := discover.FindTestFiles(path, cfg)
	if err != nil {
		return nil, fmt.Errorf("finding test files: %w", err)
	}

	// 4. Pair with production files
	pairs := discover.PairWithProductionFiles(testFiles, path)

	// 5. Build project context (non-fatal if it fails)
	ctx, err := discover.BuildProjectContext(path, cfg)
	if err != nil {
		ctx = &model.ProjectContext{
			Language:        lang,
			DIBoundaryTypes: map[string]bool{},
			FakeTypes:       map[string]bool{},
			TypePackages:    map[string]string{},
		}
	}
	ctx.Language = lang

	// 6. Parse and analyze all files concurrently
	type fileResult struct {
		analysis model.FileAnalysis
		err      error
	}

	results := make([]fileResult, len(pairs))
	var wg sync.WaitGroup
	sem := make(chan struct{}, runtime.GOMAXPROCS(0))

	for i, pair := range pairs {
		wg.Add(1)
		go func(idx int, p discover.FilePair) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			analysis, err := analyzeFile(p, lang, ctx, cfg)
			results[idx] = fileResult{analysis: analysis, err: err}
		}(i, pair)
	}
	wg.Wait()

	// 7. Aggregate results
	scanResult := &model.ScanResult{
		Path:     path,
		Language: lang,
	}

	for i, r := range results {
		if r.err != nil {
			scanResult.UnparseableFiles++
			fmt.Fprintf(os.Stderr, "warning: skipping %s: %v\n", pairs[i].TestFile, r.err)
			continue
		}
		// Skip files with 0 test methods (abstract base classes, helper files).
		methodCount := 0
		for _, cls := range r.analysis.File.Classes {
			methodCount += len(cls.Methods)
		}
		if methodCount == 0 {
			continue
		}
		scanResult.FileResults = append(scanResult.FileResults, r.analysis)
		scanResult.TotalTestFiles++
		scanResult.TotalTestMethods += methodCount
	}

	// 7b. Aggregate scope distribution
	for _, fa := range scanResult.FileResults {
		switch fa.Scope.Scope {
		case model.ScopeLocal:
			scanResult.ScopeDistribution.Local++
		case model.ScopeDevice:
			scanResult.ScopeDistribution.Device++
		case model.ScopeSnapshot:
			scanResult.ScopeDistribution.Snapshot++
		}
	}

	// 7b2. Aggregate assertion strength distribution across all files
	scanResult.AggregateStrength = make(map[model.Strength]int)
	for _, fa := range scanResult.FileResults {
		for strength, count := range fa.Assertions.StrengthDistribution {
			scanResult.AggregateStrength[strength] += count
		}
	}

	// 7c. Auto-classify remaining unknown placements using codebase-wide usage consensus
	autoClassifyByUsageConsensus(scanResult)

	// 7c. Compute per-file structural coupling scores.
	// This runs after placement classification is finalized so the internal mock
	// ratio signal uses the best available placement data.
	for i := range scanResult.FileResults {
		input := analyze.BuildScoreInput(scanResult.FileResults[i])
		scanResult.FileResults[i].StructuralCouplingScore = analyze.ComputeStructuralCouplingScore(input)
	}

	// 8. Git analysis (non-fatal — skip if not a git repo or on error)
	var gitAnalysis *gitpkg.GitAnalysis
	gitPairs := make([]gitpkg.FilePair, 0, len(pairs))
	for _, p := range pairs {
		if p.ProductionFile != "" {
			gitPairs = append(gitPairs, gitpkg.FilePair{
				TestFile:       p.TestFile,
				ProductionFile: p.ProductionFile,
			})
		}
	}
	if len(gitPairs) > 0 {
		ga, err := gitpkg.Analyze(path, gitPairs, gitpkg.DefaultAnalysisOptions())
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: git analysis skipped: %v\n", err)
		} else {
			gitAnalysis = ga
		}
	}

	// 9. Surface discovery (non-fatal)
	testPaths := make([]string, 0, len(pairs))
	for _, p := range pairs {
		testPaths = append(testPaths, p.TestFile)
	}

	gitChurn := map[string]surface.ChurnInfo{}
	if gitAnalysis != nil {
		for path, count := range gitAnalysis.AllFileChurn {
			info := gitChurn[path]
			info.TotalCommits = count
			gitChurn[path] = info
		}
		for path, count := range gitAnalysis.RecentFileChurn {
			info := gitChurn[path]
			info.RecentCommits = count
			gitChurn[path] = info
		}
	}

	var surfaceAnalysis *surface.SurfaceAnalysis
	sa, surfErr := surface.DiscoverSurfaces(path, testPaths, gitChurn, lang)
	if surfErr != nil {
		fmt.Fprintf(os.Stderr, "warning: surface discovery skipped: %v\n", surfErr)
	} else if len(sa.Surfaces) > 0 {
		// Enrich untested surfaces that didn't get churn data from the map
		// by querying git log directly for each file.
		surface.EnrichSurfaceChurnFromGit(path, sa)
		surfaceAnalysis = sa
	}

	// 10. Cross-reference: find behavioral files with high co-change (setup coupling)
	var setupCoupled []SetupCoupledFile
	if gitAnalysis != nil {
		// Build a set of behavioral test file paths (no structural indicators)
		behavioralFiles := map[string]bool{}
		for _, fa := range scanResult.FileResults {
			if len(fa.Structural) == 0 && fa.Assertions.TotalAssertions > 0 {
				behavioralFiles[fa.File.Path] = true
			}
		}

		for _, fp := range gitAnalysis.FilePairStats {
			if fp.ProdChanges < 5 || fp.CoChangeRatio < 0.6 {
				continue
			}
			if behavioralFiles[fp.TestFile] {
				setupCoupled = append(setupCoupled, SetupCoupledFile{
					TestFile:       fp.TestFile,
					CoChangeRate:   fp.CoChangeRatio,
					ProdChanges:    fp.ProdChanges,
					StructuralRate: fp.StructuralCouplingRate,
				})
			}
		}
	}

	// 11. Compute observed cost — test files ranked by unnecessary changes
	var costlyTests []CostlyTest
	totalUnnecessary := 0
	if gitAnalysis != nil {
		for _, fp := range gitAnalysis.FilePairStats {
			if fp.SmallProdWithTestChange > 0 {
				costlyTests = append(costlyTests, CostlyTest{
					TestFile:           fp.TestFile,
					UnnecessaryChanges: fp.SmallProdWithTestChange,
					TotalProdChanges:   fp.ProdChanges,
					CoChangeRate:       fp.CoChangeRatio,
				})
				totalUnnecessary += fp.SmallProdWithTestChange
			}
		}
		// Sort by cost descending
		for i := 0; i < len(costlyTests); i++ {
			for j := i + 1; j < len(costlyTests); j++ {
				if costlyTests[j].UnnecessaryChanges > costlyTests[i].UnnecessaryChanges {
					costlyTests[i], costlyTests[j] = costlyTests[j], costlyTests[i]
				}
			}
		}
	}

	// 12. Risk hotspots — cross-reference churn with test quality problems
	riskHotspots := computeRiskHotspots(scanResult, gitAnalysis, pairs)

	return &ScanOutput{
		Result:          scanResult,
		GitAnalysis:     gitAnalysis,
		SurfaceAnalysis: surfaceAnalysis,
		SetupCoupled:            setupCoupled,
		CostlyTests:             costlyTests,
		TotalUnnecessaryChanges: totalUnnecessary,
		RiskHotspots:            riskHotspots,
	}, nil
}

// analyzeFile parses a test file (and optionally its production counterpart) and
// runs all analyzers against it. The returned FileAnalysis retains the
// ParsedTestFile including its tree-sitter Tree. We intentionally do NOT call
// Close() here because FileAnalysis holds references to the parsed data. For a
// CLI tool the trees are freed when the process exits.
func analyzeFile(pair discover.FilePair, lang model.Language, ctx *model.ProjectContext, cfg config.Config) (model.FileAnalysis, error) {
	src, err := os.ReadFile(pair.TestFile)
	if err != nil {
		return model.FileAnalysis{}, err
	}

	parsed, err := parse.ParseTestFile(pair.TestFile, src, lang)
	if err != nil {
		return model.FileAnalysis{}, err
	}

	// Parse production file for tautology analysis (optional)
	var prodFile *model.ParsedProductionFile
	if pair.ProductionFile != "" {
		prodSrc, readErr := os.ReadFile(pair.ProductionFile)
		if readErr == nil {
			pf, parseErr := parse.ParseProductionFile(pair.ProductionFile, prodSrc, lang)
			if parseErr == nil {
				prodFile = pf
				// Don't close prodFile — tautology analyzer needs the tree alive.
				// It will be GC'd when ScanResult goes out of scope.
			}
		}
	}

	// Run all analyzers
	antiPatterns := analyze.AnalyzeAntiPatterns(parsed, cfg)
	assertions := analyze.AnalyzeAssertions(parsed)
	structural := analyze.AnalyzeStructural(parsed)
	tautologies := analyze.AnalyzeTautology(parsed, prodFile)
	doubles := analyze.AnalyzeDoubles(parsed)
	setupComplexity := analyze.AnalyzeSetupComplexity(parsed, assertions)

	// Enrich doubles with placement classification
	for i := range doubles.Doubles {
		placement, signals := analyze.ClassifyPlacement(doubles.Doubles[i], ctx, cfg)
		doubles.Doubles[i].Placement = placement
		doubles.Doubles[i].PlacementSignals = signals
	}

	// Classify test scope
	scopeInfo := analyze.ClassifyScope(parsed)

	return model.FileAnalysis{
		File:         *parsed,
		AntiPatterns: antiPatterns,
		Assertions:   assertions,
		Structural:   structural,
		Tautologies:  tautologies,
		Doubles:         doubles,
		Scope:           scopeInfo,
		SetupComplexity: setupComplexity,
	}, nil
}

// autoClassifyByUsageConsensus re-classifies doubles with UnknownPlacement
// when codebase-wide usage data shows strong consensus. If a type appears in
// multiple test files and >80% of instances share the same usage pattern,
// all unknown instances get that placement.
func autoClassifyByUsageConsensus(result *model.ScanResult) {
	// Build per-type usage map across all files (only for unknown-placement doubles)
	typeUsage := map[string]map[model.Usage]int{} // typeName → usage → count
	for _, fa := range result.FileResults {
		for _, d := range fa.Doubles.Doubles {
			if d.Placement != model.UnknownPlacement || d.Usage == model.FakeUsage {
				continue
			}
			if typeUsage[d.TypeName] == nil {
				typeUsage[d.TypeName] = map[model.Usage]int{}
			}
			typeUsage[d.TypeName][d.Usage]++
		}
	}

	// For types where all instances agree on usage, classify
	for typeName, usages := range typeUsage {
		total := 0
		for _, c := range usages {
			total += c
		}
		if total < 2 {
			continue // not enough data
		}

		setupOnly := usages[model.SetupOnly]
		verification := usages[model.Verification]

		var placement model.Placement
		signal := ""
		if float64(setupOnly)/float64(total) > 0.8 {
			placement = model.Boundary
			signal = fmt.Sprintf("codebase consensus: %d/%d instances are setup-only", setupOnly, total)
		} else if float64(verification)/float64(total) > 0.8 {
			placement = model.Internal
			signal = fmt.Sprintf("codebase consensus: %d/%d instances are verification", verification, total)
		} else {
			continue
		}

		// Apply to all unknown instances of this type
		for i := range result.FileResults {
			for j := range result.FileResults[i].Doubles.Doubles {
				d := &result.FileResults[i].Doubles.Doubles[j]
				if d.TypeName == typeName && d.Placement == model.UnknownPlacement {
					d.Placement = placement
					d.PlacementSignals = append(d.PlacementSignals, model.PlacementSignal{
						Signal: "codebase_consensus",
						Value:  signal,
						Result: placement,
					})
				}
			}
		}
	}
}


// computeRiskHotspots cross-references git churn data with per-file test quality
// signals to find production files that are both high-churn AND have fragile tests.
// Only includes files where churn > median AND the test has quality problems.
func computeRiskHotspots(result *model.ScanResult, gitAnalysis *gitpkg.GitAnalysis, pairs []discover.FilePair) []RiskHotspot {
	if gitAnalysis == nil || len(gitAnalysis.FilePairStats) == 0 {
		return nil
	}

	// Build a map from test file path -> FileAnalysis for quick lookup
	testAnalysis := map[string]*model.FileAnalysis{}
	for i := range result.FileResults {
		testAnalysis[result.FileResults[i].File.Path] = &result.FileResults[i]
	}

	// Build churn data from FilePairStats (which use absolute paths matching FileResults)
	type churnData struct {
		prodFile      string
		commits       int
		recentCommits int
	}
	testChurn := map[string]churnData{} // test file path -> churn data for its prod file
	var churnValues []int
	for _, fp := range gitAnalysis.FilePairStats {
		if fp.ProdChanges > 0 {
			testChurn[fp.TestFile] = churnData{
				prodFile:      fp.ProductionFile,
				commits:       fp.ProdChanges,
				recentCommits: fp.RecentProdChanges,
			}
			churnValues = append(churnValues, fp.ProdChanges)
		}
	}
	if len(churnValues) == 0 {
		return nil
	}
	sort.Ints(churnValues)
	medianChurn := churnValues[len(churnValues)/2]
	// Use at least 3 commits as threshold to avoid noise
	if medianChurn < 3 {
		medianChurn = 3
	}

	var hotspots []RiskHotspot

	for _, fa := range result.FileResults {
		cd, ok := testChurn[fa.File.Path]
		if !ok {
			continue
		}

		commits := cd.commits
		if commits <= medianChurn {
			continue // below median churn — skip
		}

		recentCommits := cd.recentCommits

		// Gather test quality signals
		couplingScore := fa.StructuralCouplingScore

		verifyOnly := 0
		for _, si := range fa.Structural {
			if si.Type == model.VerifyOnlyTest {
				verifyOnly++
			}
		}

		setupRatio := fa.SetupComplexity.Ratio
		zeroAssertion := len(fa.Assertions.ZeroAssertionMethods)
		tautologies := len(fa.Tautologies)

		// Check if this test actually has problems
		hasProblems := couplingScore > 0 || verifyOnly > 0 || setupRatio > 5.0 || zeroAssertion > 0 || tautologies > 0
		if !hasProblems {
			continue
		}

		// Compute risk score: churn factor × quality problem factor
		// Churn factor: log-scaled to avoid extreme outliers dominating
		churnFactor := math.Log2(float64(commits) + 1)

		// Quality problem factor: combine signals
		qualityFactor := 0.0
		if couplingScore > 0 {
			qualityFactor += couplingScore / 100.0 * 60 // coupling is the primary signal, up to 60
		}
		if verifyOnly > 0 {
			qualityFactor += math.Min(float64(verifyOnly)*5, 20) // up to 20
		}
		if setupRatio > 5.0 {
			qualityFactor += math.Min((setupRatio-5)*2, 10) // up to 10
		}
		if tautologies > 0 {
			qualityFactor += math.Min(float64(tautologies)*5, 10) // up to 10
		}

		riskScore := churnFactor * qualityFactor

		// Generate cost and fix hints based on the dominant problem
		costHint, fixHint := riskHints(verifyOnly, setupRatio, tautologies, couplingScore)

		hotspots = append(hotspots, RiskHotspot{
			ProductionFile: cd.prodFile,
			TestFile:       fa.File.Path,
			Commits:        commits,
			RecentCommits:  recentCommits,
			CouplingScore:  couplingScore,
			VerifyOnly:     verifyOnly,
			SetupRatio:     setupRatio,
			ZeroAssertion:  zeroAssertion,
			Tautologies:    tautologies,
			RiskScore:      riskScore,
			FixHint:        fixHint,
			CostHint:       costHint,
		})
	}

	// Sort by risk score descending
	sort.Slice(hotspots, func(i, j int) bool {
		return hotspots[i].RiskScore > hotspots[j].RiskScore
	})

	return hotspots
}

// riskHints generates cost and fix text based on the dominant test quality problem.
func riskHints(verifyOnly int, setupRatio float64, tautologies int, couplingScore float64) (cost, fix string) {
	// Pick the dominant problem for the hint
	if verifyOnly > 2 {
		return "this file changes often and its tests break on refactors",
			"replace verify calls with output assertions on return values"
	}
	if setupRatio > 8.0 {
		return "heavy setup means tests are tightly coupled to implementation",
			"introduce fakes to simplify setup — a FakeRepository replaces 10 every{} lines"
	}
	if tautologies > 0 {
		return "pass-through tests don't catch real bugs in this high-churn file",
			"add meaningful assertions that test actual logic, not just wiring"
	}
	if couplingScore > 40 {
		return "this file changes often and its tests are structurally coupled",
			"reduce mock verification — assert on observable outputs instead"
	}
	return "this file changes often and its tests have quality problems",
		"review test structure — aim for behavioral assertions over interaction verification"
}

// RiskHotspot is a production file with high churn AND fragile tests.
// It combines git churn data with static test quality signals to identify
// the highest-risk areas of the codebase: code that changes often where
// the tests punish refactoring.
type RiskHotspot struct {
	ProductionFile string  // repo-relative path
	TestFile       string  // repo-relative path
	Commits        int     // total commits touching the production file
	RecentCommits  int     // commits in last 90 days
	CouplingScore  float64 // structural coupling score (0-100)
	VerifyOnly     int     // verify-only test methods
	SetupRatio     float64 // setup-to-assertion ratio
	ZeroAssertion  int     // zero-assertion test methods
	Tautologies    int     // pass-through tests
	RiskScore      float64 // composite: churn × quality problems
	FixHint        string  // mechanical fix suggestion
	CostHint       string  // explanation of the cost
}

// EnrichSurfacesWithCoverage adds coverage data to surfaces and untested complex files.
// A surface with 0% coverage from the report is confirmed untested; one the report
// doesn't mention is unknown (may or may not be covered).
func EnrichSurfacesWithCoverage(sa *surface.SurfaceAnalysis, cr *coverage.CoverageReport) {
	if sa == nil || cr == nil {
		return
	}

	// Enrich surfaces
	for i := range sa.Surfaces {
		fc := coverage.LookupCoverage(cr, sa.Surfaces[i].File)
		if fc != nil {
			sa.Surfaces[i].LineCoverage = fc.LineCoverage
			sa.Surfaces[i].BranchCoverage = fc.BranchCoverage
			sa.Surfaces[i].HasCoverageData = true
		}
	}

	// Enrich untested-by-churn list
	for i := range sa.UntestedByChurn {
		fc := coverage.LookupCoverage(cr, sa.UntestedByChurn[i].Surface.File)
		if fc != nil {
			sa.UntestedByChurn[i].Surface.LineCoverage = fc.LineCoverage
			sa.UntestedByChurn[i].Surface.BranchCoverage = fc.BranchCoverage
			sa.UntestedByChurn[i].Surface.HasCoverageData = true
		}
	}

	// Enrich untested complex files
	for i := range sa.UntestedComplexFiles {
		fc := coverage.LookupCoverage(cr, sa.UntestedComplexFiles[i].File)
		if fc != nil {
			sa.UntestedComplexFiles[i].LineCoverage = fc.LineCoverage
			sa.UntestedComplexFiles[i].HasCoverageData = true
		}
	}
}
