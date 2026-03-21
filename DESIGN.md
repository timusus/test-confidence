# Confidence — Design Philosophy

## What Are We Solving?

A tech lead points this tool at a codebase and gets an honest read on whether the test suite actually protects the code from breakage. Not coverage. Not test count. Not pyramid compliance. The answer to one question:

**"If I change this code, will the tests catch it if I break something — and will they leave me alone if I don't?"**

That's two properties in tension (Kent Beck's Test Desiderata, properties 7 and 8):
- **Sensitive to behavior changes** — the test fails when the code does the wrong thing
- **Insensitive to structure changes** — the test passes when the code is refactored without changing behavior

A test that has both properties is a good test. A test that has neither is dead weight. A test that only has structure-sensitivity is actively harmful — it punishes refactoring and rewards stagnation.

This is the core axis. Everything else in the tool serves to measure or contextualize it.

**No tool currently classifies tests as behavioral vs. structural with measured accuracy.** This is genuinely novel territory. We are honest about what we can and can't detect, and we frame our output as "structural coupling indicators" rather than definitive classification.

---

## Design Principles

### 1. Observe, Don't Prescribe — But Do Explain Cost and Fix

The tool describes what it sees. It surfaces patterns, signals, and data points. It does not prescribe test strategy.

However, it DOES explain the cost of each finding and suggest concrete fixes. "64 tests stub and verify the same method" is an observation. "Cost: these break on every refactor. Fix: replace verify with a Fake that captures state" is actionable guidance. The distinction: we explain the consequences and mechanics, but we don't say whether the team should prioritize this over other work.

Why: test strategy depends on context the tool cannot know — team size, release cadence, risk tolerance, product stage, regulatory requirements. But the cost/fix context is universal — a stub-and-verify test IS more fragile than a behavioral test regardless of context.

In practice:
- Present findings as observations: "72% of tests use internal mocks" not "you have too many mocks"
- Surface anti-patterns as patterns, not judgments: "these 14 tests contain `Thread.sleep()`" not "these tests are bad"
- Explain cost: "these tests break on every refactor, creating friction against improving code"
- Suggest fixes: "replace mockk<GetUser>() with FakeGetUser that returns test data"
- Provide codebase-level context: "Test style: 67% behavioral, 19% structural"
- Never gate on a score or fail a build by default

### 2. Signals Over Scores

While the tool computes aggregate numbers for convenience, the real value is in individual signals. A score of 65 is meaningless. "Your most-changed file has no tests, and 40% of your test suite is verify-heavy mocks on internal collaborators" is actionable.

Scores are summaries for dashboards. Signals are insights for engineers.

### 3. Evidence-Based

Every finding points to specific files, lines, and code. No vague warnings. If the tool says "this test has high structural coupling," it shows which verify calls, which argument captors, and which verify-ordering constraints produce that signal.

### 4. Offline, Deterministic, Fast

- No LLM, no network calls, no API keys
- Same input produces same output, every time
- Fast enough to run in CI on every PR (seconds, not minutes)
- Single binary, zero dependencies at runtime

### 5. Accuracy-Aware — Don't Report What You Can't Count

The tool is transparent about what it can and can't detect reliably.

**Hard rule: if detection accuracy is below ~80% for a signal, don't report a number.** Report the pattern qualitatively or acknowledge the detection limit. This was learned from real-world validation: Compose UI tests use assertion patterns (`composeTestRule.onNodeWithText("...").assertIsDisplayed()`) that tree-sitter can partially but not fully detect (~60% accuracy). Rather than reporting an inflated zero-assertion count, we note "N Compose UI test files detected — assertions not fully analyzed."

Research confirms these tiers:

| Confidence | Signal | Why |
|---|---|---|
| **High (>90%)** | Anti-patterns (sleep, empty, ignored tests), assertion counts, verify call counts, `verifyOrder`/`verifySequence` usage, `ArgumentCaptor` usage, fake class presence, untested surfaces, git co-change frequency (raw number) | Pattern matching on well-defined syntax; objective facts |
| **Good (70-90%)** | Output-vs-interaction assertion ratio, assertion strength distribution, mock density, setup-to-assertion ratio (at extremes), high-churn untested files | Requires interpreting usage patterns; some ambiguity but strong signal |
| **Moderate (50-70%)** | Mock placement (boundary vs internal), test scope sub-classification (collaborative vs isolated), stub specificity ratio, git diff-size correlation | Composite signals; context-dependent; heuristic-based |

