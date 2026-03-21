package discover

import (
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/timusus/test-confidence/internal/config"
)

// DiscoveredFile represents a test file found during directory scanning.
type DiscoveredFile struct {
	Path          string
	IsAndroidTest bool // true if under src/androidTest/
}

// FilePair links a test file to its corresponding production file.
type FilePair struct {
	TestFile       string
	ProductionFile string // empty if no match found
}

// FindTestFiles walks the directory tree rooted at root and returns test files
// matching *Test.kt, *Tests.kt, or *Spec.kt under recognized Kotlin source sets
// (src/test/, src/androidTest/, src/commonTest/, src/jvmTest/, src/androidUnitTest/,
// src/nativeTest/, src/iosTest/, src/desktopTest/) or Swift test directories.
// Paths matching any exclude pattern from config are skipped.
// skipDirs are directory names that should never be walked into.
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

// kotlinTestSourceSets lists the Gradle source set directory names that contain tests.
// This covers standard Android, KMP shared, and KMP target-specific source sets.
var kotlinTestSourceSets = []string{
	"src/test/",
	"src/androidTest/",
	"src/commonTest/",
	"src/jvmTest/",
	"src/androidUnitTest/",
	"src/nativeTest/",
	"src/iosTest/",
	"src/desktopTest/",
}

// kotlinMainSourceSets lists the Gradle source set directory names that contain production code.
var kotlinMainSourceSets = []string{
	"src/main/",
	"src/commonMain/",
	"src/jvmMain/",
	"src/androidMain/",
	"src/nativeMain/",
	"src/iosMain/",
	"src/desktopMain/",
}

// isInKotlinTestSourceSet returns true if rel path is under any recognized test source set.
func isInKotlinTestSourceSet(rel string) bool {
	for _, ss := range kotlinTestSourceSets {
		if strings.Contains(rel, ss) {
			return true
		}
	}
	return false
}

// isInKotlinMainSourceSet returns true if rel path is under any recognized main source set.
func isInKotlinMainSourceSet(rel string) bool {
	for _, ss := range kotlinMainSourceSets {
		if strings.Contains(rel, ss) {
			return true
		}
	}
	return false
}

func FindTestFiles(root string, cfg config.Config) ([]DiscoveredFile, error) {
	var files []DiscoveredFile

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip known non-source directories early (before walking into them).
		if d.IsDir() {
			if skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		// Use forward slashes for consistent matching.
		rel = filepath.ToSlash(rel)

		// Check exclude patterns.
		for _, pattern := range cfg.Exclude {
			matched, _ := filepath.Match(pattern, rel)
			if matched {
				return nil
			}
			// Also try matching against each path segment for ** patterns.
			// filepath.Match doesn't support **, so check if any segment matches
			// the non-** part.
			if strings.Contains(pattern, "**") {
				// Convert **/foo/** to a simple contains check on the segment.
				simple := strings.ReplaceAll(pattern, "**/", "")
				simple = strings.ReplaceAll(simple, "/**", "")
				if strings.Contains(rel, simple+"/") || strings.HasSuffix(rel, "/"+simple) {
					return nil
				}
			}
		}

		name := d.Name()

		// Kotlin: must be under a recognized test source set and match *Test.kt/*Tests.kt/*Spec.kt.
		inKotlinTest := isInKotlinTestSourceSet(rel)
		isKotlinTest := inKotlinTest &&
			(strings.HasSuffix(name, "Test.kt") ||
				strings.HasSuffix(name, "Tests.kt") ||
				strings.HasSuffix(name, "Spec.kt"))

		// Swift: must be in a *Tests/ directory and match *Tests.swift/*Test.swift/*Spec.swift.
		isSwiftTest := strings.HasSuffix(name, ".swift") &&
			(strings.HasSuffix(name, "Tests.swift") || strings.HasSuffix(name, "Test.swift") || strings.HasSuffix(name, "Spec.swift")) &&
			(strings.Contains(rel, "Tests/") || strings.Contains(rel, "Test/"))

		if !isKotlinTest && !isSwiftTest {
			return nil
		}

		files = append(files, DiscoveredFile{
			Path:          path,
			IsAndroidTest: strings.Contains(rel, "src/androidTest/"),
		})
		return nil
	})

	return files, err
}

// PairWithProductionFiles attempts to find a matching production file under
// recognized main source sets for each test file by stripping the Test/Tests/Spec suffix.
func PairWithProductionFiles(testFiles []DiscoveredFile, root string) []FilePair {
	// Build an index of production files under main source sets.
	prodFiles := make(map[string]string) // base filename -> full path
	filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if isInKotlinMainSourceSet(rel) && strings.HasSuffix(d.Name(), ".kt") {
			prodFiles[d.Name()] = path
		}
		// Swift production files: any .swift file NOT in a Tests directory.
		if strings.HasSuffix(d.Name(), ".swift") &&
			!strings.Contains(rel, "Tests/") && !strings.Contains(rel, "Test/") {
			prodFiles[d.Name()] = path
		}
		return nil
	})

	pairs := make([]FilePair, 0, len(testFiles))
	for _, tf := range testFiles {
		pair := FilePair{TestFile: tf.Path}

		base := filepath.Base(tf.Path)
		// Strip suffix to get production filename.
		prodName := ""
		switch {
		case strings.HasSuffix(base, "Tests.kt"):
			prodName = strings.TrimSuffix(base, "Tests.kt") + ".kt"
		case strings.HasSuffix(base, "Test.kt"):
			prodName = strings.TrimSuffix(base, "Test.kt") + ".kt"
		case strings.HasSuffix(base, "Spec.kt"):
			prodName = strings.TrimSuffix(base, "Spec.kt") + ".kt"
		case strings.HasSuffix(base, "Tests.swift"):
			prodName = strings.TrimSuffix(base, "Tests.swift") + ".swift"
		case strings.HasSuffix(base, "Test.swift"):
			prodName = strings.TrimSuffix(base, "Test.swift") + ".swift"
		case strings.HasSuffix(base, "Spec.swift"):
			prodName = strings.TrimSuffix(base, "Spec.swift") + ".swift"
		}

		if prodName != "" {
			if prodPath, ok := prodFiles[prodName]; ok {
				pair.ProductionFile = prodPath
			}
		}

		pairs = append(pairs, pair)
	}

	return pairs
}
