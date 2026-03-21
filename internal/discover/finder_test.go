package discover

import (
	"strings"
	"testing"

	"github.com/timusus/test-confidence/internal/config"
)

func TestFindTestFiles(t *testing.T) {
	files, err := FindTestFiles("../../testdata/projects/android-simple", config.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 4 {
		t.Errorf("expected 4 test files, got %d", len(files))
		for _, f := range files {
			t.Logf("  found: %s", f.Path)
		}
	}
	// Should find files in src/test, src/androidTest, and src/commonTest
	hasUnitTest := false
	hasAndroidTest := false
	hasCommonTest := false
	for _, f := range files {
		if strings.Contains(f.Path, "src/test/") {
			hasUnitTest = true
		}
		if strings.Contains(f.Path, "src/androidTest/") {
			hasAndroidTest = true
			if !f.IsAndroidTest {
				t.Error("expected IsAndroidTest=true for androidTest file")
			}
		}
		if strings.Contains(f.Path, "src/commonTest/") {
			hasCommonTest = true
			if f.IsAndroidTest {
				t.Error("expected IsAndroidTest=false for commonTest file")
			}
		}
	}
	if !hasUnitTest || !hasAndroidTest || !hasCommonTest {
		t.Errorf("expected unit test, android test, and common test files; unit=%v android=%v common=%v",
			hasUnitTest, hasAndroidTest, hasCommonTest)
	}
}

func TestPairFiles(t *testing.T) {
	testFiles, _ := FindTestFiles("../../testdata/projects/android-simple", config.DefaultConfig())
	pairs := PairWithProductionFiles(testFiles, "../../testdata/projects/android-simple")
	paired := 0
	for _, p := range pairs {
		if p.ProductionFile != "" {
			paired++
		}
	}
	// Should pair: UserRepositoryTest->UserRepository, UserViewModelTest->UserViewModel,
	// SharedUtilTest->SharedUtil = 3 pairs
	if paired < 3 {
		t.Errorf("expected at least 3 paired files, got %d", paired)
		for _, p := range pairs {
			t.Logf("  %s -> %s", p.TestFile, p.ProductionFile)
		}
	}
}
