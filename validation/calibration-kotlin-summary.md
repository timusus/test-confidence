# Kotlin Calibration Summary

**Codebases:** Google Architecture Samples, Tivi (KMP)
**Date:** 2026-03-20
**Files reviewed:** 10 (5 per codebase)

## Overall Accuracy by Signal

| Signal | Correct | Partially Wrong | Wrong | Accuracy |
|--------|---------|-----------------|-------|----------|
| Behavioral classification | 10/10 | 0 | 0 | 100% |
| Structural coupling score | 10/10 | 0 | 0 | 100% |
| Output/Interaction ratio | 10/10 | 0 | 0 | 100% |
| Assertion detection (count) | 8/10 | 2 | 0 | 80% |
| Assertion strength | 5/10 | 5 | 0 | 50% |
| Fake/double detection | 7/10 | 2 | 1 | 70% |
| Mock placement | 10/10 | 0 | 0 | 100% |
| Setup complexity | 10/10 | 0 | 0 | 100% (by design) |

**Aggregate accuracy: ~85% on major signals (behavioral/structural/ratio), ~67% on detail signals (strength/fakes)**

## Specific Disagreements

### 1. assertFails (kotlin.test) not recognized as Exception assertion
**Files affected:** EpisodesTest, EpisodeWatchEntryTest, SeasonsTest (all tivi DAO tests)
**Impact:** 3 assertions across 3 files misclassified

`assertFails { ... }` from `kotlin.test` is functionally identical to `assertThrows`/`assertFailsWith` but is not in the `exceptionAssertions` map in `assertions.go`. It gets matched by the "assert" prefix in `isAssertionName()` so it IS detected as an assertion, but:
- **Target:** Classified as Output instead of Exception
- **Strength:** Classified as Medium (default) instead of Strong

**Fix:** Add `"assertFails": true` to the `exceptionAssertions` map in `internal/analyze/assertions.go`.

### 2. assertk's isNull() classified as Medium, should be Weak
**Files affected:** EpisodesTest, EpisodeWatchEntryTest, SeasonsTest (all tivi DAO tests)
**Impact:** 5 assertions across 3 files get wrong strength

The `weakAssertions` map contains `assertNull` and `isNotNull` but not `isNull`. When assertk's `assertThat(x).isNull()` is encountered, the terminal method `isNull` doesn't match any strength map entry, defaulting to Medium. Null-checking assertions should be Weak, consistent with `assertNull`.

**Fix:** Add `"isNull": true` to the `weakAssertions` map in `internal/analyze/assertions.go`.

### 3. Hamcrest assertions all classified as Medium
**Files affected:** StatisticsUtilsTest (architecture-samples)
**Impact:** 8 assertions in 1 file

Hamcrest's `assertThat(value, is(expected))` pattern uses a two-argument call where the matcher is an argument, not a chained method. The tool sees `assertThat` as the root call, which doesn't appear in any strength map, so it defaults to Medium. The `is()` matcher IS equality checking and should be Strong.

**Severity:** Low. This is a known detection limit. Hamcrest is legacy and uncommon in new Kotlin code. The tool cannot analyze matcher arguments without significant complexity. Documenting this as a known limitation is sufficient.

### 4. Fakes accessed via ObjectGraph helper not detected
**Files affected:** SeasonsEpisodesRepositoryTest (fakeCount=0, should be 3+), FollowedShowRepositoryTest (fakeCount=1, should be 2+)
**Impact:** Significant undercount of fake usage in tivi

Tivi uses an `ObjectGraph` class that constructs all test dependencies (real DAOs + fake data sources). Test files access fakes through delegated properties like `private val traktSeasonsEpisodesDataSource get() = objectGraph.traktSeasonsEpisodesDataSource`. The tool only detects fakes when:
1. The type name starts with "Fake" AND
2. The property is declared directly in the test file

When fakes are accessed via delegation to a helper object, the tool cannot trace the type without cross-file analysis.

