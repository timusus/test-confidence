package git

// GitAnalysis holds all git-derived signals for a scan.
type GitAnalysis struct {
	// Per file-pair co-change data
	FilePairStats []FilePairStat

	// AllFileChurn maps every file touched in the analyzed commits to its commit count.
	// Includes files that don't have test pairs — used for surface coverage churn ranking.
	AllFileChurn    map[string]int // repo-relative path → total commits
	RecentFileChurn map[string]int // repo-relative path → commits in last 90 days

	// Codebase-level summary
	WorkflowType    WorkflowType // Squash, Merge, Rebase, Mixed
	AnalyzedCommits int
	FilteredCommits int    // commits excluded by noise filters
	AnalysisWindow  string // e.g., "6 months" or "523 commits"
}

// FilePairStat holds co-change metrics for a single test/production file pair.
type FilePairStat struct {
	TestFile       string
	ProductionFile string

	// Co-change frequency (Tier 3 -- present as fact)
	ProdChanges   int     // total commits touching production file
	CoChanges     int     // commits touching both prod and test
	CoChangeRatio float64 // CoChanges / ProdChanges

	// Diff-size correlation (Tier 3 -- present with caveats)
	SmallProdChanges        int     // prod changes <SmallDiffThreshold lines
	SmallProdWithTestChange int     // small prod changes where test also changed
	StructuralCouplingRate  float64 // SmallProdWithTestChange / SmallProdChanges

	// Recent changes (last 90 days)
	RecentProdChanges int // commits touching production file in last 90 days

	// Test churn
	TestChanges    int     // total commits touching test file
	TestChurnRatio float64 // TestChanges / ProdChanges (>1 = test changes more than code)
}

// WorkflowType classifies the git workflow used in the repository.
type WorkflowType int

const (
	WorkflowSquash WorkflowType = iota
	WorkflowMerge
	WorkflowRebase
	WorkflowMixed
)

func (w WorkflowType) String() string {
	switch w {
	case WorkflowSquash:
		return "Squash"
	case WorkflowMerge:
		return "Merge"
	case WorkflowRebase:
		return "Rebase"
	case WorkflowMixed:
		return "Mixed"
	default:
		return "Unknown"
	}
}
