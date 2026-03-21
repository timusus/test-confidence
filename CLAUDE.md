# Confidence

A CLI tool that analyzes test suite quality through static analysis and git history. It answers: "Do these tests protect this code from breakage, without punishing refactoring?"

## Quick Reference

- **Language**: Go
- **Build**: `go build -o confidence ./cmd/confidence`
- **Run**: `./confidence scan <path>`
- **Test**: `go test ./...`
- **Design doc**: Read `DESIGN.md` before making architectural decisions. When in doubt, refer back to it.

## Usage

```bash
./confidence scan <path>                     # terminal report
./confidence scan <path> --json              # machine-readable JSON
./confidence scan <path> --verbose           # per-file detail
./confidence scan <path> --since 3months     # compare to 3 months ago
./confidence scan <path> --compare-ref v1.0  # compare to a git ref
./confidence scan <path> --compare old.json  # compare to a saved baseline
./confidence scan <path> --config path.yaml  # custom config
./confidence scan <path> --platform ios      # override platform detection
```

## Core Concepts

The tool measures **structural coupling indicators** in test code:
- Tests coupled to structure (verify calls, argument captors, mock orchestration) = fragile, punish refactoring
- Tests coupled to behavior (output assertions, fakes, boundary stubs) = robust, catch real bugs

This is not a binary classification — it's a spectrum. We compute indicators, not verdicts.

## Report Sections

The terminal report outputs these sections (each omitted if no data):

1. **Header + Test Style** — file/method counts, behavioral/structural/unclassified percentage
2. **Findings** — anti-patterns (Tier 1: sleep, empty, ignored, conditional, god class, assertion roulette)
3. **Structural Coupling Indicators** — output-vs-interaction ratio, verify-only tests, ArgumentCaptor, stub-and-verify overlap, Compose test file note
4. **Worst Files** — top 5 files ranked by structural coupling density (mock count, verify-only, argument captors)
5. **Mock Placement** — boundary vs internal percentage, most-mocked types with context labels
6. **Tautological Tests** — tests that assert the same value they stubbed
7. **Surface Coverage** — untested screens/ViewModels/Activities with file size, function count, and recent git churn
8. **Git Signals** — co-change rates, structural coupling from diff-size correlation, test churn
9. **Where To Start** — prioritized recommendations with cost context and fix strategies

## Architecture

```
cmd/confidence/          CLI entry point (Cobra)
internal/
  scan/                  Orchestration — discovery, analysis, reporting pipeline
  discover/              Find test files, pair with production files, detect platform
  parse/                 Tree-sitter parsing, AST traversal, symbol extraction
  analyze/               Per-file analysis (assertions, doubles, anti-patterns, tautology, structural, placement)
  git/                   Git history analysis (co-change, churn, noise filtering, workflow detection)
  surface/               Surface discovery (Screens, ViewModels, Activities) with churn ranking
  report/                Output formatting (terminal, JSON, comparison)
  platform/              Platform-specific patterns and detection
    android/             Kotlin/Android patterns (MockK, Compose, Robolectric, Hilt)
    ios/                 Swift/iOS patterns (XCTest, XCUITest, manual mocks, SnapshotTesting)
  model/                 Core data types
  config/                User configuration (.confidence.yaml), defaults, pattern lists
```

Note: `score/` is listed in the original spec but not implemented — aggregate scoring was deferred. Signals are reported individually without a composite score.

## Key Decisions

### Parsing: Tree-sitter, not regex
We use tree-sitter for all source code analysis. This gives us AST-level accuracy without requiring compilation or a language-specific runtime. Regex is acceptable only for trivial patterns in non-source files (manifests, configs).

### Classification by usage, not framework
MockK can create stubs, mocks, fakes, and spies. We classify based on how the double is *used* in the test — a double that is only stubbed (returns data, never verified) is functionally different from one that is primarily verified. The framework name is irrelevant.

