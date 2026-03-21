# Calibration Round 2 Summary

Calibrated against 7 new codebases (35 test files total), expanding from the original 4 codebases to cover testing patterns that were previously untested.

**Date:** 2026-03-21

## Codebases Added

| Codebase | Platform | Test Files | Key Patterns | Result |
|----------|----------|------------|--------------|--------|
| **Wire Android** | Kotlin | 5 | MockK, Turbine, JUnit 5 @ParameterizedTest, Robolectric, Compose UI | Scanned successfully |
| **DuckDuckGo Android** | Kotlin | 5 | Mockito-Kotlin, Robolectric, Turbine, Espresso | **CRASH** — nil pointer in findLambda |
| **Bitwarden Android** | Kotlin | 5 | MockK, JUnit 5, Turbine, Compose UI, Robolectric | Scanned successfully |
| **Kickstarter Android** | Kotlin | 5 | MockK + Mockito (dual), Compose UI, RxJava TestSubscriber | Scanned successfully |
| **Element X iOS** | Swift | 5 | Swift Testing (@Test, #expect), SnapshotTesting, Combine, Generated mocks | Scanned successfully |
| **TCA** | Swift | 5 | Swift Testing (@Suite, @Test), TestStore custom assertions, Macro testing | Scanned successfully |
| **Moya** | Swift | 5 | Quick/Nimble BDD (describe/context/it), Combine, RxSwift | **0 test files detected** |

## Combined Coverage (All 13 Codebases)

| Codebase | Mock Framework | Assertion Library | Special Patterns |
|----------|---------------|-------------------|-----------------|
| Now in Android | Fakes only | Google Truth | — |
| Architecture Samples | Fakes only | Google Truth, JUnit, Hamcrest | — |
| Tivi | Fakes only | assertk, kotlin.test | Turbine, ObjectGraph DI |
| **Wire Android** | **MockK** | JUnit assertEquals, Kotlin assert() | **Turbine, JUnit 5 @ParameterizedTest, Robolectric, Compose UI** |
| **DuckDuckGo Android** | **Mockito-Kotlin** | JUnit assertEquals | **Turbine, Robolectric, Espresso** |
| **Bitwarden Android** | **MockK** | JUnit 5 Jupiter | **Turbine, Compose UI, Robolectric** |
| **Kickstarter Android** | **MockK + Mockito** | JUnit assertEquals, RxJava TestSubscriber | **Dual framework, Compose UI** |
| Alamofire | Hand-written doubles | XCTest | Async patterns |
| RxSwift | None | XCTest | TestScheduler |
| private-ios-app | Mockable | XCTest | DependencyContainer DI |
| **Element X iOS** | **Sourcery-generated** | **Swift Testing #expect** | **SnapshotTesting, Combine, SwiftUI previews** |
| **TCA** | None | **Swift Testing #expect**, XCTest | **TestStore, Macro testing, #if canImport(Testing)** |
| **Moya** | None | **Quick/Nimble** | **BDD describe/context/it, RxSwift, Combine** |

## New Issues Discovered

### CRITICAL (tool fails completely)

#### 1. Crash on Mockito-Kotlin `whenever().thenReturn()` pattern
- **Affected:** DuckDuckGo Android (1,309 test files)
- **Location:** `internal/parse/kotlin_helpers.go:439` — `findLambda()` receives nil Node
- **Root cause:** `ExtractCallTarget` encounters an `infix_expression` node from Mockito-Kotlin's chained call pattern. When the first child isn't a `call_expression`, `callExpr` remains zero-value with nil Node field.
- **Fix:** Nil guard before `findLambda(callExpr)` call at line 271
- **Impact:** Any codebase using Mockito-Kotlin is unscannable

#### 2. Quick/Nimble specs completely invisible
- **Affected:** Moya (143 test methods across 5 files), plus any project using Quick/Nimble
- **Root cause:** Tool discovers test files by looking for `XCTestCase` subclasses and `func test*()` signatures. Quick specs subclass `QuickSpec` and use `it("...") { }` blocks.
- **Fix:** Recognize `QuickSpec` as a test base class; `it()` as test method; Nimble `expect().to()` as assertions
- **Impact:** Entire BDD-style Swift test suites are invisible

#### 3. Swift Testing `#expect()` not recognized as assertion
- **Affected:** Element X iOS (all unit tests), TCA (mixed files)
- **Root cause:** The assertion detection maps don't include `#expect` (Swift Testing's assertion macro)
- **Impact:** Methods using only `#expect` are flagged as zero-assertion. Assertion counts are undercounted by ~64% in Swift Testing files.

### SIGNIFICANT (wrong classification)

#### 4. Kotlin `assert()` not recognized as assertion
- **Affected:** Wire Android (84 assert() calls missed in ConversationAudioMessagePlayerTest alone)
- **Root cause:** Kotlin stdlib `assert()` is not in the assertion name maps
- **Impact:** Methods using Kotlin's built-in assert are marked as zero-assertion, cascading to wrong behavioral/structural classification

#### 5. `@ParameterizedTest` not detected as test method
- **Affected:** Wire Android, Bitwarden Android (both use JUnit 5)
- **Root cause:** Tool only recognizes `@Test` annotation, not `@ParameterizedTest`, `@RepeatedTest`, or other JUnit 5 test annotations
- **Impact:** Parameterized tests are invisible — methods not counted, assertions not counted