The tool never presents moderate-confidence findings with the same weight as high-confidence findings. Tier the output.

### 6. Don't Overfit — Validate Across Codebases

Every change to the tool must be validated against multiple projects with different characteristics. This was a core lesson from development: patterns that look correct on one codebase can mislead on another.

Validate against codebases with different characteristics: healthy modern codebases (fakes-only, high behavioral %), large enterprise codebases (mixed mocks and fakes, analytics tests), and legacy codebases (minimal test coverage). A change that improves one output but breaks another is a regression. The tool should produce useful output with zero configuration on any Android/Kotlin or iOS/Swift project.

### 7. Context Matters — Maturity Shapes Expectations

A legacy codebase with broad integration tests that fake at system boundaries is **well-tested for its context**. The same shape in a mature, stable codebase might indicate missing granularity.

The tool detects maturity signals and adjusts its framing:
- **High churn, frequent structural changes** → broad behavioral tests are the right strategy. Reward them.
- **Stable architecture, incremental feature work** → granular tests are appropriate. Note their absence.
- **Mixed** → present the data, don't assume.

---

## The Core Taxonomy

### What We Call Things (And Why)

Test terminology is notoriously overloaded. This tool uses precise definitions internally and presents them clearly to users.

#### Test Doubles — Classified by Usage, Not Framework

MockK, Mockito, OCMock — these frameworks can create any kind of double. The framework name tells you nothing. What matters is how the double is used in each test.

| Term | Definition | How We Detect It |
|---|---|---|
| **Fake** | A working alternative implementation with simplified behavior. Has real logic. | Classes named `Fake*`/`*Fake`, implements an interface, contains logic (if/when/loops), no record-and-verify |
| **Stub** | Returns canned data. No verification of how it was called. | `every { } returns`, `whenever().thenReturn()`, mock framework usage WITHOUT corresponding `verify` calls on those methods |
| **Mock** | Records interactions for verification. The test asserts on how it was called. | `verify { }`, `coVerify { }`, `Mockito.verify()`, manual mock properties like `*.calledCount`, `*.lastArgument` |

A single test double can be both a stub (for setup) and a mock (for verification). The tool classifies **usage sites**, not objects. Empirical data (Spadini et al., MSR 2017) confirms this matters: only 4% of stubbed invocations are also verified, while 17% of non-stubbed invocations are verified. The distinction is real and detectable.

#### Test Scope — Three Reliable Categories

Instead of attempting a 5-level classification (isolated/collaborative/integrated/device/snapshot), we use **three categories we can detect with high confidence**, plus data points for further context:

| Scope | How We Detect It | Confidence |
|---|---|---|
| **Local** | Everything in `src/test/` (Kotlin) or host test targets (Swift). Runs on JVM/host machine. | High — path-based |
| **Device** | In `src/androidTest/`, imports XCUITest, uses `ActivityScenario` | High — path/import-based |
| **Snapshot** | Imports Paparazzi, Roborazzi, SnapshotTesting, or Compose Preview Screenshot | High — import-based |

Within **Local** tests, we present mock density and framework usage (Robolectric, Hilt, in-memory Room) as data points rather than attempting to sub-classify into isolated/collaborative/integrated. The boundary between these is fuzzy without type resolution, and we'd rather be honest than guess.

#### Assertion Target — What the Test Actually Checks

This is the most important classification for the behavioral/structural axis:

| Target | What's Asserted | Behavioral? | Detection Confidence |
|---|---|---|---|
| **Output** | Return values, emitted state, rendered UI, written data | Yes | High |
| **Interaction** | `verify` calls, call counts, argument capture | No — structural | High |
| **Side effect** | Observable external effects (file written, event emitted, message sent) | Depends — see below | Moderate |
| **Exception** | That an error is thrown/not thrown | Yes | High |

**The side-effect nuance**: `verify(emailService).send()` and `verify(repository).save()` use the same syntax but have different implications. Research (2025, "Understanding and Characterizing Mock Assertions") found 46% of verified methods relate to external resources — those are arguably behavioral. A verify on an **unstubbed** mock (never had `every/whenever` setup) is more likely a side-effect check (behavioral) than an orchestration check (structural). We use this as a signal.

---

## High-Value Signals

Ordered by diagnostic value, informed by stress-testing each signal's accuracy.

### Tier 1: Facts (>90% accuracy) — Present as definitive findings

