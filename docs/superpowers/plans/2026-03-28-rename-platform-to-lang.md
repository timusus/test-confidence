# Rename Platform to Lang Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the `--platform android|ios` flag with `--lang kotlin|swift` and improve auto-detection to fall back on file extensions instead of defaulting to Android.

**Architecture:** Remove the `Platform` enum entirely since it maps 1:1 to `Language`. All internal code switches from `Platform` to `Language`. Auto-detection keeps marker file checks (fast path) but falls back to counting `.kt` vs `.swift` files instead of defaulting to Android.

**Tech Stack:** Go, tree-sitter, Cobra CLI

---

## File Map

| File | Action | Responsibility |
|---|---|---|
| `internal/model/enums.go` | Modify | Remove `Platform` enum |
| `internal/model/types.go` | Modify | `ProjectContext.Platform` → `Language`, `ScanResult.Platform` → `Language` |
| `internal/model/results.go` | Modify | `ScanResult.Platform` field type change |
| `internal/discover/platform.go` | Modify → rename to `lang.go` | Detection logic: marker files + extension fallback |
| `internal/discover/platform_test.go` | Modify → rename to `lang_test.go` | Tests for detection |
| `internal/discover/context.go` | Modify | Use `Language` instead of `Platform`, handle Swift skip |
| `internal/scan/engine.go` | Modify | Remove platform→lang derivation (now direct), update signatures |
| `internal/config/config.go` | Modify | `ApplyPlatformDefaults` → `ApplyLanguageDefaults` |
| `internal/config/defaults.go` | Modify | `DefaultConfigForPlatform` → `DefaultConfigForLanguage` |
| `internal/report/terminal.go` | Modify | Remove `platformLanguage()` helper, use `Language.String()` |
| `internal/report/json.go` | Modify | JSON field `platform` → `language` |
| `internal/surface/discovery.go` | Modify | Parameter type `Platform` → `Language` |
| `cmd/confidence/main.go` | Modify | `--platform` → `--lang`, values `kotlin|swift` |
| `internal/report/terminal_test.go` | Modify | Update fixture Platform references |
| `internal/report/json_test.go` | Modify | Update fixture Platform references |
| `internal/scan/engine_test.go` | Modify | Update Platform references to Language |
| `internal/analyze/placement_test.go` | Modify | Update Platform references to Language |

---

### Task 1: Remove Platform enum, update model types

**Files:**
- Modify: `internal/model/enums.go` — remove `Platform` type and constants
- Modify: `internal/model/types.go:116-123` — `ProjectContext.Platform` → `ProjectContext.Language` (already has `Language` field, just remove `Platform`)
- Modify: `internal/model/results.go:91-93` — `ScanResult.Platform` → `ScanResult.Language`

- [ ] **Step 1: Remove Platform enum from enums.go**

Remove the `Platform` type, `Android`/`IOS` constants, and `Platform.String()` method (lines 21-37 of `enums.go`).

- [ ] **Step 2: Update ProjectContext in types.go**

Remove the `Platform Platform` field from `ProjectContext` (line 117). The `Language Language` field (line 118) already exists and stays.

- [ ] **Step 3: Update ScanResult in results.go**

Change `Platform Platform` (line 93) to `Language Language`.

- [ ] **Step 4: Verify the build fails with expected errors**

Run: `go build ./... 2>&1 | head -30`
Expected: Many compilation errors referencing `model.Platform`, `model.Android`, `model.IOS` — these will be fixed in subsequent tasks.

---

### Task 2: Update detection logic — marker files + extension fallback

**Files:**
- Modify: `internal/discover/platform.go` (rename to `lang.go`)
- Modify: `internal/discover/platform_test.go` (rename to `lang_test.go`)

- [ ] **Step 1: Rename platform.go to lang.go**

```bash
git mv internal/discover/platform.go internal/discover/lang.go
git mv internal/discover/platform_test.go internal/discover/lang_test.go
```

- [ ] **Step 2: Rewrite DetectPlatform → DetectLanguage in lang.go**

Replace the entire function. New logic:
1. Check for `build.gradle.kts` or `build.gradle` → `model.Kotlin`
2. Check for `Package.swift` or `*.xcodeproj` → `model.Swift`
3. Fallback: walk the directory (max depth 4, bail early at 20 files), count `.kt` vs `.swift` files. Majority wins.
4. If no source files found at all, return `model.Kotlin` (preserve existing default).

