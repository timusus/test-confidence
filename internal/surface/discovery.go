package surface

import (
	"bytes"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/timusus/test-confidence/internal/model"
	"github.com/timusus/test-confidence/internal/parse"
)

// SurfaceType classifies the kind of user-facing surface.
type SurfaceType int

const (
	Screen         SurfaceType = iota // @Composable function matching screen patterns (Android) or SwiftUI View (iOS)
	ViewModel                         // class ending in ViewModel
	Activity                          // class ending in Activity
	Fragment                          // class ending in Fragment
	View                              // SwiftUI struct conforming to View protocol
	ViewController                    // UIKit class ending in ViewController
)

func (s SurfaceType) String() string {
	switch s {
	case Screen:
		return "Screen"
	case ViewModel:
		return "ViewModel"
	case Activity:
		return "Activity"
	case Fragment:
		return "Fragment"
	case View:
		return "View"
	case ViewController:
		return "ViewController"
	default:
		return "Unknown"
	}
}

// Surface represents a discovered user-facing surface in the codebase.
type Surface struct {
	Name            string      // e.g., "HomeScreen", "UserViewModel", "MainActivity"
	Type            SurfaceType // Screen, ViewModel, Activity, Fragment
	File            string      // production file path
	Line            int
	Lines           int    // total lines in the file
	FunctionCount   int    // number of functions in the file
	HasTest         bool   // any test file references this surface
	TestFile        string // path to the test file (empty if no test)
	TotalChurn      int    // total git commits touching this file
	RecentChurn     int    // git commits in the last 90 days
	LineCoverage    float64 // 0.0-1.0, from coverage report (-1 if not available)
	BranchCoverage  float64 // 0.0-1.0, from coverage report (-1 if not available)
	HasCoverageData bool    // true if coverage data was found for this file
}

// SurfaceAnalysis holds the results of surface discovery.
type SurfaceAnalysis struct {
	Surfaces        []Surface
	TestedCount     int
	UntestedCount   int
	UntestedByChurn []SurfaceChurn // untested surfaces sorted by git churn (most-changed first)

	// UntestedComplexFiles are non-surface production files with high complexity
	// and git churn but no test pair. These represent business logic (repositories,
	// managers, services, etc.) that the surface scanner doesn't categorize as
	// user-facing but that still carry risk when untested.
	UntestedComplexFiles []UntestedComplexFile
}

// UntestedComplexFile represents a production file with significant complexity
// and churn that has no corresponding test file.
type UntestedComplexFile struct {
	File            string // production file path
	Name            string // class/struct name or filename
	Lines           int
	FunctionCount   int
	TotalChurn      int
	RecentChurn     int
	LineCoverage    float64 // 0.0-1.0, from coverage report
	HasCoverageData bool    // true if coverage data was found for this file
}

// SurfaceChurn pairs a surface with its git churn count.
type SurfaceChurn struct {
	Surface     Surface
	GitChurn    int // total commits touching the production file
	RecentChurn int // commits in last 90 days
}

// lookupChurn finds churn info for a file, trying exact and suffix path matching.
func lookupChurn(filePath string, gitChurn map[string]ChurnInfo) ChurnInfo {
	if info, ok := gitChurn[filePath]; ok {
		return info
	}
	for k, v := range gitChurn {
		if strings.HasSuffix(filePath, k) || strings.HasSuffix(k, filepath.Base(filePath)) {
			return v
		}
	}
	return ChurnInfo{}
}

// ChurnInfo holds git churn data for a file.
type ChurnInfo struct {
	TotalCommits  int
	RecentCommits int // commits in last 90 days
}

// quickFileStats counts lines and functions in a source file using fast text scanning.
// This avoids full tree-sitter parsing for files that aren't surfaces.
func quickFileStats(path string) (lines int, functions int) {
	src, err := os.ReadFile(path)
	if err != nil {
		return 0, 0
	}
	for _, b := range src {
		if b == '\n' {
			lines++
		}
	}
	lines++ // count last line without trailing newline

	// Count function definitions by scanning for common patterns.
	// Kotlin: "fun " at start of line (after whitespace)
	// Swift: "func " at start of line (after whitespace)
	text := string(src)
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "func ") || strings.HasPrefix(trimmed, "fun ") {
			functions++
		}
		// Also count overrides and static/class funcs
		if strings.Contains(trimmed, " func ") || strings.Contains(trimmed, " fun ") {
			if !strings.HasPrefix(trimmed, "//") && !strings.HasPrefix(trimmed, "*") {
				functions++
			}
		}
	}
	return lines, functions
}

