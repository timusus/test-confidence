package surface

import (
	"testing"

	"github.com/timusus/test-confidence/internal/config"
	"github.com/timusus/test-confidence/internal/discover"
	"github.com/timusus/test-confidence/internal/model"
)

func TestDiscoverSurfaces(t *testing.T) {
	root := "../../testdata/projects/android-simple"

	// Get test file paths
	cfg := config.DefaultConfig()
	testFiles, err := discover.FindTestFiles(root, cfg)
	if err != nil {
		t.Fatal(err)
	}
	testPaths := make([]string, len(testFiles))
	for i, f := range testFiles {
		testPaths[i] = f.Path
	}

	result, err := DiscoverSurfaces(root, testPaths, nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Surfaces) == 0 {
		t.Fatal("expected surfaces to be discovered")
	}

	// Should find UserViewModel
	foundVM := false
	for _, s := range result.Surfaces {
		if s.Name == "UserViewModel" {
			foundVM = true
			if s.Type != ViewModel {
				t.Errorf("expected UserViewModel type ViewModel, got %s", s.Type)
			}
			if !s.HasTest {
				t.Error("UserViewModel should have a test (UserViewModelTest.kt exists)")
			}
		}
	}
	if !foundVM {
		t.Error("expected to find UserViewModel surface")
	}

	// Should find HomeScreen (composable)
	foundScreen := false
	for _, s := range result.Surfaces {
		if s.Name == "HomeScreen" {
			foundScreen = true
			if s.Type != Screen {
				t.Errorf("expected HomeScreen type Screen, got %s", s.Type)
			}
			if s.HasTest {
				t.Error("HomeScreen should not have a test (no HomeScreenTest.kt)")
			}
		}
	}
	if !foundScreen {
		t.Error("expected to find HomeScreen surface")
	}

	// Should find SettingsScreen (composable, untested)
	foundSettings := false
	for _, s := range result.Surfaces {
		if s.Name == "SettingsScreen" {
			foundSettings = true
			if s.Type != Screen {
				t.Errorf("expected SettingsScreen type Screen, got %s", s.Type)
			}
			if s.HasTest {
				t.Error("SettingsScreen should not have a test")
			}
		}
	}
	if !foundSettings {
		t.Error("expected to find SettingsScreen surface")
	}

	// Should find MainActivity
	foundActivity := false
	for _, s := range result.Surfaces {
		if s.Name == "MainActivity" {
			foundActivity = true
			if s.Type != Activity {
				t.Errorf("expected MainActivity type Activity, got %s", s.Type)
			}
		}
	}
	if !foundActivity {
		t.Error("expected to find MainActivity surface")
	}

	// Check counts
	if result.TestedCount == 0 {
		t.Error("expected at least one tested surface")
	}
	if result.UntestedCount == 0 {
		t.Error("expected at least one untested surface")
	}
}

func TestDiscoverSurfacesWithChurn(t *testing.T) {
	root := "../../testdata/projects/android-simple"

	cfg := config.DefaultConfig()
	testFiles, err := discover.FindTestFiles(root, cfg)
	if err != nil {
		t.Fatal(err)
	}
	testPaths := make([]string, len(testFiles))
	for i, f := range testFiles {
		testPaths[i] = f.Path
	}

	// Simulate git churn data
	gitChurn := map[string]ChurnInfo{
		"HomeScreen.kt":   {TotalCommits: 12, RecentCommits: 5},
		"MainActivity.kt": {TotalCommits: 3, RecentCommits: 1},
	}

	result, err := DiscoverSurfaces(root, testPaths, gitChurn)
	if err != nil {
		t.Fatal(err)
	}

	if len(result.UntestedByChurn) == 0 {
		t.Fatal("expected untested surfaces sorted by churn")
	}

	// Verify sorted by churn descending
	for i := 1; i < len(result.UntestedByChurn); i++ {
		if result.UntestedByChurn[i].GitChurn > result.UntestedByChurn[i-1].GitChurn {
			t.Error("untested surfaces should be sorted by churn descending")
		}
	}
}