```go
// DetectLanguage determines the project language by checking for build system
// markers first (fast path), then falling back to file extension counting.
func DetectLanguage(root string) model.Language {
	// Fast path: check build system markers at root.
	if fileExists(filepath.Join(root, "build.gradle.kts")) ||
		fileExists(filepath.Join(root, "build.gradle")) {
		return model.Kotlin
	}

	if fileExists(filepath.Join(root, "Package.swift")) {
		return model.Swift
	}

	entries, err := os.ReadDir(root)
	if err == nil {
		for _, e := range entries {
			if e.IsDir() && filepath.Ext(e.Name()) == ".xcodeproj" {
				return model.Swift
			}
		}
	}

	// Slow path: count source file extensions.
	return detectByFileExtensions(root)
}

// detectByFileExtensions walks the directory tree (limited depth) counting
// .kt vs .swift files. Returns the majority language, defaulting to Kotlin.
func detectByFileExtensions(root string) model.Language {
	ktCount := 0
	swiftCount := 0
	threshold := 20 // stop after this many source files — enough signal

	filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			// Limit depth to 4 levels
			rel, relErr := filepath.Rel(root, path)
			if relErr == nil && strings.Count(rel, string(filepath.Separator)) >= 4 {
				return filepath.SkipDir
			}
			if skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}

		switch filepath.Ext(d.Name()) {
		case ".kt":
			ktCount++
		case ".swift":
			swiftCount++
		}

		if ktCount+swiftCount >= threshold {
			return filepath.SkipAll
		}
		return nil
	})

	if swiftCount > ktCount {
		return model.Swift
	}
	return model.Kotlin // default
}
```