#### 6. Verify inside Compose test blocks invisible
- **Affected:** Bitwarden Android (RootNavScreenTest: 8 verify-based tests all reported as 0 coupling)
- **Root cause:** `verify { }` blocks inside `composeTestRule.runOnIdle { }` lambdas are not detected
- **Impact:** Structural coupling in Compose integration tests is completely missed

#### 7. Sourcery-generated protocol mocks not detected
- **Affected:** Element X iOS (AudioPlayerMock with .stopCalled, .seekToReceivedProgress properties)
- **Root cause:** Tool doesn't recognize the `*Mock` suffix with property-based call recording pattern
- **Impact:** mockDensity=0 for files using generated mocks

### MODERATE (wrong details but right direction)

#### 8. `assertEquals` classified as Medium instead of Strong
- **Affected:** Wire Android, Bitwarden Android, Kickstarter Android
- **Root cause:** JUnit 5 `assertEquals` (from `org.junit.jupiter.api.Assertions`) may not be in the `strongAssertions` map, or the function name resolution differs from JUnit 4
- **Impact:** Assertion strength distribution skewed toward Medium

#### 9. `assertThrows` / `assertFails` classified as Medium instead of Strong
- **Affected:** Wire Android, Bitwarden Android, Tivi
- **Root cause:** Exception assertion functions not in the exception assertions map (known issue from Round 1, still unfixed)

#### 10. Switch/case enum testing flagged as ConditionalLogic
- **Affected:** Element X iOS (RoomSummaryTests)
- **Root cause:** Swift's `switch` statement in tests is detected as conditional logic, but exhaustive enum pattern matching is a standard test assertion pattern
- **Impact:** False positive anti-pattern findings

#### 11. Struct-based Swift Testing extension files not discovered
- **Affected:** Element X iOS (GeneratedPreviewTests.swift — 18 @Test methods invisible)
- **Root cause:** Files that define test methods as extensions on a struct (rather than in the struct definition itself) are not discovered
- **Impact:** Generated test files (common with Sourcery + SnapshotTesting) are invisible

#### 12. `mockkStatic` not detected as a double
- **Affected:** Bitwarden Android
- **Root cause:** Static mock declarations not tracked in doubles array
- **Impact:** Incomplete doubles inventory

#### 13. TestStore classified as Fake double
- **Affected:** TCA (all files using TestStore)
- **Root cause:** `TestStore` matches the Fake naming heuristic, but it's a test harness, not a double of a production type
- **Impact:** Inflated fakeCount in TCA tests

#### 14. Dual MockK + Mockito detection works
- **Positive finding:** Kickstarter's ProjectPageViewModelTest uses both frameworks. The tool correctly identifies both. However, 5/8 ConditionalLogic findings are false positives (flagging `when(flagKey)` inside fake implementations, not test logic).

## Accuracy Summary by Signal (All 13 Codebases, 65 Files)

| Signal | Round 1 (4 repos) | Round 2 (+7 repos) | Trend |
|--------|-------------------|---------------------|-------|
| Behavioral classification | 90% | ~75% | Down — Kotlin assert() and #expect blindspots |
| Structural coupling score | 90% | ~80% | Down — Compose verify blocks missed |
| Output/interaction ratio | 90% | ~80% | Down — same root causes |
| Platform detection | 100% | 100% | Stable |
| Anti-pattern detection | 95% | ~85% | Down — ConditionalLogic false positives on switch/when |
| Assertion count | 80% | ~60% | Down — #expect, assert(), helper indirection, Compose |
| Assertion strength | 50% | ~45% | Stable-low — assertEquals/assertThrows still misclassified |
| Mock/fake detection | 70% | ~55% | Down — Sourcery mocks, mockkStatic, Quick/Nimble |
| Swift Testing support | 0% | ~50% | Up — @Suite/@Test detected, but #expect still missing |
| Quick/Nimble support | N/A | 0% | New gap discovered |
| JUnit 5 support | N/A | ~70% | @Test works, @ParameterizedTest missing |

## Priority Fixes (Ordered by Impact)

1. **Fix Mockito-Kotlin crash** — nil guard in `findLambda`. Unblocks scanning of all Mockito-Kotlin codebases.
2. **Add `#expect` to assertion detection** — Unblocks Swift Testing assertion counting for the growing Swift ecosystem.
3. **Add Kotlin `assert()` to assertion detection** — Common in Kotlin codebases, currently causes cascading misclassification.
4. **Add `@ParameterizedTest` to test method annotations** — Standard JUnit 5 pattern, currently invisible.
5. **Add Quick/Nimble support** — `QuickSpec` as test class, `it()` as test method, Nimble matchers as assertions.
6. **Detect verify inside Compose lambdas** — `composeTestRule.runOnIdle { verify { } }` pattern.
7. **Add `assertEquals` to strong assertions** (JUnit 5 variant) — Systematic strength undervaluation.
8. **Handle Swift Testing extensions** — Test methods in `extension FooTests { }` not discovered.
9. **Recognize Sourcery-generated mocks** — `*Mock` types with recorded properties.
10. **Suppress ConditionalLogic for switch/when on enums** — Standard test assertion pattern, not an anti-pattern.