// hasTestPair checks if a base name has a corresponding test file.
func hasTestPair(baseName string, testBases map[string]bool) bool {
	return testBases[baseName+"Test.swift"] || testBases[baseName+"Tests.swift"] ||
		testBases[baseName+"Test.kt"] || testBases[baseName+"Tests.kt"] ||
		testBases[baseName+"Spec.swift"] || testBases[baseName+"Spec.kt"]
}

// isLikelyNonLogicFile returns true for files that are unlikely to contain
// testable business logic: protocol definitions, constants, configs, generated code,
// package manifests, extensions with only trivial helpers, etc.
func isLikelyNonLogicFile(name string, path string) bool {
	lname := strings.ToLower(name)
	// Package manifests
	if name == "Package" {
		return true
	}
	// Protocol-only files (just interface definitions, no logic)
	if strings.HasSuffix(name, "Protocol") || strings.HasSuffix(name, "Protocols") {
		return true
	}
	// Constants/keys files
	if strings.HasSuffix(name, "Keys") || strings.HasSuffix(name, "Constants") ||
		strings.HasSuffix(name, "Strings") || strings.HasSuffix(name, "Colors") {
		return true
	}
	// Generated code
	if strings.Contains(lname, "generated") || strings.Contains(lname, ".generated") {
		return true
	}
	// App lifecycle boilerplate
	if name == "AppDelegate" || name == "SceneDelegate" || name == "Application" {
		return true
	}
	// DI/module registration
	if strings.HasSuffix(name, "Module") && !strings.HasSuffix(name, "ViewModel") {
		return true
	}
	// Paths suggesting non-logic files
	if strings.Contains(path, "/Resources/") || strings.Contains(path, "/Generated/") ||
		strings.Contains(path, "/Mock/") || strings.Contains(path, "/Mocks/") {
		return true
	}
	return false
}

// screenSuffixes are function name suffixes that identify composable screens.
var screenSuffixes = []string{"Screen", "Route", "Page"}

// skipDirs are directory names to skip during walking.
var skipDirs = map[string]bool{
	".git":         true,
	".claude":      true,
	".worktrees":   true,
	".gradle":      true,
	"build":        true,
	".build":       true, // Swift Package Manager build artifacts
	"checkouts":    true, // SPM resolved dependencies
	"generated":    true,
	"buildSrc":     true,
	"node_modules": true,
	"Pods":         true, // CocoaPods dependencies
	"Carthage":     true, // Carthage dependencies
	"DerivedData":  true, // Xcode build artifacts
}

