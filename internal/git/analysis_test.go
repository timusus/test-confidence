package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestGitAnalysis(t *testing.T) {
	// Create a temp git repo with known commit history
	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@test.com")
	runGit(t, dir, "config", "user.name", "Test")

	// Create initial files
	writeFile(t, dir, "src/main/Foo.kt", "class Foo {}")
	writeFile(t, dir, "src/test/FooTest.kt", "class FooTest {}")
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "initial")

	// Commit 1: small prod change + test change (potential structural coupling)
	writeFile(t, dir, "src/main/Foo.kt", "class Foo { val x = 1 }")
	writeFile(t, dir, "src/test/FooTest.kt", "class FooTest { fun test() {} }")
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "small change")

	// Commit 2: prod change only (test survived)
	writeFile(t, dir, "src/main/Foo.kt", "class Foo { val x = 2 }")
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "prod only")

	// Commit 3: large prod change + test change (expected)
	bigContent := "class Foo {\n"
	for i := 0; i < 20; i++ {
		bigContent += fmt.Sprintf("    fun method%d() {}\n", i)
	}
	bigContent += "}"
	writeFile(t, dir, "src/main/Foo.kt", bigContent)
	writeFile(t, dir, "src/test/FooTest.kt", "class FooTest { fun test1() {} fun test2() {} }")
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "big change")

	pairs := []FilePair{
		{
			TestFile:       filepath.Join(dir, "src/test/FooTest.kt"),
			ProductionFile: filepath.Join(dir, "src/main/Foo.kt"),
		},
	}

	result, err := Analyze(dir, pairs, DefaultAnalysisOptions())
	if err != nil {
		t.Fatal(err)
	}

	if len(result.FilePairStats) != 1 {
		t.Fatalf("expected 1 file pair stat, got %d", len(result.FilePairStats))
	}

	stat := result.FilePairStats[0]

	// ProdChanges: initial + commits 1, 2, 3 = 4
	if stat.ProdChanges != 4 {
		t.Errorf("expected 4 prod changes, got %d", stat.ProdChanges)
	}

	// CoChanges: initial + commits 1, 3 = 3
	if stat.CoChanges != 3 {
		t.Errorf("expected 3 co-changes, got %d", stat.CoChanges)
	}

	// CoChangeRatio: 3/4 = 0.75
	if stat.CoChangeRatio < 0.7 || stat.CoChangeRatio > 0.8 {
		t.Errorf("expected CoChangeRatio ~0.75, got %f", stat.CoChangeRatio)
	}

	// SmallProdChanges: initial + commits 1, 2 (< 10 lines) = 3
	if stat.SmallProdChanges != 3 {
		t.Errorf("expected 3 small prod changes, got %d", stat.SmallProdChanges)
	}

	// SmallProdWithTestChange: initial + commit 1 = 2
	if stat.SmallProdWithTestChange != 2 {
		t.Errorf("expected 2 small prod with test change, got %d", stat.SmallProdWithTestChange)
	}

	// StructuralCouplingRate: 2/3 ~ 0.667
	if stat.StructuralCouplingRate < 0.6 || stat.StructuralCouplingRate > 0.7 {
		t.Errorf("expected StructuralCouplingRate ~0.67, got %f", stat.StructuralCouplingRate)
	}

	if result.AnalyzedCommits == 0 {
		t.Error("expected analyzed commits > 0")
	}
}

func TestGitAnalysisFiltersBulkCommits(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@test.com")
	runGit(t, dir, "config", "user.name", "Test")

	// Create 60 files + commit (exceeds default 50 threshold)
	for i := 0; i < 60; i++ {
		writeFile(t, dir, fmt.Sprintf("file%d.kt", i), fmt.Sprintf("class File%d", i))
	}
	writeFile(t, dir, "src/main/Foo.kt", "class Foo")
	writeFile(t, dir, "src/test/FooTest.kt", "class FooTest")
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "bulk commit")

	// Normal commit
	writeFile(t, dir, "src/main/Foo.kt", "class Foo { val x = 1 }")
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "normal")

	pairs := []FilePair{{
		TestFile:       filepath.Join(dir, "src/test/FooTest.kt"),
		ProductionFile: filepath.Join(dir, "src/main/Foo.kt"),
	}}

	result, err := Analyze(dir, pairs, DefaultAnalysisOptions())
	if err != nil {
		t.Fatal(err)
	}

	// The bulk commit should be filtered
	if result.FilteredCommits == 0 {
		t.Error("expected at least 1 filtered commit")
	}

	stat := result.FilePairStats[0]
	// Only the normal commit should count
	if stat.ProdChanges != 1 {
		t.Errorf("expected 1 prod change (bulk filtered), got %d", stat.ProdChanges)
	}
}

func TestGitAnalysisWorkflowDetection(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@test.com")
	runGit(t, dir, "config", "user.name", "Test")

	writeFile(t, dir, "main.go", "package main")
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "initial")

	// No merge commits means Rebase workflow
	pairs := []FilePair{}
	result, err := Analyze(dir, pairs, DefaultAnalysisOptions())
	if err != nil {
		t.Fatal(err)
	}

	if result.WorkflowType != WorkflowRebase {
		t.Errorf("expected Rebase workflow for repo with no merges, got %s", result.WorkflowType)
	}
}

func TestGitAnalysisNotAGitRepo(t *testing.T) {
	dir := t.TempDir()
	pairs := []FilePair{}
	_, err := Analyze(dir, pairs, DefaultAnalysisOptions())
	if err == nil {
		t.Error("expected error for non-git directory")
	}
}

func TestGitAnalysisTestChurnRatio(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@test.com")
	runGit(t, dir, "config", "user.name", "Test")

	writeFile(t, dir, "src/main/Bar.kt", "class Bar {}")
	writeFile(t, dir, "src/test/BarTest.kt", "class BarTest {}")
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "initial")

	// Commit touching both
	writeFile(t, dir, "src/main/Bar.kt", "class Bar { val x = 1 }")
	writeFile(t, dir, "src/test/BarTest.kt", "class BarTest { fun test1() {} }")
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "change both")

	// Commit touching only test (test churn)
	writeFile(t, dir, "src/test/BarTest.kt", "class BarTest { fun test1() {} fun test2() {} }")
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "test only")

	pairs := []FilePair{{
		TestFile:       filepath.Join(dir, "src/test/BarTest.kt"),
		ProductionFile: filepath.Join(dir, "src/main/Bar.kt"),
	}}

	result, err := Analyze(dir, pairs, DefaultAnalysisOptions())
	if err != nil {
		t.Fatal(err)
	}

	stat := result.FilePairStats[0]

	// ProdChanges: initial + change both = 2
	// TestChanges: initial + change both + test only = 3
	// TestChurnRatio: 3/2 = 1.5
	if stat.TestChanges != 3 {
		t.Errorf("expected 3 test changes, got %d", stat.TestChanges)
	}
	if stat.TestChurnRatio < 1.4 || stat.TestChurnRatio > 1.6 {
		t.Errorf("expected TestChurnRatio ~1.5, got %f", stat.TestChurnRatio)
	}
}

// Helper functions

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}

func writeFile(t *testing.T, dir, relPath, content string) {
	t.Helper()
	full := filepath.Join(dir, relPath)
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
