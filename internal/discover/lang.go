package discover

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/timusus/test-confidence/internal/model"
)

// DetectLanguage determines the project language by checking for marker files.
// It checks for build.gradle.kts/build.gradle (Kotlin) and Package.swift/*.xcodeproj (Swift).
// Falls back to counting .kt vs .swift extensions (max depth 4, bail at 20 files).
// Defaults to Kotlin if no files found.
func DetectLanguage(root string) model.Language {
	// Check for Kotlin/Android markers.
	if fileExists(filepath.Join(root, "build.gradle.kts")) ||
		fileExists(filepath.Join(root, "build.gradle")) {
		return model.Kotlin
	}

	// Check for Swift/iOS markers.
	if fileExists(filepath.Join(root, "Package.swift")) {
		return model.Swift
	}

	// Check for *.xcodeproj.
	entries, err := os.ReadDir(root)
	if err == nil {
		for _, e := range entries {
			if e.IsDir() && filepath.Ext(e.Name()) == ".xcodeproj" {
				return model.Swift
			}
		}
	}

	// Fallback: walk directory counting .kt vs .swift extensions.
	ktCount := 0
	swiftCount := 0
	const maxDepth = 4
	const bailAt = 20

	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			// Check depth.
			rel, relErr := filepath.Rel(root, path)
			if relErr != nil {
				return nil
			}
			if rel == "." {
				return nil
			}
			depth := len(strings.Split(rel, string(filepath.Separator)))
			if depth > maxDepth {
				return filepath.SkipDir
			}
			return nil
		}
		name := d.Name()
		if strings.HasSuffix(name, ".kt") {
			ktCount++
		} else if strings.HasSuffix(name, ".swift") {
			swiftCount++
		}
		if ktCount+swiftCount >= bailAt {
			return filepath.SkipAll
		}
		return nil
	})

	if swiftCount > ktCount {
		return model.Swift
	}
	return model.Kotlin
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