**Severity:** Moderate for tivi-style codebases. The behavioral classification and structural coupling score are still correct (the tool doesn't need fake detection to classify tests correctly in this case). But the fake count underreports the test's dependency strategy.

### 5. Turbine .test { } block assertions undercounted
**Files affected:** SeasonsEpisodesRepositoryTest (testObserveNextEpisodeToWatch_singleFlow)
**Impact:** 1 assertion undercounted, 2 assertions get wrong strength

The Turbine `flow.test { awaitItem(); assertThat(...) }` pattern puts assertions inside a lambda. The AST walker sees the `.test { }` call as a single statement. The text-based fallback correctly detects it contains assertions, but:
- Counts the entire block as 1 assertion instead of 2
- Classifies it as Medium (fallback default) instead of Strong (the actual assertions use isEqualTo)

**Severity:** Minor. Turbine usage is common in Kotlin Flow testing. The tool already has the text-based fallback which prevents zero-assertion false positives, but it loses granularity inside the lambda.

### 6. doesNotContain (Google Truth) not in strength map
**Files affected:** DefaultTaskRepositoryTest (architecture-samples)
**Impact:** 2 assertions classified as Medium instead of Strong

Google Truth's `doesNotContain()` is a containment assertion (equivalent in strength to `contains()` which IS in the Strong map). Currently defaults to Medium.

**Fix:** Add `"doesNotContain": true` to `strongAssertions` map.

### 7. @Test(expected=...) annotations not counted as assertions
**Files affected:** DefaultTaskRepositoryTest (architecture-samples)
**Impact:** 2 implicit assertions not counted

JUnit's `@Test(expected = Exception::class)` is an annotation-based assertion that the test should throw. The tool doesn't parse test annotations for expected exceptions.

**Severity:** Low. This pattern is deprecated in favor of explicit assertThrows/assertFails. Only affects legacy test code.

## Patterns the Tool Gets Right for Kotlin

1. **Behavioral vs structural classification:** Perfect accuracy across both codebases. Both use fakes with no mocking, and the tool correctly identifies 100% behavioral for all files.

2. **Structural coupling score:** Correctly 0 for all files. Neither codebase uses verify/mock interactions.

3. **Google Truth assertion chains:** `assertThat(x).isEqualTo(y)`, `assertThat(x).isTrue()`, `assertThat(x).hasSize(n)`, `assertThat(x).isEmpty()`, `assertThat(x).contains(y)` all correctly detected and classified.

4. **assertk assertion chains:** `assertThat(x).isEqualTo(y)`, `assertThat(x).containsExactly(a, b)`, `assertThat(x).isEmpty()` all correctly detected. The tool handles assertk identically to Google Truth since both use the same chaining pattern.

5. **Fake detection by naming convention:** `FakeTaskRepository`, `FakeNetworkDataSource`, `FakeTaskDao`, `FakeFollowedShowsDataSource` all correctly identified as fakes with boundary placement.

6. **Scope detection:** Correctly distinguishes Device (androidTest with @RunWith(AndroidJUnit4)) from Local (unit tests, commonTest).

7. **Compose UI test file detection:** Correctly identifies Compose test files and applies the "assertions not fully analyzed" caveat.

## Comparison: Fakes-Only (Architecture Samples) vs assertk (Tivi)

### Architecture Samples (Google Truth + JUnit + Fakes)
- **Detection accuracy:** Very high. Google Truth's chaining pattern is well-supported. JUnit assertEquals/assertNotNull directly matched.
- **Fake detection:** Good for directly-declared fakes. The FakeTaskRepository pattern is detected perfectly.
- **Gap:** Hamcrest matchers (legacy, used in StatisticsUtilsTest only) get downgraded to Medium.
- **Overall:** The tool was likely calibrated against this style of test. It performs well.

### Tivi (assertk + kotlin.test + Turbine + ObjectGraph)
- **Detection accuracy:** Good for assertk (same chaining as Truth). Gaps in kotlin.test (assertFails) and assertk edge cases (isNull).
- **Fake detection:** Poor when fakes are accessed via helper classes (ObjectGraph pattern). The tool's fake detection is file-scoped.
- **Turbine:** Partially handled by text fallback but loses assertion count and strength granularity.
- **Overall:** The tool handles the common paths well but misses KMP-specific patterns (kotlin.test assertions, cross-file fake detection).

### Key Takeaway
The tool's strength is in structural-vs-behavioral classification and structural coupling detection, which are correct 100% of the time across both codebases. The assertion strength classification has systematic gaps for lesser-used assertion functions (isNull, assertFails, doesNotContain) but gets the major ones right. Fake detection is file-scoped, which works for simple test setups but fails for dependency injection helper patterns (ObjectGraph, test modules).

The most impactful fixes would be:
1. Add `assertFails` to `exceptionAssertions` (easy, high impact for kotlin.test users)
2. Add `isNull` to `weakAssertions` (easy, correctness fix)
3. Add `doesNotContain` to `strongAssertions` (easy, correctness fix)
4. Consider cross-file fake type resolution for delegated properties (hard, high impact for DI-heavy test suites)
