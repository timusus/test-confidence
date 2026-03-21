package discover

import (
	"os"
	"path/filepath"

	"github.com/timusus/test-confidence/internal/model"
)

// DetectPlatform determines the project platform by checking for marker files.
// It checks for build.gradle.kts/build.gradle (Android) and Package.swift/*.xcodeproj (iOS).
// Defaults to Android if no markers are found.
func DetectPlatform(root string) model.Platform {
	// Check for Android markers.
	if fileExists(filepath.Join(root, "build.gradle.kts")) ||
		fileExists(filepath.Join(root, "build.gradle")) {
		return model.Android
	}

	// Check for iOS markers.
	if fileExists(filepath.Join(root, "Package.swift")) {
		return model.IOS
	}

	// Check for *.xcodeproj.
	entries, err := os.ReadDir(root)
	if err == nil {
		for _, e := range entries {
			if e.IsDir() && filepath.Ext(e.Name()) == ".xcodeproj" {
				return model.IOS
			}
		}
	}

	return model.Android
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