// DiscoverSurfaces walks source directories under rootPath and discovers user-facing surfaces.
// For Android: walks src/main/ for .kt files to find Screens, ViewModels, Activities, Fragments.
// For iOS: walks non-test .swift files to find Views, ViewControllers, Screens.
// It cross-references with testFiles to determine which surfaces have tests.
// If gitChurn is provided, untested surfaces are sorted by churn descending.
func DiscoverSurfaces(rootPath string, testFiles []string, gitChurn map[string]ChurnInfo, lang ...model.Language) (*SurfaceAnalysis, error) {
	var surfaces []Surface

	// Track all production files for untested-complex-file detection.
	// Key: file path, Value: basic file stats.
	type prodFileInfo struct {
		path          string
		lines         int
		functionCount int
		isSurface     bool // set to true if this file produced a surface
	}
	var allProdFiles []prodFileInfo

	// Determine language (default to Kotlin for backward compatibility)
	l := model.Kotlin
	if len(lang) > 0 {
		l = lang[0]
	}

	err := filepath.WalkDir(rootPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			if skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}

		rel, relErr := filepath.Rel(rootPath, path)
		if relErr != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)

		if l == model.Swift {
			// iOS: process .swift files NOT in test directories
			if !strings.HasSuffix(d.Name(), ".swift") {
				return nil
			}
			if strings.Contains(rel, "Tests/") || strings.Contains(rel, "Test/") ||
				strings.Contains(rel, "UITests/") || strings.Contains(rel, "Specs/") {
				return nil
			}
			fileSurfaces, parseErr := extractSwiftSurfaces(path)
			if parseErr != nil {
				return nil
			}
			// Track this production file
			lines, funcs := quickFileStats(path)
			pf := prodFileInfo{path: path, lines: lines, functionCount: funcs, isSurface: len(fileSurfaces) > 0}
			allProdFiles = append(allProdFiles, pf)
			surfaces = append(surfaces, fileSurfaces...)
		} else {
			// Android: only process .kt files under src/main/
			if !strings.Contains(rel, "src/main/") || !strings.HasSuffix(d.Name(), ".kt") {
				return nil
			}
			fileSurfaces, parseErr := extractSurfaces(path)
			if parseErr != nil {
				return nil
			}
			// Track this production file
			lines, funcs := quickFileStats(path)
			pf := prodFileInfo{path: path, lines: lines, functionCount: funcs, isSurface: len(fileSurfaces) > 0}
			allProdFiles = append(allProdFiles, pf)
			surfaces = append(surfaces, fileSurfaces...)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	// Deduplicate by name (same surface can appear in multiple module variants)
	surfaces = deduplicateSurfaces(surfaces)

	// Cross-reference with test files
	for i := range surfaces {
		for _, testFile := range testFiles {
			testBase := filepath.Base(testFile)
			if strings.Contains(testBase, surfaces[i].Name) {
				surfaces[i].HasTest = true
				surfaces[i].TestFile = testFile
				break
			}
		}
	}

	// Build analysis
	analysis := &SurfaceAnalysis{
		Surfaces: surfaces,
	}
	for _, s := range surfaces {
		if s.HasTest {
			analysis.TestedCount++
		} else {
			analysis.UntestedCount++
		}
	}

	// Sort untested surfaces by git churn
	if gitChurn != nil {
		var untestedChurn []SurfaceChurn
		for _, s := range surfaces {
			if !s.HasTest {
				info := lookupChurn(s.File, gitChurn)
				untestedChurn = append(untestedChurn, SurfaceChurn{
					Surface:     s,
					GitChurn:    info.TotalCommits,
					RecentChurn: info.RecentCommits,
				})
			}
		}
		sort.Slice(untestedChurn, func(i, j int) bool {
			if untestedChurn[i].RecentChurn != untestedChurn[j].RecentChurn {
				return untestedChurn[i].RecentChurn > untestedChurn[j].RecentChurn
			}
			return untestedChurn[i].GitChurn > untestedChurn[j].GitChurn
		})
		analysis.UntestedByChurn = untestedChurn
	}

	// Detect untested complex files — non-surface production files with high
	// complexity and churn that have no test pair.
	if gitChurn != nil {
		// Build a set of surface file paths for deduplication
		surfaceFiles := make(map[string]bool)
		for _, s := range surfaces {
			surfaceFiles[s.File] = true
		}

		// Build a set of test file base names for pairing
		testBases := make(map[string]bool)
		for _, tf := range testFiles {
			testBases[filepath.Base(tf)] = true
		}

		var complexFiles []UntestedComplexFile
		for _, pf := range allProdFiles {
			if pf.isSurface {
				continue // already reported as a surface
			}

			// Check if this file has a test pair.
			// Also check the base name before "+" for Swift extension files
			// (e.g., "Foo+Bar.swift" is covered by "FooTests.swift").
			base := filepath.Base(pf.path)
			baseName := strings.TrimSuffix(strings.TrimSuffix(base, ".kt"), ".swift")
			hasPair := hasTestPair(baseName, testBases)
			if !hasPair {
				// Check base name before "+" for extension files
				if plusIdx := strings.Index(baseName, "+"); plusIdx > 0 {
					hasPair = hasTestPair(baseName[:plusIdx], testBases)
				}
			}
			if hasPair {
				continue
			}

			// Skip files that are unlikely to contain testable business logic
			if isLikelyNonLogicFile(baseName, pf.path) {
				continue
			}

			// Complexity threshold: must have meaningful logic.
			// Require at least 3 functions (rules out pure config/data files)
			// AND at least 50 lines (rules out trivial wrappers).
			if pf.functionCount < 3 || pf.lines < 50 {
				continue
			}

			// Churn threshold: at least 3 total commits
			info := lookupChurn(pf.path, gitChurn)
			if info.TotalCommits < 3 {
				continue
			}

			complexFiles = append(complexFiles, UntestedComplexFile{
				File:          pf.path,
				Name:          baseName,
				Lines:         pf.lines,
				FunctionCount: pf.functionCount,
				TotalChurn:    info.TotalCommits,
				RecentChurn:   info.RecentCommits,
			})
		}

		// Sort by recent churn descending, then total churn, then complexity
		sort.Slice(complexFiles, func(i, j int) bool {
			if complexFiles[i].RecentChurn != complexFiles[j].RecentChurn {
				return complexFiles[i].RecentChurn > complexFiles[j].RecentChurn
			}
			if complexFiles[i].TotalChurn != complexFiles[j].TotalChurn {
				return complexFiles[i].TotalChurn > complexFiles[j].TotalChurn
			}
			return complexFiles[i].FunctionCount > complexFiles[j].FunctionCount
		})

		analysis.UntestedComplexFiles = complexFiles
	}

	return analysis, nil
}