func TestDiscoverSurfacesEmpty(t *testing.T) {
	dir := t.TempDir()

	result, err := DiscoverSurfaces(dir, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Surfaces) != 0 {
		t.Error("expected no surfaces in empty directory")
	}
}

func TestSurfaceTypeString(t *testing.T) {
	tests := []struct {
		st   SurfaceType
		want string
	}{
		{Screen, "Screen"},
		{ViewModel, "ViewModel"},
		{Activity, "Activity"},
		{Fragment, "Fragment"},
	}

	for _, tt := range tests {
		if got := tt.st.String(); got != tt.want {
			t.Errorf("SurfaceType(%d).String() = %q, want %q", tt.st, got, tt.want)
		}
	}
}

func TestDiscoverSwiftSurfaces(t *testing.T) {
	root := "../../testdata/projects/ios-simple"

	// Get test file paths
	cfg := config.DefaultConfig()
	testFiles, err := discover.FindTestFiles(root, cfg)
	if err != nil {
		t.Fatal(err)
	}
	testPaths := make([]string, len(testFiles))
	for i, f := range testFiles {
		testPaths[i] = f.Path
	}

	result, err := DiscoverSurfaces(root, testPaths, nil, model.IOS)
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Surfaces) == 0 {
		t.Fatal("expected surfaces to be discovered")
	}

	// Should find ContentView (SwiftUI View)
	foundContentView := false
	for _, s := range result.Surfaces {
		if s.Name == "ContentView" {
			foundContentView = true
			if s.Type != View {
				t.Errorf("expected ContentView type View, got %s", s.Type)
			}
			if !s.HasTest {
				t.Error("ContentView should have a test (ContentViewTests.swift exists)")
			}
		}
	}
	if !foundContentView {
		t.Error("expected to find ContentView surface")
		for _, s := range result.Surfaces {
			t.Logf("  found: %s (%s) at %s", s.Name, s.Type, s.File)
		}
	}

	// Should find SettingsScreen (SwiftUI View with Screen suffix)
	foundSettings := false
	for _, s := range result.Surfaces {
		if s.Name == "SettingsScreen" {
			foundSettings = true
			// SettingsScreen conforms to View, so it should be View type
			if s.Type != View {
				t.Errorf("expected SettingsScreen type View, got %s", s.Type)
			}
		}
	}
	if !foundSettings {
		t.Error("expected to find SettingsScreen surface")
	}

	// Should find MainViewController
	foundVC := false
	for _, s := range result.Surfaces {
		if s.Name == "MainViewController" {
			foundVC = true
			if s.Type != ViewController {
				t.Errorf("expected MainViewController type ViewController, got %s", s.Type)
			}
		}
	}
	if !foundVC {
		t.Error("expected to find MainViewController surface")
	}

	// Should find ProfileViewModel
	foundVM := false
	for _, s := range result.Surfaces {
		if s.Name == "ProfileViewModel" {
			foundVM = true
			if s.Type != ViewModel {
				t.Errorf("expected ProfileViewModel type ViewModel, got %s", s.Type)
			}
		}
	}
	if !foundVM {
		t.Error("expected to find ProfileViewModel surface")
	}

	// Should find LoginPage (SwiftUI View with Page suffix)
	foundLogin := false
	for _, s := range result.Surfaces {
		if s.Name == "LoginPage" {
			foundLogin = true
			// LoginPage conforms to View, so it should be View type
			if s.Type != View {
				t.Errorf("expected LoginPage type View, got %s", s.Type)
			}
		}
	}
	if !foundLogin {
		t.Error("expected to find LoginPage surface")
	}

	// Check counts
	if result.TestedCount == 0 {
		t.Error("expected at least one tested surface")
	}
	if result.UntestedCount == 0 {
		t.Error("expected at least one untested surface")
	}
}