These are syntactic patterns with minimal ambiguity:

- **Anti-patterns**: `Thread.sleep()`, `delay()` outside `runTest`, conditional logic in tests, empty test methods, `@Ignore`/`@Disabled` tests
- **Zero-assertion tests**: Test methods with no assertions ("the Liar")
- **Verify call count**: Total `verify`/`coVerify`/`Mockito.verify()` calls per file — unambiguously detectable
- **Verify ordering**: `verifyOrder`/`verifySequence`/`InOrder` usage — definitive structural coupling signal
- **ArgumentCaptor usage**: Almost always structural (Google's own guidance calls this out)
- **Stub-and-verify overlap**: Methods that are both stubbed AND verified in the same test — redundant/structural. Exception: verify calls containing `withArg` assertions (testing content, not just wiring) are excluded.
- **Fake class presence**: `Fake*` classes and their usage — strong architectural signal. Fakes are excluded from mock placement percentages and most-mocked type lists.
- **Untested surfaces**: Production entrypoints with zero test references
- **Git co-change frequency**: Raw number — objective fact (interpretation is Tier 3)

### Tier 2: Indicators (70-90% accuracy) — Present with confidence

Strong signals that require some interpretation:

- **Output-vs-interaction assertion ratio**: The best single static signal for behavioral vs structural. High output ratio = more behavioral. Not perfect (misses tautological tests) but captures the major axis.
- **Setup-only doubles vs verification doubles**: Doubles that are only stubbed (return data, never verified) = likely boundary/behavioral pattern. Doubles primarily verified = likely structural. Based on usage classification, not framework. Empirically validated by Spadini et al.
- **Assertion strength distribution**: Strong (equality, containment) vs weak (not-null) vs absent. Useful at the extremes.
- **Mock density**: Raw count of mock-related lines / total test lines per file.
- **High-churn untested files**: Files that change often and have no tests. Cross-referencing `git log --numstat` with test file pairing.
- **Tautological test detection**: When the asserted value is the same identifier as a stub's return value AND the SUT method has no branches/transformations (simple delegation). Detectable via AST — catches the canonical pattern with low false positives. (No existing tool does this; the concept is described by Vera-Pérez et al. as "pseudo-tested methods" and validated by the Descartes mutation engine.)

### Tier 3: Observations (50-70% accuracy) — Present with explicit caveats

Heuristic signals that depend on context:

- **Mock placement (boundary vs internal)**: Composite of multiple sub-signals (see below). Configurable, not hardcoded.
- **Test scope sub-classification within Local**: Whether a local test is isolated vs collaborative vs integrated. Depends on mock density thresholds and framework detection — fuzzy boundary.
- **Stub specificity ratio**: `any()` vs `eq()` matchers. `verifyOrder` is Tier 1, but the general any/eq ratio is weaker.
- **Git diff-size correlation**: When a small production change triggers a test change, it MIGHT indicate structural coupling — but small behavioral changes exist too (changing a default value). Present as an observation with caveats, not a primary signal.
- **Refactoring resilience**: What % of production changes did the test survive without changing? Directionally useful but confounded by active development, stale tests, and workflow differences.

### Mock Placement Sub-Signals

Mock placement (boundary vs internal) is a composite of multiple detectable signals, roughly prioritized by reliability:

1. **Import origin** (when import is from a third-party library): Always boundary. Definitive.
2. **Hilt/Dagger `@Binds` return types** (when DI modules are accessible): Definitively boundary interfaces (~95% accuracy).
3. **Package/module path** (always available): Types in `data/`, `network/`, `api/`, `persistence/` packages = boundary. Types in `domain/`, `feature/`, `ui/` = internal. High accuracy for well-structured projects.
4. **Type name** (always available): `*Api`, `*Client`, `*Dao`, `*DataSource` = boundary. `*UseCase`, `*ViewModel`, `*Presenter` = internal. `*Repository` = ambiguous (configurable, defaults to boundary).
5. **Verb-prefix auto-detection** (always available): Classes named `GetUser`, `UpdateProfile`, `RemoveItem`, `TogglePlayback` etc. are auto-classified as internal (use cases/commands). The verb prefix list covers common patterns (Get, Set, Update, Delete, Remove, Add, Create, Toggle, Follow, etc.). Requires the next character after the prefix to be uppercase to avoid false matches.
6. **Stubbing pattern** (always available, from test code alone): Mocks that are only stubbed and never verified = likely boundary. Mocks primarily verified = likely internal. Most robust fallback when other signals are absent.

Expected accuracy: 85-92% in Clean Architecture projects, 70-80% in moderately structured, 55-65% in flat/unstructured. The `*Repository` ambiguity is the single biggest classification error for Android codebases — make it configurable from day one.

Reference: MockSniffer (ASE 2020) achieved 63-78% accuracy with ML-based classification. Our composite heuristic approach should be competitive, especially with configurability.

---

## What the Tool Does NOT Do

- **Does not run tests.** It analyzes test code and git history. No compilation, no execution.
- **Does not compute code coverage.** Coverage requires execution. Other tools do this.
- **Does not run mutation testing.** Too slow for the tool's fast-feedback model. However, PIT with the Descartes engine is the recommended calibration tool — it can validate our signals against ground truth for pseudo-tested methods and tautological tests.
- **Does not use LLMs.** Offline, deterministic, no API keys.
- **Does not prescribe test strategy.** It doesn't say "you need more unit tests" or "your pyramid is wrong." It DOES explain the cost of specific patterns and suggest mechanical fixes (e.g., "replace verify with a Fake"), but it doesn't tell you what to prioritize.
- **Does not assign blame.** It never attributes findings to authors or commits.
- **Does not fail builds.** It reports. Humans decide.
- **Does not report numbers it can't accurately detect.** If detection accuracy is below ~80%, it acknowledges the limit rather than producing a misleading count.
- **Does not classify tests as definitively "behavioral" or "structural."** It computes structural coupling indicators. The behavioral/structural distinction is ultimately a semantic property (Rice's theorem) that no static analysis can perfectly determine. We measure proxies.

---

## How We Think About "What Would an LLM Look For?"

An LLM reviewing a test would consider:

1. "What is this test actually verifying?" → We approximate with assertion target classification (output vs interaction vs side-effect)
2. "If I refactored the implementation, would this test break?" → We approximate with structural coupling indicators (verify calls, argument captors, verify ordering, stub-and-verify overlap)
3. "Is this test testing real behavior or just mirroring the implementation?" → We approximate with tautological test detection (same-identifier heuristic + SUT method complexity check) and output-vs-interaction ratio
4. "Is this test fragile?" → We detect anti-patterns and timing dependencies
5. "Is this mock at the right level?" → We approximate with mock placement analysis (composite of package, stubbing pattern, type name, DI module signals)
6. "Does this test add confidence?" → Composite of all the above

The tool replicates LLM-level judgment by decomposing it into measurable signals and combining them. Each signal is individually less accurate than an LLM's holistic read, but the combination — applied consistently across thousands of files — provides a systematic assessment no human or LLM could practically perform at scale.

**What we fundamentally cannot detect** (the hard boundary of static analysis):
- Whether a dependency represents a contract boundary vs an internal collaborator — this is an architectural decision in the developer's head
- Whether an expected value in an assertion is semantically meaningful or copy-pasted from the implementation
- Whether a test would survive an arbitrary refactoring — this requires reasoning about all possible refactorings (undecidable)
- Developer intent behind a verify call (contractual obligation vs couldn't-think-of-a-better-assertion)

---

## Git Analysis Strategy

Git history is valuable but noisy. Different teams work differently. The tool must handle this gracefully.

### Noise Sources and Mitigations

| Noise Source | Mitigation | Reference |
|---|---|---|
| **Large/bulk commits** | Ignore commits touching >N files (default 50, configurable). CodeScene uses this. | CodeScene docs |
| **Formatting/linting commits** | Detect via: commits touching >50% of files, or uniform diff patterns (whitespace-only changes). Exclude automatically. Support `.git-blame-ignore-revs` file. | Git 2.23+ feature |
| **Squash merges** | Each commit on main IS a logical unit already — the simplest case. Detect squash workflow automatically. | Tornhill, "Software Design X-Rays" |
| **Quick remedy commits** | Merge commits by the same author within 5 minutes into one logical unit. | EMSE 2021 research |
| **Monorepo noise** | Scope analysis to configured sub-project boundaries. Only flag cross-boundary coupling when explicitly requested. | CodeScene component boundaries |
| **Tangled commits** | Accept that 7-20% of commits contain unrelated changes (Herzig & Zeller, 2013). Statistical aggregation over many commits surfaces the real signal. | MSR 2013 |
| **Generated code** | Exclude files matching configurable patterns (`*Generated*`, `*_generated.*`, `build/`, etc.) | Standard practice |

### Workflow Detection

The tool detects the team's git workflow and adjusts:

- **Squash merge**: Each main-branch commit = one logical unit. Clean signal.
- **Merge commits**: Use `--first-parent` to treat each merge as one logical unit. Optionally analyze within-PR commits separately.
- **Rebase workflow**: Commits are individual. Apply noise filters (changeset size, quick-remedy merging).
- **Unknown/mixed**: Apply all filters conservatively. Note reduced confidence in git signals.

### What Git Analysis Can and Can't Tell Us

**Can tell us (present as facts):**
- Co-change frequency between file pairs — objective number
- Which production files change most often with no test counterpart
- Test file churn rate relative to production file churn rate

**Can suggest (present as indicators with caveats):**
- Diff-size correlation as a proxy for structural coupling (small prod change + test change = possibly structural)
- Refactoring resilience (% of production changes test survived without changing)

**Cannot tell us:**
- Whether a co-change was because of structural coupling or legitimate behavior change
- Whether a test that never changes is behavioral or simply abandoned
- Whether a specific commit was a refactor — RefactoringMiner can do this with 99.6% precision for Java/Kotlin but requires full source checkout at both commits, making it impractical as a default analysis

---

## Platform Strategy

### Android/Kotlin (Primary)
- Tree-sitter Kotlin grammar for parsing
- MockK and Mockito pattern recognition
- Compose testing patterns (`composeTestRule`, `onNode*`, `assert*`)
- Robolectric detection (test runner, `@Config`, shadow usage)
- Hilt/Dagger module analysis for boundary interface detection (`@Binds`, `@Provides`, `@EntryPoint`)
- PIT + Descartes available as external calibration tools for pseudo-tested method and tautological test validation

### iOS/Swift (Secondary)
- Tree-sitter Swift grammar for parsing
- Manual mock pattern detection (`*Called`, `*CallCount`, `*LastArgument` properties)
- XCTest assertion patterns
- XCUITest element query patterns
- ViewInspector detection
- SnapshotTesting detection
- No mutation testing calibration available — rely on static signals + Android-calibrated weights

### Future Languages
- The architecture should not preclude adding Go, TypeScript, etc.
- Tree-sitter grammars exist for most languages
- The core signals (assertion analysis, git analysis) are language-agnostic
- Platform-specific analyzers are pluggable

---

## Key References

These informed the design and should be consulted when making architectural decisions:

- Kent Beck, [Test Desiderata](https://testdesiderata.com/) — the 12 properties framework
- Martin Fowler, [Mocks Aren't Stubs](https://martinfowler.com/articles/mocksArentStubs.html) — state vs interaction testing
- Freeman & Pryce, [Mock Roles, Not Objects](https://jmock.org/oopsla2004.pdf) — mock at boundaries, not internals
- Google, [Software Engineering at Google Ch.12](https://abseil.io/resources/swe-book/html/ch12.html) — "test via public APIs", interaction tests are brittle
- Google, [Change-Detector Tests Considered Harmful](https://testing.googleblog.com/2015/01/testing-on-toilet-change-detector-tests.html)
- Spadini et al., [To Mock or Not to Mock?](https://sback.it/publications/msr2017b.pdf) — empirical data on what gets mocked
- Vera-Pérez et al., [Pseudo-tested Methods](https://inria.hal.science/hal-01867423v2) — methods covered but not actually tested
- Spadini et al., [Test Smells and Software Quality](https://ieeexplore.ieee.org/document/8529832/) — smelly tests are 81% more defect-prone
- Herzig & Zeller, [Tangled Code Changes](https://www.semanticscholar.org/paper/The-impact-of-tangled-code-changes-Herzig-Zeller/390ef1df3f3552e6d17b34d25bb1dcb783272cc6) — 7-20% of commits are tangled
- Adam Tornhill, [Your Code as a Crime Scene](https://www.adamtornhill.com/articles/crimescene/codeascrimescene.htm) — temporal coupling analysis
- MockSniffer, [ASE 2020](https://liliweise.github.io/materials/ASE20_MockSniffer.pdf) — ML-based mock recommendation (63-78% accuracy)
- tsDetect, [ESEC/FSE 2020](https://dl.acm.org/doi/10.1145/3368089.3417921) — test smell detection (96% precision)