// extractSurfaces parses a single .kt file and extracts surfaces from it.
func extractSurfaces(path string) ([]Surface, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	tree, err := parse.ParseKotlin(src)
	if err != nil {
		return nil, err
	}
	defer tree.Close()

	root := tree.RootNode()

	// Count file-level stats
	lineCount := bytes.Count(src, []byte("\n")) + 1
	funcCount := countFunctionDeclarations(root)

	var surfaces []Surface
	extractSurfacesFromNode(root, src, path, &surfaces, false)

	// Apply file stats to all surfaces found in this file
	for i := range surfaces {
		surfaces[i].Lines = lineCount
		surfaces[i].FunctionCount = funcCount
	}

	return surfaces, nil
}

func countFunctionDeclarations(node *sitter.Node) int {
	count := 0
	if node.Type() == "function_declaration" {
		count++
	}
	for i := 0; i < int(node.NamedChildCount()); i++ {
		count += countFunctionDeclarations(node.NamedChild(i))
	}
	return count
}

// extractSurfacesFromNode recursively walks the AST to find surfaces.
func extractSurfacesFromNode(node *sitter.Node, src []byte, path string, surfaces *[]Surface, hasComposableAnnotation bool) {
	switch node.Type() {
	case "class_declaration":
		name := extractClassName(node, src)
		if name == "" {
			break
		}

		line := int(node.StartPoint().Row) + 1

		switch {
		case strings.HasSuffix(name, "ViewModel"):
			// Skip trivial delegate ViewModels (very small class bodies)
			classText := string(src[node.StartByte():node.EndByte()])
			if len(classText) < 100 {
				break
			}
			*surfaces = append(*surfaces, Surface{
				Name: name, Type: ViewModel, File: path, Line: line,
			})
		case strings.HasSuffix(name, "Activity"):
			*surfaces = append(*surfaces, Surface{
				Name: name, Type: Activity, File: path, Line: line,
			})
		case strings.HasSuffix(name, "Fragment"):
			*surfaces = append(*surfaces, Surface{
				Name: name, Type: Fragment, File: path, Line: line,
			})
		}

	case "function_declaration":
		// Check if this function has @Composable annotation and a screen-like name.
		// Skip @Preview composables — they are not real screens.
		// Skip private composables — they are internal implementation details.
		if hasComposableAnnotation || functionHasComposableAnnotation(node, src) {
			if functionHasPreviewAnnotation(node, src) {
				return
			}
			if functionHasPrivateModifier(node, src) {
				return
			}
			name := extractFunctionName(node, src)
			if name != "" && isScreenName(name) && !isPreviewName(name) {
				line := int(node.StartPoint().Row) + 1
				*surfaces = append(*surfaces, Surface{
					Name: name, Type: Screen, File: path, Line: line,
				})
			}
		}
		return // Don't recurse into function bodies
	}

	// Recurse into children
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		extractSurfacesFromNode(child, src, path, surfaces, false)
	}
}

// functionHasComposableAnnotation checks if a function_declaration has @Composable.
func functionHasComposableAnnotation(funcNode *sitter.Node, src []byte) bool {
	for i := 0; i < int(funcNode.NamedChildCount()); i++ {
		child := funcNode.NamedChild(i)
		if child.Type() == "modifiers" {
			for j := 0; j < int(child.NamedChildCount()); j++ {
				ann := child.NamedChild(j)
				if ann.Type() == "annotation" {
					text := string(src[ann.StartByte():ann.EndByte()])
					if strings.Contains(text, "Composable") {
						return true
					}
				}
			}
		}
	}
	return false
}

