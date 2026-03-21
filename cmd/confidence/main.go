package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/timusus/test-confidence/internal/calibrate"
	"github.com/timusus/test-confidence/internal/config"
	"github.com/timusus/test-confidence/internal/coverage"
	"github.com/timusus/test-confidence/internal/model"
	"github.com/timusus/test-confidence/internal/report"
	"github.com/timusus/test-confidence/internal/scan"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "confidence",
		Short: "Analyze test suite quality through static analysis",
	}

	scanCmd := &cobra.Command{
		Use:   "scan <path>",
		Short: "Scan a directory for test quality signals",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			jsonFlag, _ := cmd.Flags().GetBool("json")
			htmlFlag, _ := cmd.Flags().GetBool("html")
			verbose, _ := cmd.Flags().GetBool("verbose")
			configPath, _ := cmd.Flags().GetString("config")
			comparePath, _ := cmd.Flags().GetString("compare")
			compareRef, _ := cmd.Flags().GetString("compare-ref")

			// Load config
			if configPath == "" {
				configPath = filepath.Join(args[0], ".confidence.yaml")
			}
			cfg := config.LoadConfig(configPath)

			// Platform override
			var platformOverride *model.Platform
			if p, _ := cmd.Flags().GetString("platform"); p != "" {
				switch p {
				case "android":
					v := model.Android
					platformOverride = &v
				case "ios":
					v := model.IOS
					platformOverride = &v
				default:
					return fmt.Errorf("unknown platform: %q (use android or ios)", p)
				}
			}

			// Parse coverage report if provided
			var coverageReport *coverage.CoverageReport
			if coveragePath, _ := cmd.Flags().GetString("coverage"); coveragePath != "" {
				cr, err := coverage.ParseCoverageFile(coveragePath)
				if err != nil {
					return fmt.Errorf("parsing coverage report: %w", err)
				}
				fmt.Fprintf(os.Stderr, "Loaded coverage data: %d files, %.0f%% overall line coverage\n",
					len(cr.Files), cr.OverallLineCoverage()*100)
				coverageReport = cr
			}

			// Compare-since mode: resolve a time duration to a git ref
			compareSince, _ := cmd.Flags().GetString("since")
			if compareSince != "" {
				ref, err := resolveTimeSince(args[0], compareSince)
				if err != nil {
					return err
				}
				compareRef = ref
			}

			// Compare-ref mode: scan a git worktree at the given ref and compare
			if compareRef != "" {
				return runCompareRef(args[0], compareRef, cfg, platformOverride)
			}

			// Run scan
			output, err := scan.Scan(args[0], cfg, platformOverride)
			if err != nil {
				return err
			}

			// Attach coverage data if available
			if coverageReport != nil {
				output.Coverage = coverageReport
				// Enrich surfaces with coverage data
				if output.SurfaceAnalysis != nil {
					scan.EnrichSurfacesWithCoverage(output.SurfaceAnalysis, coverageReport)
				}
			}

			// Comparison mode
			if comparePath != "" {
				baseline, err := report.LoadBaseline(comparePath)
				if err != nil {
					return err
				}
				current := report.SnapshotFromCurrent(output.Result, output.SurfaceAnalysis)
				return report.WriteComparisonReport(os.Stdout, baseline, current)
			}

			// Report
			if htmlFlag {
				// Build historical trend data by scanning at monthly intervals
				trendMonths, _ := cmd.Flags().GetInt("trend-months")
				if trendMonths <= 0 {
					trendMonths = 6
				}
				var history []report.Snapshot
				history = buildHistoricalTrend(args[0], cfg, platformOverride, trendMonths)
				// Add current as the latest point
				current := report.SnapshotFromCurrent(output.Result, output.SurfaceAnalysis)
				current.Label = "Now"
				history = append(history, *current)
				// Build cost data for HTML report
				var htmlCostData *report.HTMLCostData
				if output.TotalUnnecessaryChanges > 0 {
					var costly []report.HTMLCostlyTest
					limit := 8
					if len(output.CostlyTests) < limit {
						limit = len(output.CostlyTests)
					}
					for _, ct := range output.CostlyTests[:limit] {
						name := ct.TestFile
						parts := strings.Split(name, "/")
						if len(parts) > 3 {
							name = strings.Join(parts[len(parts)-3:], "/")
						}
						costly = append(costly, report.HTMLCostlyTest{
							Name: name, UnnecessaryChanges: ct.UnnecessaryChanges,
							CoChangeRate: int(ct.CoChangeRate * 100),
						})
					}
					commits := 0
					if output.GitAnalysis != nil {
						commits = output.GitAnalysis.AnalyzedCommits
					}
					htmlCostData = &report.HTMLCostData{
						CostlyTests: costly, TotalUnnecessaryChanges: output.TotalUnnecessaryChanges,
						SetupCoupledCount: len(output.SetupCoupled), AnalyzedCommits: commits,
					}
				}
				// Build risk hotspots for HTML
				var htmlRiskHotspots []report.RiskHotspot
				for _, rh := range output.RiskHotspots {
					htmlRiskHotspots = append(htmlRiskHotspots, report.RiskHotspot{
						ProductionFile: rh.ProductionFile, TestFile: rh.TestFile,
						Commits: rh.Commits, RecentCommits: rh.RecentCommits,
						CouplingScore: rh.CouplingScore, VerifyOnly: rh.VerifyOnly,
						SetupRatio: rh.SetupRatio, ZeroAssertion: rh.ZeroAssertion,
						Tautologies: rh.Tautologies, RiskScore: rh.RiskScore,
						FixHint: rh.FixHint, CostHint: rh.CostHint,
					})
				}
				jsonData, err := report.MarshalJSONReport(report.HTMLReportInput{
					Result:          output.Result,
					GitAnalysis:     output.GitAnalysis,
					SurfaceAnalysis: output.SurfaceAnalysis,
					RiskHotspots:    htmlRiskHotspots,
					History:         history,
					CostData:        htmlCostData,
					Coverage:        coverageReport,
				})
				if err != nil {
					return fmt.Errorf("marshaling HTML data: %w", err)
				}
				return report.WriteHTMLReport(os.Stdout, jsonData, args[0])
			}
			if jsonFlag {
				var jsonRiskHotspots []report.RiskHotspot
				for _, rh := range output.RiskHotspots {
					jsonRiskHotspots = append(jsonRiskHotspots, report.RiskHotspot{
						ProductionFile: rh.ProductionFile, TestFile: rh.TestFile,
						Commits: rh.Commits, RecentCommits: rh.RecentCommits,
						CouplingScore: rh.CouplingScore, VerifyOnly: rh.VerifyOnly,
						SetupRatio: rh.SetupRatio, ZeroAssertion: rh.ZeroAssertion,
						Tautologies: rh.Tautologies, RiskScore: rh.RiskScore,
						FixHint: rh.FixHint, CostHint: rh.CostHint,
					})
				}
				return report.WriteJSONReport(os.Stdout, output.Result, output.GitAnalysis, output.SurfaceAnalysis, jsonRiskHotspots, coverageReport)
			}
			// Convert scan output types for the reporter
			var termSetupCoupled []report.SetupCoupledFile
			for _, sc := range output.SetupCoupled {
				termSetupCoupled = append(termSetupCoupled, report.SetupCoupledFile{
					TestFile: sc.TestFile, CoChangeRate: sc.CoChangeRate, ProdChanges: sc.ProdChanges,
				})
			}
			var termCostly []report.CostlyTest
			for _, ct := range output.CostlyTests {
				termCostly = append(termCostly, report.CostlyTest{
					TestFile: ct.TestFile, UnnecessaryChanges: ct.UnnecessaryChanges,
					TotalProdChanges: ct.TotalProdChanges, CoChangeRate: ct.CoChangeRate,
				})
			}
			var termRiskHotspots []report.RiskHotspot
			for _, rh := range output.RiskHotspots {
				termRiskHotspots = append(termRiskHotspots, report.RiskHotspot{
					ProductionFile: rh.ProductionFile, TestFile: rh.TestFile,
					Commits: rh.Commits, RecentCommits: rh.RecentCommits,
					CouplingScore: rh.CouplingScore, VerifyOnly: rh.VerifyOnly,
					SetupRatio: rh.SetupRatio, ZeroAssertion: rh.ZeroAssertion,
					Tautologies: rh.Tautologies, RiskScore: rh.RiskScore,
					FixHint: rh.FixHint, CostHint: rh.CostHint,
				})
			}
			return report.WriteTerminalReport(os.Stdout, output.Result, verbose, output.GitAnalysis, output.SurfaceAnalysis, termSetupCoupled, termCostly, output.TotalUnnecessaryChanges, termRiskHotspots, coverageReport)
		},
	}

	scanCmd.Flags().Bool("json", false, "Output JSON instead of terminal")
	scanCmd.Flags().Bool("html", false, "Output self-contained HTML report")
	scanCmd.Flags().Int("trend-months", 6, "Number of months of history for HTML trend charts")
	scanCmd.Flags().Bool("verbose", false, "Show per-file detail")
	scanCmd.Flags().String("config", "", "Path to .confidence.yaml")
	scanCmd.Flags().String("platform", "", "Override platform detection (android|ios)")
	scanCmd.Flags().String("compare", "", "Compare against a baseline JSON file (saved from a previous --json run)")
	scanCmd.Flags().String("compare-ref", "", "Compare current scan against a git ref (branch, tag, or commit)")
	scanCmd.Flags().String("since", "", "Compare current scan against a time ago (e.g., 3months, 6weeks, 1year)")
	scanCmd.Flags().String("coverage", "", "Path to JaCoCo/Kover XML coverage report")

	calibrateCmd := &cobra.Command{
		Use:   "calibrate <path>",
		Short: "Compare scan results against human-annotated ground truth",
		Long: `Runs a normal scan, then compares results against annotations in a
.confidence-calibration.yaml file. Reports per-signal accuracy and per-tier breakdown.

The calibration file should be placed at <path>/.confidence-calibration.yaml
or specified with --calibration-file.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			configPath, _ := cmd.Flags().GetString("config")
			calibrationPath, _ := cmd.Flags().GetString("calibration-file")

			// Load config
			if configPath == "" {
				configPath = filepath.Join(args[0], ".confidence.yaml")
			}
			cfg := config.LoadConfig(configPath)

			// Platform override
			var platformOverride *model.Platform
			if p, _ := cmd.Flags().GetString("platform"); p != "" {
				switch p {
				case "android":
					v := model.Android
					platformOverride = &v
				case "ios":
					v := model.IOS
					platformOverride = &v
				default:
					return fmt.Errorf("unknown platform: %q (use android or ios)", p)
				}
			}

			// Find calibration file
			if calibrationPath == "" {
				calibrationPath = calibrate.FindCalibrationFile(args[0])
			}
			if calibrationPath == "" {
				return fmt.Errorf("no calibration file found — create .confidence-calibration.yaml in %s or use --calibration-file", args[0])
			}

			// Load ground truth
			gt, err := calibrate.LoadGroundTruth(calibrationPath)
			if err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "Loaded %d annotations from %s\n", len(gt.Annotations), calibrationPath)

			// Run scan
			fmt.Fprintf(os.Stderr, "Scanning %s...\n", args[0])
			output, err := scan.Scan(args[0], cfg, platformOverride)
			if err != nil {
				return err
			}

			// Compare
			result := calibrate.Compare(output.Result, gt, args[0])

			// Report
			return calibrate.WriteReport(os.Stdout, result)
		},
	}

	calibrateCmd.Flags().String("config", "", "Path to .confidence.yaml")
	calibrateCmd.Flags().String("platform", "", "Override platform detection (android|ios)")
	calibrateCmd.Flags().String("calibration-file", "", "Path to calibration YAML file (default: <path>/.confidence-calibration.yaml)")

	rootCmd.AddCommand(scanCmd)
	rootCmd.AddCommand(calibrateCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// resolveTimeSince converts a duration string like "3months", "6weeks", "1year"
// to a git commit hash by finding the last commit before that time.
func resolveTimeSince(scanPath, since string) (string, error) {
	absPath, err := filepath.Abs(scanPath)
	if err != nil {
		return "", fmt.Errorf("resolving path: %w", err)
	}

	// Parse the duration string into a git --before format
	gitBefore, human, err := parseTimeSince(since)
	if err != nil {
		return "", err
	}

	// Find the last commit before that date
	out, err := exec.Command("git", "-C", absPath,
		"rev-list", "-1", "--before="+gitBefore, "HEAD").Output()
	if err != nil {
		return "", fmt.Errorf("finding commit from %s ago: %w", human, err)
	}

	ref := strings.TrimSpace(string(out))
	if ref == "" {
		return "", fmt.Errorf("no commits found from %s ago", human)
	}

	// Show what we resolved to
	msgOut, _ := exec.Command("git", "-C", absPath,
		"log", "--format=%h %ad %s", "--date=short", "-1", ref).Output()
	fmt.Fprintf(os.Stderr, "Comparing against %s ago: %s\n", human, strings.TrimSpace(string(msgOut)))

	return ref, nil
}

// parseTimeSince converts "3months", "6weeks", "1year" to a git date string and human label.
func parseTimeSince(since string) (gitDate, human string, err error) {
	// Normalize: strip spaces, lowercase
	s := strings.ToLower(strings.TrimSpace(since))

	// Try to parse as "<number><unit>"
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i == 0 {
		return "", "", fmt.Errorf("invalid duration %q — use format like 3months, 6weeks, 1year", since)
	}

	num := s[:i]
	unit := strings.TrimSpace(s[i:])

	// Normalize unit
	switch unit {
	case "d", "day", "days":
		return num + " days ago", num + " days", nil
	case "w", "week", "weeks":
		return num + " weeks ago", num + " weeks", nil
	case "m", "mo", "month", "months":
		return num + " months ago", num + " months", nil
	case "y", "yr", "year", "years":
		return num + " years ago", num + " years", nil
	default:
		return "", "", fmt.Errorf("unknown time unit %q — use days, weeks, months, or years", unit)
	}
}

// runCompareRef creates a git worktree at the given ref, scans it, scans the
// current path, and writes a comparison report.
func runCompareRef(scanPath, ref string, cfg config.Config, platformOverride *model.Platform) error {
	// 1. Find repo root
	absPath, err := filepath.Abs(scanPath)
	if err != nil {
		return fmt.Errorf("resolving path: %w", err)
	}
	out, err := exec.Command("git", "-C", absPath, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return fmt.Errorf("not a git repository: %w", err)
	}
	repoRoot := strings.TrimSpace(string(out))

	// 2. Determine subdirectory offset (if scan path is a subdir of the repo)
	subdir, err := filepath.Rel(repoRoot, absPath)
	if err != nil {
		subdir = "."
	}

	// 3. Create temp worktree
	tmpDir, err := os.MkdirTemp("", "confidence-compare-*")
	if err != nil {
		return fmt.Errorf("creating temp dir: %w", err)
	}
	defer func() {
		// Clean up worktree
		_ = exec.Command("git", "-C", repoRoot, "worktree", "remove", tmpDir, "--force").Run()
		_ = os.RemoveAll(tmpDir)
	}()

	if out, err := exec.Command("git", "-C", repoRoot, "worktree", "add", tmpDir, ref, "--detach").CombinedOutput(); err != nil {
		return fmt.Errorf("creating worktree at %s: %s: %w", ref, string(out), err)
	}

	// 4. Copy .confidence.yaml to worktree if it exists
	worktreeScanPath := filepath.Join(tmpDir, subdir)
	configSrc := filepath.Join(absPath, ".confidence.yaml")
	configDst := filepath.Join(worktreeScanPath, ".confidence.yaml")
	if data, readErr := os.ReadFile(configSrc); readErr == nil {
		_ = os.WriteFile(configDst, data, 0644)
	}

	// 5. Scan worktree (baseline)
	fmt.Fprintf(os.Stderr, "Scanning %s at %s...\n", ref, worktreeScanPath)
	baselineOutput, err := scan.Scan(worktreeScanPath, cfg, platformOverride)
	if err != nil {
		return fmt.Errorf("scanning worktree (%s): %w", ref, err)
	}
	baselineSnapshot := report.SnapshotFromCurrent(baselineOutput.Result, baselineOutput.SurfaceAnalysis)

	// 6. Scan current
	fmt.Fprintf(os.Stderr, "Scanning current...\n")
	currentOutput, err := scan.Scan(scanPath, cfg, platformOverride)
	if err != nil {
		return fmt.Errorf("scanning current: %w", err)
	}
	currentSnapshot := report.SnapshotFromCurrent(currentOutput.Result, currentOutput.SurfaceAnalysis)

	// 7. Write comparison
	return report.WriteComparisonReport(os.Stdout, baselineSnapshot, currentSnapshot)
}

// buildHistoricalTrend scans the codebase at monthly intervals going back N months.
// Returns snapshots oldest-first for use in trend charts. Silently skips months
// where the worktree can't be created (e.g., subdir didn't exist yet).
func buildHistoricalTrend(scanPath string, cfg config.Config, platformOverride *model.Platform, months int) []report.Snapshot {
	absPath, err := filepath.Abs(scanPath)
	if err != nil {
		return nil
	}
	out, err := exec.Command("git", "-C", absPath, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return nil
	}
	repoRoot := strings.TrimSpace(string(out))

	subdir, err := filepath.Rel(repoRoot, absPath)
	if err != nil {
		subdir = "."
	}

	var snapshots []report.Snapshot

	for i := months; i >= 1; i-- {
		before := fmt.Sprintf("%d months ago", i)

		// Find commit at this point in time
		refOut, err := exec.Command("git", "-C", absPath,
			"rev-list", "-1", "--before="+before, "HEAD").Output()
		if err != nil {
			continue
		}
		ref := strings.TrimSpace(string(refOut))
		if ref == "" {
			continue
		}

		// Create temp worktree
		tmpDir, err := os.MkdirTemp("", "confidence-trend-*")
		if err != nil {
			continue
		}

		wkOut, err := exec.Command("git", "-C", repoRoot, "worktree", "add", tmpDir, ref, "--detach").CombinedOutput()
		if err != nil {
			os.RemoveAll(tmpDir)
			continue
		}
		_ = wkOut

		worktreeScanPath := filepath.Join(tmpDir, subdir)

		// Copy config
		configSrc := filepath.Join(absPath, ".confidence.yaml")
		configDst := filepath.Join(worktreeScanPath, ".confidence.yaml")
		if data, readErr := os.ReadFile(configSrc); readErr == nil {
			_ = os.WriteFile(configDst, data, 0644)
		}

		// Scan (silently skip failures — subdir may not have existed)
		// Suppress stderr during historical scans to avoid contaminating HTML output
		origStderr := os.Stderr
		os.Stderr, _ = os.Open(os.DevNull)
		scanOutput, scanErr := scan.Scan(worktreeScanPath, cfg, platformOverride)
		os.Stderr = origStderr

		// Clean up immediately
		_ = exec.Command("git", "-C", repoRoot, "worktree", "remove", tmpDir, "--force").Run()
		_ = os.RemoveAll(tmpDir)

		if scanErr != nil || scanOutput.Result.TotalTestFiles == 0 {
			continue
		}

		snap := report.SnapshotFromCurrent(scanOutput.Result, scanOutput.SurfaceAnalysis)
		snap.Label = fmt.Sprintf("-%dmo", i)
		snapshots = append(snapshots, *snap)
	}

	return snapshots
}