### Signals, not opinions
Every output should be traceable to specific evidence. "72% of tests use internal mocks" is good. "Your tests are bad" is not. If you're adding a new signal, ask: "Can I point to the specific lines of code that produced this number?"

### Tiered confidence
Findings are presented in tiers: facts (>90% accuracy), indicators (70-90%), observations (50-70%). Never present a Tier 3 signal with the same visual weight as a Tier 1 signal.

### Don't report what you can't accurately count
If detection accuracy is below ~80% for a signal, don't report a number. Report the pattern qualitatively or acknowledge the detection limit. Example: Compose UI test files are noted as "assertions not fully analyzed" rather than being miscounted as assertion-free.

### Git analysis: diffs not messages, with noise filtering
Commit messages vary in quality. We analyze diffs — size, content, co-change frequency. We filter noise: bulk commits (>50 files), formatting commits, quick-remedy commits (same author <5 minutes). We detect the team's workflow (squash/merge/rebase) and adjust.

### Git churn tracks ALL production files
The git analyzer tracks churn for every file in the repo, not just files with test pairs. This ensures untested surfaces get churn data for risk ranking.

### Mock placement is configurable
The boundary/internal type indicator lists are user-editable via `.confidence.yaml`. `*Repository` defaults to boundary (Clean Architecture convention) but users can override. Ship sensible defaults, don't hardcode opinions.

### Verb-prefix auto-detection for use cases
Classes named `GetUser`, `UpdateProfile`, `RemoveItem` etc. are auto-classified as internal (use cases) without config. The verb prefix list is in `platform/android/patterns.go`.

### Verify calls with `withArg` assertions are not purely structural
`verify { withArg<T> { assertThat(it.field).isEqualTo(x) } }` contains real content assertions inside the verify block. The tool detects this and excludes these from stub-and-verify and verify-only counts.

### Fast path: no compilation, no execution
The tool reads source files and git history. It never compiles, executes tests, or requires a build environment.

## Coding Guidelines

- **Keep analyzers independent.** Each analyzer in `analyze/` examines one concern. They receive parsed AST data and return typed results. They don't call each other.
- **Platform patterns are data, not logic.** Platform-specific knowledge (what counts as a boundary mock in Android vs iOS) lives as pattern definitions in `platform/`, not as branching logic in analyzers.
- **Test with fixture files.** Analysis tests use real source code fixtures in `testdata/`. Don't test against string snippets — they drift from real patterns.
- **Accuracy tier matters.** When adding signals, document whether it's Tier 1/2/3 (see DESIGN.md). The tier determines how the signal is presented in output.
- **No external runtime dependencies.** The built binary should run anywhere without needing Go, JVM, Node, or anything else installed.
- **Git analysis must handle noise.** Every git analysis function must respect the noise filtering pipeline (changeset size, quick-remedy merging, generated file exclusion). Don't read raw git log without filtering.
- **Don't overfit to one codebase.** Validate changes against multiple projects with different architectures, test maturity, and conventions. The tool should produce useful output with zero configuration.

## Test Strategy (For This Tool)

- Unit tests for each analyzer against fixture files
- Integration tests for the full scan pipeline against small sample projects in `testdata/projects/`
- Golden file tests for report output formatting
- No mocking of the filesystem — use real temp directories with real fixture files
- Git analysis tests use small git repos created in test setup with known commit patterns

## What Not To Do

- Don't add LLM analysis. The tool is offline and deterministic.
- Don't add build/compilation steps. We work with source text and git.
- Don't add prescriptive recommendations like "you should write more unit tests." The "Where To Start" section recommends actions based on data, with cost and fix context.
- Don't use commit messages for classification. They're unreliable.
- Don't chase 100% accuracy on Tier 3 signals. Be transparent about confidence levels instead.
- Don't present Tier 3 observations as definitive findings. They need explicit caveats in output.
- Don't report numbers you can't get >80% accuracy on. Acknowledge detection limits instead.