// functionHasPrivateModifier checks if a function_declaration has a private visibility modifier.
func functionHasPrivateModifier(funcNode *sitter.Node, src []byte) bool {
	for i := 0; i < int(funcNode.NamedChildCount()); i++ {
		child := funcNode.NamedChild(i)
		if child.Type() == "modifiers" {
			text := string(src[child.StartByte():child.EndByte()])
			if strings.Contains(text, "private") {
				return true
			}
		}
	}
	return false
}

// extractClassName extracts the class name from a class_declaration node.
func extractClassName(node *sitter.Node, src []byte) string {
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		if child.Type() == "type_identifier" {
			return string(src[child.StartByte():child.EndByte()])
		}
	}
	return ""
}

// extractFunctionName extracts the function name from a function_declaration node.
func extractFunctionName(node *sitter.Node, src []byte) string {
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		if child.Type() == "simple_identifier" {
			return string(src[child.StartByte():child.EndByte()])
		}
	}
	return ""
}

// isScreenName checks if a composable function name matches screen patterns.
func isScreenName(name string) bool {
	for _, suffix := range screenSuffixes {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	return false
}

// deduplicateSurfaces removes duplicate surfaces by name, keeping the first occurrence.
func deduplicateSurfaces(surfaces []Surface) []Surface {
	seen := map[string]bool{}
	var result []Surface
	for _, s := range surfaces {
		if !seen[s.Name] {
			seen[s.Name] = true
			result = append(result, s)
		}
	}
	return result
}

// isPreviewName returns true if the name suggests a preview composable.
func isPreviewName(name string) bool {
	return strings.HasPrefix(name, "Preview_") || strings.HasPrefix(name, "Preview")
}

// functionHasPreviewAnnotation checks if a function has @Preview annotation.
func functionHasPreviewAnnotation(funcNode *sitter.Node, src []byte) bool {
	for i := 0; i < int(funcNode.NamedChildCount()); i++ {
		child := funcNode.NamedChild(i)
		if child.Type() == "modifiers" {
			for j := 0; j < int(child.NamedChildCount()); j++ {
				ann := child.NamedChild(j)
				if ann.Type() == "annotation" {
					text := string(src[ann.StartByte():ann.EndByte()])
					if strings.Contains(text, "Preview") {
						return true
					}
				}
			}
		}
	}
	return false
}

// EnrichSurfaceChurnFromGit runs git log for each untested surface that has
// zero churn data (from the map-based lookup) and populates TotalChurn and
// RecentChurn directly. This is a fallback that provides accurate churn data
// by querying git for each file individually.
func EnrichSurfaceChurnFromGit(rootPath string, analysis *SurfaceAnalysis) {
	if analysis == nil || len(analysis.UntestedByChurn) == 0 {
		return
	}

	// Find git repo root
	out, err := exec.Command("git", "-C", rootPath, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return // not a git repo, skip
	}
	repoRoot := strings.TrimSpace(string(out))

	recentCutoff := time.Now().AddDate(0, 0, -90).Format("2006-01-02")

	changed := false
	for i := range analysis.UntestedByChurn {
		sc := &analysis.UntestedByChurn[i]
		if sc.GitChurn > 0 || sc.RecentChurn > 0 {
			continue // already has churn data from the map lookup
		}

		// Get relative path for git
		relPath, err := filepath.Rel(repoRoot, sc.Surface.File)
		if err != nil {
			continue
		}

		// Total commits
		out, err := exec.Command("git", "-C", repoRoot, "log", "--oneline", "--", relPath).Output()
		if err == nil {
			trimmed := strings.TrimSpace(string(out))
			if trimmed != "" {
				lines := strings.Split(trimmed, "\n")
				sc.GitChurn = len(lines)
				sc.Surface.TotalChurn = len(lines)
			}
		}

		// Recent commits (last 90 days)
		out, err = exec.Command("git", "-C", repoRoot, "log", "--oneline", "--after="+recentCutoff, "--", relPath).Output()
		if err == nil {
			trimmed := strings.TrimSpace(string(out))
			if trimmed != "" {
				lines := strings.Split(trimmed, "\n")
				sc.RecentChurn = len(lines)
				sc.Surface.RecentChurn = len(lines)
			}
		}

		if sc.GitChurn > 0 || sc.RecentChurn > 0 {
			changed = true
		}
	}

	// Re-sort if any churn data was added
	if changed {
		sort.Slice(analysis.UntestedByChurn, func(i, j int) bool {
			if analysis.UntestedByChurn[i].RecentChurn != analysis.UntestedByChurn[j].RecentChurn {
				return analysis.UntestedByChurn[i].RecentChurn > analysis.UntestedByChurn[j].RecentChurn
			}
			return analysis.UntestedByChurn[i].GitChurn > analysis.UntestedByChurn[j].GitChurn
		})
	}
}


// extractSwiftSurfaces parses a single .swift file and extracts iOS surfaces from it.
func extractSwiftSurfaces(path string) ([]Surface, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	tree, err := parse.ParseSwift(src)
	if err != nil {
		return nil, err
	}
	defer tree.Close()

	root := tree.RootNode()

	lineCount := bytes.Count(src, []byte("\n")) + 1
	funcCount := countSwiftFunctionDeclarations(root)

	var surfaces []Surface
	extractSwiftSurfacesFromNode(root, src, path, &surfaces)

	for i := range surfaces {
		surfaces[i].Lines = lineCount
		surfaces[i].FunctionCount = funcCount
	}

	return surfaces, nil
}

// extractSwiftSurfacesFromNode recursively walks a Swift AST to find surfaces.
// Note: Swift tree-sitter represents both structs and classes as class_declaration.
func extractSwiftSurfacesFromNode(node *sitter.Node, src []byte, path string, surfaces *[]Surface) {
	switch node.Type() {
	case "class_declaration":
		name := extractSwiftDeclName(node, src)
		if name == "" {
			break
		}
		line := int(node.StartPoint().Row) + 1

		switch {
		case strings.HasSuffix(name, "ViewController") || inheritsUIViewController(node, src):
			*surfaces = append(*surfaces, Surface{
				Name: name, Type: ViewController, File: path, Line: line,
			})
		case conformsToView(node, src):
			*surfaces = append(*surfaces, Surface{
				Name: name, Type: View, File: path, Line: line,
			})
		case strings.HasSuffix(name, "ViewModel"):
			classText := string(src[node.StartByte():node.EndByte()])
			if len(classText) < 100 {
				break
			}
			*surfaces = append(*surfaces, Surface{
				Name: name, Type: ViewModel, File: path, Line: line,
			})
		case isSwiftScreenName(name):
			*surfaces = append(*surfaces, Surface{
				Name: name, Type: Screen, File: path, Line: line,
			})
		}

	case "protocol_declaration":
		// Skip protocols — they are not concrete surfaces
		return
	}

	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		extractSwiftSurfacesFromNode(child, src, path, surfaces)
	}
}

