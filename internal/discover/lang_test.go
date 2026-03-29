package discover

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/timusus/test-confidence/internal/model"
)

func TestDetectLanguage_AndroidMarker(t *testing.T) {
	lang := DetectLanguage("../../testdata/projects/android-simple")
	if lang != model.Kotlin {
		t.Errorf("expected Kotlin, got %v", lang)
	}
}

func TestDetectLanguage_FallbackToExtensions(t *testing.T) {
	dir := t.TempDir()
	// Create a few .swift files with no markers
	for _, name := range []string{"View.swift", "ViewModel.swift", "Controller.swift"} {
		os.WriteFile(filepath.Join(dir, name), []byte(""), 0644)
	}
	lang := DetectLanguage(dir)
	if lang != model.Swift {
		t.Errorf("expected Swift from extension fallback, got %v", lang)
	}
}

func TestDetectLanguage_NoFilesDefaultsToKotlin(t *testing.T) {
	dir := t.TempDir()
	lang := DetectLanguage(dir)
	if lang != model.Kotlin {
		t.Errorf("expected Kotlin default for empty dir, got %v", lang)
	}
}

func TestDetectLanguage_XcodeprojMarker(t *testing.T) {
	dir := t.TempDir()
	// Create a *.xcodeproj directory
	os.MkdirAll(filepath.Join(dir, "MyApp.xcodeproj"), 0755)
	lang := DetectLanguage(dir)
	if lang != model.Swift {
		t.Errorf("expected Swift for xcodeproj marker, got %v", lang)
	}
}

func TestDetectLanguage_PackageSwiftMarker(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "Package.swift"), []byte(""), 0644)
	lang := DetectLanguage(dir)
	if lang != model.Swift {
		t.Errorf("expected Swift for Package.swift marker, got %v", lang)
	}
}