Add the missing import `"io/fs"` and ensure `skipDirs` is accessible (it's already defined in `finder.go` in the same package).

- [ ] **Step 3: Update lang_test.go**

Replace the existing test and add new cases:

```go
func TestDetectLanguage_AndroidMarker(t *testing.T) {
	lang := DetectLanguage("../../testdata/projects/android-simple")
	if lang != model.Kotlin {
		t.Errorf("expected Kotlin, got %v", lang)
	}
}

func TestDetectLanguage_FallbackToExtensions(t *testing.T) {
	// Create a temp dir with .swift files but no marker files
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "Sources"), 0755)
	os.WriteFile(filepath.Join(dir, "Sources/App.swift"), []byte("import Foundation"), 0644)
	os.WriteFile(filepath.Join(dir, "Sources/Model.swift"), []byte("struct Model {}"), 0644)

	lang := DetectLanguage(dir)
	if lang != model.Swift {
		t.Errorf("expected Swift from extension fallback, got %v", lang)
	}
}

func TestDetectLanguage_NoFilesDefaultsToKotlin(t *testing.T) {
	dir := t.TempDir()
	lang := DetectLanguage(dir)
	if lang != model.Kotlin {
		t.Errorf("expected Kotlin default, got %v", lang)
	}
}

func TestDetectLanguage_XcodeprojMarker(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "MyApp.xcodeproj"), 0755)

	lang := DetectLanguage(dir)
	if lang != model.Swift {
		t.Errorf("expected Swift from .xcodeproj marker, got %v", lang)
	}
}

func TestDetectLanguage_PackageSwiftMarker(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "Package.swift"), []byte("// swift-tools-version: 5.9"), 0644)

	lang := DetectLanguage(dir)
	if lang != model.Swift {
		t.Errorf("expected Swift from Package.swift marker, got %v", lang)
	}
}
```

- [ ] **Step 4: Run detection tests**

Run: `go test ./internal/discover/ -run TestDetectLanguage -v`
Expected: All pass.

- [ ] **Step 5: Commit**

```bash
git add internal/discover/lang.go internal/discover/lang_test.go
git commit -m "rename DetectPlatform to DetectLanguage with extension fallback"
```

---

### Task 3: Update config package — Platform → Language

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/defaults.go`

- [ ] **Step 1: Update config.go**

Rename `ApplyPlatformDefaults(platform model.Platform)` to `ApplyLanguageDefaults(lang model.Language)`. Update the parameter name inside the function.

- [ ] **Step 2: Update defaults.go**

Rename `DefaultConfigForPlatform(platform model.Platform)` to `DefaultConfigForLanguage(lang model.Language)`. Change the switch from `model.IOS` to `model.Swift`, `model.Android` to `model.Kotlin` (default case).

Update `DefaultConfig()` to call `DefaultConfigForLanguage(model.Kotlin)`.

- [ ] **Step 3: Verify config compiles**

Run: `go build ./internal/config/`
Expected: Success (this package has no other references to Platform).

- [ ] **Step 4: Commit**

```bash
git add internal/config/
git commit -m "rename config platform methods to language"
```

---

### Task 4: Update scan engine — remove Platform indirection

**Files:**
- Modify: `internal/scan/engine.go`
- Modify: `internal/scan/engine_test.go`

- [ ] **Step 1: Update Scan function in engine.go**

1. Change `platformOverride *model.Platform` parameter to `langOverride *model.Language` in `Scan()`.
2. Replace lines 54-67:
   ```go
   // 1. Detect language
   lang := discover.DetectLanguage(path)
   if langOverride != nil {
       lang = *langOverride
   }

   // 1b. Apply language-specific defaults for placement patterns
   cfg.ApplyLanguageDefaults(lang)
   ```
   (Remove the old platform detection + language derivation — it's now direct.)
3. Update `scanResult` construction: `Platform: platform` → `Language: lang`.
4. In `BuildProjectContext` fallback (lines 81-88): remove `Platform: platform` line, keep `Language: lang`.
5. Remove `ctx.Platform = platform` (line 89), keep `ctx.Language = lang`.
6. Update `surface.DiscoverSurfaces` call: pass `lang` instead of `platform`.

- [ ] **Step 2: Update runCompareRef and buildHistoricalTrend signatures in main.go (done in Task 6)**

Note: engine.go's `Scan` signature change cascades to `cmd/confidence/main.go` — that's Task 6.

- [ ] **Step 3: Update engine_test.go**

- `TestScanEngine`: Change `result.Platform != model.Android` to `result.Language != model.Kotlin`.
- `TestScanWithPlatformOverride`: Rename to `TestScanWithLanguageOverride`. Change `ios := model.IOS` to `swift := model.Swift`, pass `&swift`, check `output.Result.Language != model.Swift`.

- [ ] **Step 4: Commit**

```bash
git add internal/scan/
git commit -m "update scan engine to use Language directly"
```

---

### Task 5: Update discover/context.go — remove Platform from ProjectContext

**Files:**
- Modify: `internal/discover/context.go`

- [ ] **Step 1: Update BuildProjectContext**

1. Line 21: `Platform: DetectPlatform(root)` → remove (Platform field no longer exists).
2. Line 22: `Language: model.Kotlin` — this is now set by the caller (engine.go), but BuildProjectContext still needs a default for the case where it's called standalone. Change to `Language: DetectLanguage(root)`.
3. Line 26: `ThirdPartyPkgs: android.ThirdPartyPkgs` — this should be language-conditional. Add:
   ```go
   lang := DetectLanguage(root)
   // ...
   ThirdPartyPkgs: thirdPartyPkgsForLanguage(lang),
   ```
   With a small helper:
   ```go
   func thirdPartyPkgsForLanguage(lang model.Language) []string {
       if lang == model.Swift {
           return ios.ThirdPartyPkgs
       }
       return android.ThirdPartyPkgs
   }
   ```
   Add `ios` import.
4. Line 39: `.kt` suffix check — for Swift projects this should be `.swift`. But since BuildProjectContext currently only does Kotlin AST scanning (@Module, @Binds), keep it Kotlin-only for now. Just skip the walk entirely if language is Swift:
   ```go
   if lang == model.Swift {
       return ctx, nil // Swift context scanning not yet implemented
   }
   ```
   Add this right before the `filepath.WalkDir` call.

- [ ] **Step 2: Commit**

```bash
git add internal/discover/context.go
git commit -m "update BuildProjectContext to use Language instead of Platform"
```

---

### Task 6: Update CLI — --platform → --lang

**Files:**
- Modify: `cmd/confidence/main.go`

- [ ] **Step 1: Update scan command flag and parsing**

1. Line 225: Change `"platform"` flag to `"lang"`:
   ```go
   scanCmd.Flags().String("lang", "", "Override language detection (kotlin|swift)")
   ```

2. Lines 43-56: Update the override parsing:
   ```go
   // Language override
   var langOverride *model.Language
   if l, _ := cmd.Flags().GetString("lang"); l != "" {
       switch l {
       case "kotlin":
           v := model.Kotlin
           langOverride = &v
       case "swift":
           v := model.Swift
           langOverride = &v
       default:
           return fmt.Errorf("unknown language: %q (use kotlin or swift)", l)
       }
   }
   ```

3. Update all `platformOverride` references to `langOverride` in the scan command.

- [ ] **Step 2: Update calibrate command identically**

Same changes for the calibrate command (lines 250-263, line 296).

- [ ] **Step 3: Update helper function signatures**

`runCompareRef` and `buildHistoricalTrend`: change `platformOverride *model.Platform` to `langOverride *model.Language`.

- [ ] **Step 4: Commit**

```bash
git add cmd/confidence/main.go
git commit -m "rename --platform flag to --lang with kotlin|swift values"
```

---

### Task 7: Update report package

**Files:**
- Modify: `internal/report/terminal.go`
- Modify: `internal/report/json.go`
- Modify: `internal/report/terminal_test.go`
- Modify: `internal/report/json_test.go`

- [ ] **Step 1: Update terminal.go**

1. Line 70: Change `platformLanguage(result.Platform)` to `result.Language.String()`.
   - Note: `Language.String()` returns "Kotlin" or "Swift". The old `platformLanguage()` returned "Android/Kotlin" or "iOS/Swift". Change the `String()` method in enums.go or keep a local helper — the simpler output ("Kotlin"/"Swift") is more accurate since this tool isn't about platforms. Use `result.Language.String()` directly.
2. Remove the `platformLanguage()` function (lines 972-981).

- [ ] **Step 2: Update json.go**

1. Line 19: Change JSON field name `"platform"` to `"language"`.
2. Line 338: Change `r.Platform.String()` to `r.Language.String()`.

- [ ] **Step 3: Update terminal_test.go**

Change `Platform: model.Kotlin` references (these were previously `Platform: model.Android` — update to use `Language: model.Kotlin`).

- [ ] **Step 4: Update json_test.go**

Same pattern — update any Platform references to Language.

- [ ] **Step 5: Commit**

```bash
git add internal/report/
git commit -m "update reports to use Language instead of Platform"
```

---

### Task 8: Update surface discovery

**Files:**
- Modify: `internal/surface/discovery.go`

- [ ] **Step 1: Update DiscoverSurfaces signature**

Change `platform ...model.Platform` to `lang ...model.Language`. Update the internal variable from `plat` checking `model.IOS` to checking `model.Swift`:

```go
// Determine language (default to Kotlin for backward compatibility)
l := model.Kotlin
if len(lang) > 0 {
    l = lang[0]
}
```

Update `if plat == model.IOS {` to `if l == model.Swift {`.

- [ ] **Step 2: Commit**

```bash
git add internal/surface/discovery.go
git commit -m "update surface discovery to use Language"
```

---

### Task 9: Update placement_test.go

**Files:**
- Modify: `internal/analyze/placement_test.go`

- [ ] **Step 1: Update any Platform references**

Check for `Platform` field usage in test fixtures and update to `Language`.

- [ ] **Step 2: Commit**

```bash
git add internal/analyze/placement_test.go
git commit -m "update placement tests to use Language"
```

---

### Task 10: Full build + test verification

- [ ] **Step 1: Build**

Run: `go build ./...`
Expected: Clean build, no errors.

- [ ] **Step 2: Run all tests**

Run: `go test ./...`
Expected: All tests pass.

- [ ] **Step 3: Verify CLI flag works**

Run: `go run ./cmd/confidence scan --help`
Expected: Shows `--lang` flag with `kotlin|swift` description. No `--platform` flag.

- [ ] **Step 4: Update CLAUDE.md usage examples**

Change `--platform ios` to `--lang swift` in the Usage section.

- [ ] **Step 5: Final commit**

```bash
git add CLAUDE.md
git commit -m "update CLAUDE.md usage for --lang flag"
```