// extractSwiftDeclName extracts the type name from a Swift class/struct declaration.
func extractSwiftDeclName(node *sitter.Node, src []byte) string {
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		if child.Type() == "type_identifier" || child.Type() == "simple_identifier" {
			return string(src[child.StartByte():child.EndByte()])
		}
	}
	return ""
}

// conformsToView checks if a struct declaration conforms to the SwiftUI View protocol.
// Looks for `: View` in the inheritance clause.
func conformsToView(node *sitter.Node, src []byte) bool {
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		if child.Type() == "inheritance_specifier" {
			text := string(src[child.StartByte():child.EndByte()])
			// Match "View" exactly — avoid matching "ViewModel", "Preview", etc.
			for _, part := range strings.Split(text, ",") {
				trimmed := strings.TrimSpace(part)
				if trimmed == "View" {
					return true
				}
			}
		}
	}
	return false
}

// inheritsUIViewController checks if a class inherits from UIViewController.
func inheritsUIViewController(node *sitter.Node, src []byte) bool {
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		if child.Type() == "inheritance_specifier" {
			text := string(src[child.StartByte():child.EndByte()])
			if strings.Contains(text, "UIViewController") {
				return true
			}
		}
	}
	return false
}

// isSwiftScreenName checks if a Swift type name matches screen patterns.
func isSwiftScreenName(name string) bool {
	return strings.HasSuffix(name, "Screen") || strings.HasSuffix(name, "Page")
}

// countSwiftFunctionDeclarations counts function declarations in a Swift AST.
func countSwiftFunctionDeclarations(node *sitter.Node) int {
	count := 0
	if node.Type() == "function_declaration" {
		count++
	}
	for i := 0; i < int(node.NamedChildCount()); i++ {
		count += countSwiftFunctionDeclarations(node.NamedChild(i))
	}
	return count
}
