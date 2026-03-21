package git

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// FilePair represents a test file and its corresponding production file.
type FilePair struct {
	TestFile       string
	ProductionFile string
}

// AnalysisOptions controls the behavior of the git analysis.
type AnalysisOptions struct {
	MaxCommits         int // max commits to analyze (default 1000)
	MaxChangesetSize   int // ignore commits touching more than N files (default 50)
	SmallDiffThreshold int // lines changed threshold for "small" (default 10)
}

// DefaultAnalysisOptions returns sensible defaults for git analysis.
func DefaultAnalysisOptions() AnalysisOptions {
	return AnalysisOptions{
		MaxCommits:         1000,
		MaxChangesetSize:   50,
		SmallDiffThreshold: 10,
	}
}

// commitInfo holds parsed data from a single git commit.
type commitInfo struct {
	hash  string
	date  string
	files []fileChange
}

// fileChange holds the numstat data for a single file in a commit.
type fileChange struct {
	added   int
	deleted int
	path    string // relative to repo root
}

// Analyze examines git history to compute co-change statistics for the given file pairs.
func Analyze(rootPath string, filePairs []FilePair, opts AnalysisOptions) (*GitAnalysis, error) {
	// Resolve symlinks in rootPath (macOS /var -> /private/var).
	rootPath = resolveSymlinks(rootPath)

	// Verify this is a git repo and get the repo root.
	repoRoot, err := gitRepoRoot(rootPath)
	if err != nil {
		return nil, fmt.Errorf("not a git repository: %w", err)
	}

	// Get commit history with numstat.
	commits, err := getCommits(repoRoot, opts.MaxCommits)
	if err != nil {
		return nil, fmt.Errorf("failed to get git log: %w", err)
	}

	// Filter bulk commits.
	var filtered []commitInfo
	filteredCount := 0
	for _, c := range commits {
		if len(c.files) > opts.MaxChangesetSize {
			filteredCount++
			continue
		}
		filtered = append(filtered, c)
	}

	// Detect workflow type.
	workflowType, err := detectWorkflow(repoRoot)
	if err != nil {
		// Non-fatal: default to Mixed if detection fails.
		workflowType = WorkflowMixed
	}

	// Convert file pairs to repo-relative paths for matching.
	// Resolve symlinks on input paths to match the resolved repo root.
	type relPair struct {
		testRel string
		prodRel string
	}
	relPairs := make([]relPair, len(filePairs))
	for i, fp := range filePairs {
		testPath := resolveSymlinks(fp.TestFile)
		prodPath := resolveSymlinks(fp.ProductionFile)
		testRel, err := filepath.Rel(repoRoot, testPath)
		if err != nil {
			testRel = fp.TestFile
		}
		prodRel, err := filepath.Rel(repoRoot, prodPath)
		if err != nil {
			prodRel = fp.ProductionFile
		}
		relPairs[i] = relPair{testRel: testRel, prodRel: prodRel}
	}

	// Compute cutoff for "recent" changes (90 days ago).
	recentCutoff := time.Now().AddDate(0, 0, -90).Format("2006-01-02")

	// Track churn for every file across all filtered commits.
	allFileChurn := map[string]int{}
	recentFileChurn := map[string]int{}
	for _, c := range filtered {
		isRecent := len(c.date) >= 10 && c.date[:10] >= recentCutoff
		for _, f := range c.files {
			allFileChurn[f.path]++
			if isRecent {
				recentFileChurn[f.path]++
			}
		}
	}

	// Compute per-pair stats.
	stats := make([]FilePairStat, len(filePairs))
	for i, fp := range filePairs {
		rp := relPairs[i]
		stat := FilePairStat{
			TestFile:       fp.TestFile,
			ProductionFile: fp.ProductionFile,
		}

		for _, c := range filtered {
			prodChange, prodLines := findFile(c.files, rp.prodRel)
			testChange, _ := findFile(c.files, rp.testRel)

			isRecent := len(c.date) >= 10 && c.date[:10] >= recentCutoff

			if prodChange {
				stat.ProdChanges++
				if isRecent {
					stat.RecentProdChanges++
				}
				if testChange {
					stat.CoChanges++
				}
				if prodLines < opts.SmallDiffThreshold {
					stat.SmallProdChanges++
					if testChange {
						stat.SmallProdWithTestChange++
					}
				}
			}
			if testChange {
				stat.TestChanges++
			}
		}

		// Compute ratios.
		if stat.ProdChanges > 0 {
			stat.CoChangeRatio = float64(stat.CoChanges) / float64(stat.ProdChanges)
			stat.TestChurnRatio = float64(stat.TestChanges) / float64(stat.ProdChanges)
		}
		if stat.SmallProdChanges > 0 {
			stat.StructuralCouplingRate = float64(stat.SmallProdWithTestChange) / float64(stat.SmallProdChanges)
		}

		stats[i] = stat
	}

	// Build analysis window description.
	window := fmt.Sprintf("%d commits", len(filtered))
	if len(filtered) > 0 {
		oldest := filtered[len(filtered)-1].date
		if len(oldest) >= 10 {
			window = fmt.Sprintf("%d commits (since %s)", len(filtered), oldest[:10])
		}
	}

	return &GitAnalysis{
		FilePairStats:   stats,
		AllFileChurn:    allFileChurn,
		RecentFileChurn: recentFileChurn,
		WorkflowType:    workflowType,
		AnalyzedCommits: len(filtered),
		FilteredCommits: filteredCount,
		AnalysisWindow:  window,
	}, nil
}

