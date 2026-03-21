package report

import (
	_ "embed"
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

//go:embed chart.min.js
var chartJS string

//go:embed viewer/shell.html
var shellHTML string

//go:embed viewer/style.css
var viewerCSS string

//go:embed viewer/app.js
var viewerJS string

// HTMLCostData holds observed cost data from the scan engine for the HTML report.
type HTMLCostData struct {
	CostlyTests             []HTMLCostlyTest
	TotalUnnecessaryChanges int
	SetupCoupledCount       int
	AnalyzedCommits         int
}

// HTMLCostlyTest is a test file with its observed maintenance cost.
type HTMLCostlyTest struct {
	Name               string
	UnnecessaryChanges int
	CoChangeRate       int
}

// WriteHTMLReport writes a self-contained HTML report to w.
// jsonData is the full JSON payload (from MarshalJSONReport).
// path is the scan path (used for the page title).
func WriteHTMLReport(w io.Writer, jsonData []byte, path string) error {
	title := shortenProjectPath(path)

	html := shellHTML
	html = strings.Replace(html, "{{PATH}}", title, 1)
	html = strings.Replace(html, "{{CSS}}", viewerCSS, 1)
	html = strings.Replace(html, "{{JSON_DATA}}", string(jsonData), 1)
	html = strings.Replace(html, "{{CHART_JS}}", chartJS, 1)
	html = strings.Replace(html, "{{APP_JS}}", viewerJS, 1)

	_, err := fmt.Fprint(w, html)
	return err
}

// shortenProjectPath extracts a meaningful short name from a full project path.
// /home/user/projects/my-app/main → my-app
// /home/user/projects/my-app/main/mobile/android → my-app
func shortenProjectPath(fullPath string) string {
	parts := strings.Split(filepath.ToSlash(fullPath), "/")
	generic := map[string]bool{"main": true, "android": true, "ios": true, "mobile": true, "app": true, "src": true}

	// Find the last meaningful (non-generic) segment
	for i := len(parts) - 1; i >= 0; i-- {
		if parts[i] != "" && !generic[parts[i]] {
			return parts[i]
		}
	}
	// Fallback: last non-empty segment
	for i := len(parts) - 1; i >= 0; i-- {
		if parts[i] != "" {
			return parts[i]
		}
	}
	return fullPath
}
