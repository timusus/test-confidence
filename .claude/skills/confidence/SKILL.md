---
name: confidence
description: Analyze test suite quality using the Confidence CLI tool. Use when asked to assess test quality, review test health, generate a test confidence report, or when phrases like "run confidence", "test quality report", "how are the tests", "scan tests", "test health" are used. Also use proactively after completing major test-related work to validate quality.
allowed-tools: Bash(confidence *)
---

# Confidence — Test Suite Quality Analysis

Analyze test suite quality through static analysis and git history using the `confidence` CLI.

## Install

If `confidence` is not on PATH, build it from the repo:

```bash
go build -o confidence ./cmd/confidence
```

Or install globally:

```bash
go install github.com/timusus/test-confidence/cmd/confidence@latest
```

## Commands

**Default to the HTML report** — it's the most useful output for understanding a codebase. Only use terminal or JSON for quick checks or CI.

### HTML report (preferred — visual, self-contained)
```bash
confidence scan <path> --html > /tmp/confidence-report.html && open /tmp/confidence-report.html
```

### HTML with historical trends (recommended for first run)
```bash
confidence scan <path> --html --trend-months 12 > /tmp/confidence-report.html && open /tmp/confidence-report.html
```

### Terminal report (quick check)
```bash
confidence scan <path>
```

### JSON output (for CI or programmatic use)
```bash
confidence scan <path> --json
```

### With coverage data (optional, enriches analysis)
```bash
# Android — Kover/JaCoCo XML
confidence scan <path> --coverage build/reports/kover/report.xml

# iOS — xccov JSON (generate with: xcrun xccov view --report --json <bundle>.xcresult > coverage.json)
confidence scan <path> --platform ios --coverage coverage.json
```

### iOS projects (need platform flag)
```bash
confidence scan <path> --platform ios
```

### Verbose per-file detail
```bash
confidence scan <path> --verbose
```

### Compare to baseline
```bash
confidence scan <path> --since 3months
confidence scan <path> --compare-ref v1.0
confidence scan <path> --compare old.json
```

## When to use

- User asks about test quality, test health, or test confidence for a project
- After completing a major feature or test refactor, to check impact
- When reviewing a project's test strategy or test suite
- When comparing test quality across projects or over time
- When asked to identify the highest-risk untested code

## Interpreting results

The tool presents signals, not opinions. Focus on:

- **Behavioral %** — proportion of tests checking outcomes vs implementation
- **Where To Start** — prioritized recommendations ranked by impact
- **Evidence** — observed maintenance cost from git history (how often refactors break tests)
- **Risk Hotspots** — files with high churn AND fragile tests
- **Coverage Gaps** — untested surfaces and business logic
- **Patterns** — novel cross-file signals (mock blast radius, test freshness, production complexity shadow)

## Notes

- The tool reads source files and git history only — never compiles or executes tests
- All analysis is offline and deterministic
- Coverage data is optional but enriches every signal when available
- Works on Android/Kotlin and iOS/Swift codebases