// gitRepoRoot returns the root directory of the git repository containing rootPath.
// It resolves symlinks to ensure consistent path matching (macOS /var -> /private/var).
func gitRepoRoot(rootPath string) (string, error) {
	cmd := exec.Command("git", "-C", rootPath, "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	root := strings.TrimSpace(string(out))
	// Resolve symlinks for consistent path comparison.
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		return root, nil
	}
	return resolved, nil
}

// getCommits parses git log --numstat output into commit structs.
func getCommits(repoRoot string, maxCommits int) ([]commitInfo, error) {
	cmd := exec.Command("git", "-C", repoRoot, "log",
		fmt.Sprintf("--format=%%H %%aI"),
		"--numstat",
		"--no-merges",
		"-n", strconv.Itoa(maxCommits),
	)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	return parseNumstatLog(string(out)), nil
}

// parseNumstatLog parses the output of git log --format="%H %aI" --numstat.
//
// Format:
//
//	<hash> <date>
//
//	<added>\t<deleted>\t<filepath>
//	<added>\t<deleted>\t<filepath>
//
//	<hash> <date>
//	...
func parseNumstatLog(output string) []commitInfo {
	var commits []commitInfo
	lines := strings.Split(output, "\n")

	var current *commitInfo
	for _, line := range lines {
		line = strings.TrimRight(line, "\r")

		if line == "" {
			continue
		}

		// Try to parse as a commit header: "<hash> <date>"
		// A hash is 40 hex chars, followed by a space and an ISO date.
		if len(line) > 41 && line[40] == ' ' && isHexString(line[:40]) {
			// Save previous commit if any.
			if current != nil {
				commits = append(commits, *current)
			}
			current = &commitInfo{
				hash: line[:40],
				date: line[41:],
			}
			continue
		}

		// Try to parse as a numstat line: "<added>\t<deleted>\t<path>"
		if current != nil {
			parts := strings.SplitN(line, "\t", 3)
			if len(parts) == 3 {
				added, err1 := strconv.Atoi(parts[0])
				deleted, err2 := strconv.Atoi(parts[1])
				if err1 == nil && err2 == nil {
					current.files = append(current.files, fileChange{
						added:   added,
						deleted: deleted,
						path:    parts[2],
					})
				}
				// Binary files show "-" for added/deleted; skip them.
			}
		}
	}

	// Don't forget the last commit.
	if current != nil {
		commits = append(commits, *current)
	}

	return commits
}

// isHexString checks if s consists entirely of hexadecimal characters.
func isHexString(s string) bool {
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return len(s) > 0
}

// findFile checks if a file path appears in the commit's file changes.
// Returns whether it was found and the total lines changed (added + deleted).
func findFile(files []fileChange, relPath string) (bool, int) {
	for _, f := range files {
		if f.path == relPath {
			return true, f.added + f.deleted
		}
	}
	return false, 0
}

// detectWorkflow determines the git workflow type by examining merge commit patterns.
func detectWorkflow(repoRoot string) (WorkflowType, error) {
	// Check for merge commits.
	cmd := exec.Command("git", "-C", repoRoot, "log", "--oneline", "--merges", "-n", "20")
	out, err := cmd.Output()
	if err != nil {
		return WorkflowMixed, err
	}

	mergeOutput := strings.TrimSpace(string(out))
	if mergeOutput != "" {
		// Has merge commits.
		mergeCount := len(strings.Split(mergeOutput, "\n"))
		if mergeCount >= 10 {
			return WorkflowMerge, nil
		}
		return WorkflowMixed, nil
	}

	// No merge commits. Check if --first-parent gives fewer commits than full log.
	fpCmd := exec.Command("git", "-C", repoRoot, "log", "--oneline", "--first-parent", "-n", "50")
	fpOut, err := fpCmd.Output()
	if err != nil {
		return WorkflowRebase, nil
	}

	fullCmd := exec.Command("git", "-C", repoRoot, "log", "--oneline", "-n", "50")
	fullOut, err := fullCmd.Output()
	if err != nil {
		return WorkflowRebase, nil
	}

	fpCount := countLines(string(fpOut))
	fullCount := countLines(string(fullOut))

	if fullCount > 0 && fpCount < fullCount {
		return WorkflowSquash, nil
	}

	return WorkflowRebase, nil
}

// resolveSymlinks resolves symlinks in a path, returning the original if resolution fails.
func resolveSymlinks(path string) string {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		// Path may not exist yet; try resolving the parent directory.
		dir := filepath.Dir(path)
		resolvedDir, err := filepath.EvalSymlinks(dir)
		if err != nil {
			return path
		}
		return filepath.Join(resolvedDir, filepath.Base(path))
	}
	return resolved
}

// countLines counts non-empty lines in a string.
func countLines(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	return len(strings.Split(s, "\n"))
}
