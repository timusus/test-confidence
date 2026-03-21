package analyze

import (
	"strings"

	"github.com/timusus/test-confidence/internal/model"
)

// snapshotImports identifies snapshot testing frameworks.
// Priority cascade: snapshot is checked first (highest priority).
var snapshotImports = []string{
	"app.cash.paparazzi",       // Paparazzi
	"com.github.takahirom",     // Roborazzi
	"io.github.takahirom",      // Roborazzi (alternate)
	"sergio.sastre",            // Android Screenshot Testing (Sergio Sastre)
	"SnapshotTesting",          // pointfreeco/swift-snapshot-testing
	"PreviewScreenshot",        // Compose Preview Screenshot Testing
	"screenshotTest",           // Compose Preview Screenshot Testing
}

// deviceImports identifies device/emulator test frameworks.
var deviceImports = []string{
	"XCUIApplication",                              // XCUITest
	"androidx.test.core.app.ActivityScenario",       // ActivityScenario
	"androidx.test.rule.ActivityTestRule",            // ActivityTestRule (legacy)
	"androidx.test.ext.junit.rules.ActivityScenarioRule", // ActivityScenarioRule
	"android.app.Instrumentation",                   // raw instrumentation
	"androidx.test.uiautomator",                     // UI Automator
}

// robolectricPatterns identifies Robolectric usage in local tests.
var robolectricPatterns = []string{
	"org.robolectric",
	"RobolectricTestRunner",
	"@Config",
}

// composeTestPatterns identifies Compose test rule usage in local tests.
var composeTestPatterns = []string{
	"ComposeTestRule",
	"ComposeContentTestRule",
	"createComposeRule",
	"createAndroidComposeRule",
}

// roomTestPatterns identifies Room in-memory DB usage in local tests.
var roomTestPatterns = []string{
	"androidx.room",
}

// ClassifyScope determines the test scope for a parsed test file.
// Priority cascade: Snapshot > Device > Local (first match wins).
func ClassifyScope(file *model.ParsedTestFile) model.ScopeInfo {
	info := model.ScopeInfo{Scope: model.ScopeLocal}

	imports := collectImportPaths(file)
	annotations := collectAnnotations(file)

	// 1. Snapshot — highest priority (import-based or file-name-based)
	if matchesAny(imports, snapshotImports) || isSnapshotTestByName(file.Path) {
		info.Scope = model.ScopeSnapshot
		return info
	}

	// 2. Device — path-based or import-based
	if isDeviceTestByPath(file.Path) || matchesAny(imports, deviceImports) {
		info.Scope = model.ScopeDevice
		return info
	}

	// 3. Local — everything else; detect data points
	info.IsRobolectric = matchesAny(imports, robolectricPatterns) || hasRobolectricAnnotation(annotations)
	info.IsComposeTest = matchesAny(imports, composeTestPatterns)
	info.IsRoomTest = matchesAny(imports, roomTestPatterns)

	return info
}

// isSnapshotTestByName checks if the file name indicates a screenshot/snapshot test.
// This catches files that use snapshot testing via wrapper helpers (e.g., captureMultiTheme)
// without directly importing the snapshot framework.
func isSnapshotTestByName(path string) bool {
	base := strings.ToLower(path)
	return strings.Contains(base, "screenshot") || strings.Contains(base, "snapshot")
}

// collectImportPaths extracts all import paths from the file.
func collectImportPaths(file *model.ParsedTestFile) []string {
	paths := make([]string, len(file.Imports))
	for i, imp := range file.Imports {
		paths[i] = imp.Path
	}
	return paths
}

// collectAnnotations extracts all class-level annotations.
func collectAnnotations(file *model.ParsedTestFile) []string {
	var anns []string
	for _, cls := range file.Classes {
		for _, ann := range cls.Annotations {
			anns = append(anns, ann.Name)
		}
	}
	return anns
}

// matchesAny checks if any import path contains any of the patterns.
func matchesAny(imports []string, patterns []string) bool {
	for _, imp := range imports {
		for _, pat := range patterns {
			if strings.Contains(imp, pat) {
				return true
			}
		}
	}
	return false
}

// isDeviceTestByPath checks if the file path indicates a device test.
func isDeviceTestByPath(path string) bool {
	// Android convention: src/androidTest/
	return strings.Contains(path, "androidTest") ||
		strings.Contains(path, "AndroidTest") // sometimes capitalized in paths
}

// hasRobolectricAnnotation checks for @RunWith(RobolectricTestRunner::class) or @Config.
func hasRobolectricAnnotation(annotations []string) bool {
	for _, ann := range annotations {
		if ann == "RunWith" || ann == "Config" {
			return true
		}
	}
	return false
}
