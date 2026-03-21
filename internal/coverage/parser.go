package coverage

import (
	"encoding/json"
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
)

// FileCoverage holds coverage data for a single source file.
type FileCoverage struct {
	SourceFile     string  // e.g., "MainViewModel.kt"
	Package        string  // e.g., "com/example/app/feature"
	LineCoverage   float64 // 0.0-1.0
	BranchCoverage float64 // 0.0-1.0
	LinesMissed    int
	LinesCovered   int
	BranchesMissed int
	BranchesCovered int
}

// CoverageReport holds parsed coverage data keyed by source filename.
type CoverageReport struct {
	Files map[string]FileCoverage // key: source filename (e.g., "MainViewModel.kt")
}

// TotalLinesCovered returns the sum of covered lines across all files.
func (r *CoverageReport) TotalLinesCovered() int {
	total := 0
	for _, f := range r.Files {
		total += f.LinesCovered
	}
	return total
}

// TotalLinesMissed returns the sum of missed lines across all files.
func (r *CoverageReport) TotalLinesMissed() int {
	total := 0
	for _, f := range r.Files {
		total += f.LinesMissed
	}
	return total
}

// OverallLineCoverage returns the aggregate line coverage percentage (0.0-1.0).
func (r *CoverageReport) OverallLineCoverage() float64 {
	covered := r.TotalLinesCovered()
	missed := r.TotalLinesMissed()
	total := covered + missed
	if total == 0 {
		return 0
	}
	return float64(covered) / float64(total)
}

// XML structures for JaCoCo/Kover format
type xmlReport struct {
	XMLName  xml.Name     `xml:"report"`
	Packages []xmlPackage `xml:"package"`
}

type xmlPackage struct {
	Name        string          `xml:"name,attr"`
	SourceFiles []xmlSourceFile `xml:"sourcefile"`
}

type xmlSourceFile struct {
	Name     string       `xml:"name,attr"`
	Counters []xmlCounter `xml:"counter"`
}

type xmlCounter struct {
	Type    string `xml:"type,attr"`
	Missed  int    `xml:"missed,attr"`
	Covered int    `xml:"covered,attr"`
}

// ParseJacocoXML parses a JaCoCo/Kover XML coverage report.
func ParseJacocoXML(path string) (*CoverageReport, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var report xmlReport
	if err := xml.Unmarshal(data, &report); err != nil {
		return nil, err
	}

	result := &CoverageReport{
		Files: make(map[string]FileCoverage),
	}

	for _, pkg := range report.Packages {
		for _, sf := range pkg.SourceFiles {
			fc := FileCoverage{
				SourceFile: sf.Name,
				Package:    pkg.Name,
			}

			for _, counter := range sf.Counters {
				switch counter.Type {
				case "LINE":
					fc.LinesMissed = counter.Missed
					fc.LinesCovered = counter.Covered
					total := counter.Missed + counter.Covered
					if total > 0 {
						fc.LineCoverage = float64(counter.Covered) / float64(total)
					}
				case "BRANCH":
					fc.BranchesMissed = counter.Missed
					fc.BranchesCovered = counter.Covered
					total := counter.Missed + counter.Covered
					if total > 0 {
						fc.BranchCoverage = float64(counter.Covered) / float64(total)
					}
				}
			}

			// Key by source filename. If there are duplicates across packages,
			// keep the one with more lines (more meaningful coverage data).
			existing, exists := result.Files[sf.Name]
			if !exists || (fc.LinesCovered+fc.LinesMissed) > (existing.LinesCovered+existing.LinesMissed) {
				result.Files[sf.Name] = fc
			}
		}
	}

	return result, nil
}

// ParseXccovJSON parses Apple's xccov JSON output (from `xcrun xccov view --report --json`).
// The format is: { "targets": [{ "files": [{ "path": "...", "lineCoverage": 0.75, ... }] }] }
func ParseXccovJSON(path string) (*CoverageReport, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var xccov xccovReport
	if err := json.Unmarshal(data, &xccov); err != nil {
		return nil, err
	}

	result := &CoverageReport{
		Files: make(map[string]FileCoverage),
	}

	for _, target := range xccov.Targets {
		for _, file := range target.Files {
			base := filepath.Base(file.Path)
			fc := FileCoverage{
				SourceFile:   base,
				LineCoverage: file.LineCoverage,
				LinesCovered: file.CoveredLines,
				LinesMissed:  file.ExecutableLines - file.CoveredLines,
			}
			// Keep the entry with more data if duplicates
			existing, exists := result.Files[base]
			if !exists || (fc.LinesCovered+fc.LinesMissed) > (existing.LinesCovered+existing.LinesMissed) {
				result.Files[base] = fc
			}
		}
	}

	return result, nil
}

type xccovReport struct {
	Targets []xccovTarget `json:"targets"`
}

type xccovTarget struct {
	Files []xccovFile `json:"files"`
}

type xccovFile struct {
	Path            string  `json:"path"`
	LineCoverage    float64 `json:"lineCoverage"`
	CoveredLines    int     `json:"coveredLines"`
	ExecutableLines int     `json:"executableLines"`
}

// ParseCoverageFile auto-detects the format (JaCoCo XML or xccov JSON) and parses accordingly.
func ParseCoverageFile(path string) (*CoverageReport, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	// Detect format by first non-whitespace character
	trimmed := strings.TrimSpace(string(data))
	if len(trimmed) > 0 && trimmed[0] == '{' {
		return ParseXccovJSON(path)
	}
	return ParseJacocoXML(path)
}

// LookupCoverage matches a production file path to coverage data.
// Tries exact filename match first, then fuzzy match on base filename.
func LookupCoverage(report *CoverageReport, productionFilePath string) *FileCoverage {
	if report == nil {
		return nil
	}

	base := filepath.Base(productionFilePath)

	// Exact filename match
	if fc, ok := report.Files[base]; ok {
		return &fc
	}

	// Fuzzy match: strip extension and try partial matches
	nameNoExt := strings.TrimSuffix(base, filepath.Ext(base))
	for key, fc := range report.Files {
		keyNoExt := strings.TrimSuffix(key, filepath.Ext(key))
		if keyNoExt == nameNoExt {
			return &fc
		}
	}

	return nil
}
