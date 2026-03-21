# Signal Calibration Summary

Calibrated against:
- **Now in Android** (Android/Kotlin) — 5 test files, fakes-only architecture
- **Alamofire** (iOS/Swift) — 5 test files, XCTest with async patterns

Date: 2026-03-20

## Overall Accuracy Per Signal

| Signal | Accuracy | Notes |
|--------|----------|-------|
| Behavioral vs. Structural classification | 9/10 (90%) | Correct for all NIA files; missed structural coupling in AuthenticationInterceptorTests |
| Structural coupling score | 9/10 (90%) | Only miss: hand-written call-count verification in Alamofire |
| Output-vs-interaction ratio | 9/10 (90%) | Same miss as above — call count assertions classified as output |
| Assertion count | 8/10 (80%) | Undercounts when assertions are in helper methods (ThemeTest) |
| Assertion strength | 8/10 (80%) | XCTAssertNil on error types incorrectly classified as "weak" |
| Mock/fake detection | 1/10 (10%) | Missed all hand-written fakes in both codebases |
| Setup complexity | 0/10 (0%) | Reported 0 for every file; @Before methods and class-level init not counted |
| Anti-pattern: ConditionalLogic | 4/6 (67%) | Defensive guards and Swift pattern matching cause false positives |
| Anti-pattern: GodTestClass | 0/1 (0%) | False positive on multi-class file |
| Anti-pattern: ZeroAssertionMethods | 3/3 (100%) | Correctly identified compilation-only tests |
| Compose/Robolectric detection | 1/1 (100%) | Correctly detected |
| Platform detection | 2/2 (100%) | Android and iOS both correct |

## Critical Disagreements

### 1. Hand-Written Fakes Are Invisible (Both Codebases)

**Impact: HIGH** — This is the most significant calibration gap.

Now in Android exclusively uses hand-written fakes (TestNewsResourceDao, TestUserDataRepository, etc.). Alamofire uses hand-written test doubles (TestAuthenticator, StubRequest). In all 10 files, fakeCount is reported as 0.

The tool only detects doubles created through mocking frameworks (MockK `mockk<T>()`, `spyk()`, etc.). It cannot recognize:
- Classes with `Test` prefix that implement interfaces
- Classes with `Stub`/`Fake`/`Mock` prefix that are hand-written
- Subclass-based test doubles (e.g., `StubRequest: DataRequest`)
- In-memory implementations (e.g., `InMemoryDataStore`)

**Why it matters**: The tool's behavioral classification still works (because there are no verify calls), but the "doubles" section of the report is completely empty for these codebases. Users see "0 fakes" and might think the tests use no test doubles at all, which is misleading.

**Recommendation**: Scan for classes/variables whose names start with Test/Stub/Fake/Mock/Dummy that appear in test files. Cross-reference with the constructors used in @Before/@SetUp methods and test class field declarations.

### 2. Setup Complexity Always Reports Zero (Both Codebases)

**Impact: MEDIUM** — The signal exists but never fires.

Every file reports `setupStatements: 0` and `ratio: 0`. The tool is not counting statements in `@Before` (JUnit), `setUp()` (XCTest), or class-level field initializations. This makes the setup complexity signal completely non-functional.

**Recommendation**: Parse @Before/@SetUp methods and count statements. Also count class-level field initializations that construct test dependencies.

### 3. Call-Count Verification Disguised as Property Assertions (Alamofire)

**Impact: HIGH** — Creates false behavioral classification.

AuthenticationInterceptorTests asserts `authenticator.applyCount == 1`, `authenticator.refreshCount == 0`, etc. These are interaction verification — functionally identical to `verify(exactly = 1) { authenticator.apply(any()) }`. The tool classifies them as output assertions because they syntactically look like property comparisons.

This is hard to detect statically. Possible heuristics:
- Properties named `*Count`, `*Called`, `*Invoked`, `*Times` on test double objects
- Properties that are only incremented inside methods of Test/Stub/Fake-prefixed classes
- The combination of a hand-written test double + count-tracking properties

**Recommendation**: When a class is identified as a hand-written test double (see point 1), flag assertions on its count-tracking properties as interaction verification, not output assertions.

### 4. Assertion Strength Misclassifies Error Nil-Checks (Alamofire)

**Impact: MEDIUM** — Misrepresents test quality for error-handling tests.

ValidationTests has 38 "weak" assertions, but most are `XCTAssertNil(error)` / `XCTAssertNotNil(error)`. In validation tests, the presence or absence of an error IS the primary behavior. Classifying these as "weak" makes a well-written test suite look poor.

**Recommendation**: Consider context when classifying nil checks. XCTAssertNil/NotNil on variables named `error`, `failure`, `result` in response handlers could be classified as "medium" rather than "weak". Alternatively, add a note when >50% of weak assertions are error-related nil checks.

### 5. Assertion Count Undercounts Helper Methods (NIA ThemeTest)

**Impact: MEDIUM** — Well-factored tests appear less thorough than they are.

ThemeTest uses `assertColorSchemesEqual()` helper containing 23 assertEquals calls. The tool counts 8 assertions (one per test method's top-level calls), missing ~200 assertions inside the helper. This penalizes good test design (extracting common assertions into helpers).

**Recommendation**: Follow one level of method calls within the same test class to count assertions in helper methods. Look for private methods that contain assertion calls (assertEquals, assertTrue, etc.) and include their assertion counts when called from test methods.

### 6. GodTestClass False Positive on Multi-Class Files (Alamofire)

**Impact: LOW** — Affects 1 file.

ParameterEncoderTests.swift contains 4 test classes in one file. The tool appears to count total test methods across all classes in the file, triggering GodTestClass for the file as a whole. The individual classes are reasonably sized.

**Recommendation**: Count test methods per class, not per file, when detecting god test classes.

## Signal Trustworthiness Ranking

**Highly Trustworthy (use with confidence):**
1. Output-vs-interaction ratio — correct 9/10, only fails on hand-written count verification
2. Structural coupling score — correct 9/10, same blind spot
3. Behavioral/structural classification — derived from the above, inherits their accuracy
4. Platform and framework detection — 100% accurate

**Trustworthy with Caveats:**
5. Assertion count — correct directionally but can undercount 2-3x for well-factored tests
6. Assertion strength — classification logic is sound but nil-check penalization is too aggressive
7. ConditionalLogic detection — catches real issues but also flags defensive guards

**Not Trustworthy (needs work):**
8. Fake/mock detection — completely broken for hand-written fakes (0% accuracy on these codebases)
9. Setup complexity — always reports 0, signal is non-functional
10. GodTestClass — counts per-file instead of per-class

## Summary

The tool's core strength — detecting structural coupling via verify/mock framework patterns — works well. For NIA (a fakes-only codebase), it correctly reports zero structural coupling and 95% behavioral. For Alamofire, it correctly reports 96% behavioral with zero verify calls.

The main blind spots are all related to **hand-written test infrastructure**: fakes not detected, setup not counted, call-count verification not recognized. These blind spots cluster together — if you can detect hand-written test doubles, you can also detect their count-tracking properties and count the setup that creates them.

For codebases that use mocking frameworks (MockK, Mockito, OCMock), the tool would likely score much higher on these signals. The calibration gap is specifically with the "fakes over mocks" testing style that both NIA and Alamofire exemplify.

---
