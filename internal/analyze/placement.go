package analyze

import (
	"strings"

	"github.com/timusus/test-confidence/internal/config"
	"github.com/timusus/test-confidence/internal/model"
	"github.com/timusus/test-confidence/internal/platform/android"
)

// testDoublePrefixes are common prefixes used by mocking frameworks that should
// be stripped before applying type name heuristics. e.g. MockDeeplinkHelper -> DeeplinkHelper.
var testDoublePrefixes = []string{"Mock", "Stub", "Fake", "Test", "Spy"}

// testDoubleSuffixes are common suffixes (e.g. Swift Protocol suffix from Mockable/Sourcery)
// that should be stripped before applying type name heuristics.
var testDoubleSuffixes = []string{"Protocol", "Mock", "Stub", "Fake", "Spy"}

// stripTestDoubleAffixes removes common mock framework prefixes and suffixes
// to recover the underlying type name for classification.
// e.g. "MockDeeplinkHelperProtocol" -> "DeeplinkHelper"
func stripTestDoubleAffixes(typeName string) string {
	name := typeName
	// Strip prefix (only one).
	for _, prefix := range testDoublePrefixes {
		if strings.HasPrefix(name, prefix) && len(name) > len(prefix) {
			nextChar := name[len(prefix)]
			if nextChar >= 'A' && nextChar <= 'Z' {
				name = name[len(prefix):]
				break
			}
		}
	}
	// Strip suffix (only one).
	for _, suffix := range testDoubleSuffixes {
		if strings.HasSuffix(name, suffix) && len(name) > len(suffix) {
			name = name[:len(name)-len(suffix)]
			break
		}
	}
	return name
}

// ClassifyPlacement determines whether a test double sits at a boundary or
// internal layer. It checks signals in priority order and returns the first
// definitive match along with all signals evaluated (for transparency).
func ClassifyPlacement(double model.TestDouble, ctx *model.ProjectContext, cfg config.Config) (model.Placement, []model.PlacementSignal) {
	var signals []model.PlacementSignal

	// 1. Import origin: look up type in TypePackages, check against ThirdPartyPkgs.
	pkg := ctx.TypePackages[double.TypeName]
	if pkg != "" {
		matched := false
		for _, tp := range ctx.ThirdPartyPkgs {
			if strings.HasPrefix(pkg, tp) {
				signals = append(signals, model.PlacementSignal{
					Signal: "import_origin",
					Value:  pkg + " matches third-party " + tp,
					Result: model.Boundary,
				})
				return model.Boundary, signals
			}
		}
		if !matched {
			signals = append(signals, model.PlacementSignal{
				Signal: "import_origin",
				Value:  pkg + " no third-party match",
				Result: model.UnknownPlacement,
			})
		}
	} else {
		signals = append(signals, model.PlacementSignal{
			Signal: "import_origin",
			Value:  double.TypeName + " not in TypePackages",
			Result: model.UnknownPlacement,
		})
	}

	// 2. DI module binding.
	if ctx.DIBoundaryTypes[double.TypeName] {
		signals = append(signals, model.PlacementSignal{
			Signal: "di_module",
			Value:  double.TypeName + " found in DIBoundaryTypes",
			Result: model.Boundary,
		})
		return model.Boundary, signals
	}
	signals = append(signals, model.PlacementSignal{
		Signal: "di_module",
		Value:  double.TypeName + " not in DIBoundaryTypes",
		Result: model.UnknownPlacement,
	})

	// 3. Package path: match package against boundary/internal package patterns.
	if pkg != "" {
		if placement, ok := matchPackagePath(pkg, cfg); ok {
			signals = append(signals, model.PlacementSignal{
				Signal: "package_path",
				Value:  pkg,
				Result: placement,
			})
			return placement, signals
		}
	}
	signals = append(signals, model.PlacementSignal{
		Signal: "package_path",
		Value:  "no package match for " + double.TypeName,
		Result: model.UnknownPlacement,
	})

	// 4. Type name pattern matching + Repository special case.
	// Strip mock framework prefixes/suffixes to recover the underlying type name.
	strippedName := stripTestDoubleAffixes(double.TypeName)
	if placement, ok := matchTypeName(strippedName, cfg); ok {
		signals = append(signals, model.PlacementSignal{
			Signal: "type_name",
			Value:  double.TypeName + " (as " + strippedName + ")",
			Result: placement,
		})
		return placement, signals
	}
	signals = append(signals, model.PlacementSignal{
		Signal: "type_name",
		Value:  double.TypeName + " no pattern match",
		Result: model.UnknownPlacement,
	})

	// 4b. Verb prefix: use-case-style names like GetUser, UpdateProfile, etc.
	// Check stripped name so MockGetUser also matches.
	if prefix := matchVerbPrefix(strippedName); prefix != "" {
		signals = append(signals, model.PlacementSignal{
			Signal: "verb_prefix",
			Value:  double.TypeName + " matches verb prefix " + prefix,
			Result: model.Internal,
		})
		return model.Internal, signals
	}

	// 5. Stubbing pattern: usage-based hint.
	switch double.Usage {
	case model.SetupOnly:
		signals = append(signals, model.PlacementSignal{
			Signal: "stubbing_pattern",
			Value:  "setup-only leans boundary",
			Result: model.Boundary,
		})
		return model.Boundary, signals
	case model.Verification:
		signals = append(signals, model.PlacementSignal{
			Signal: "stubbing_pattern",
			Value:  "verification-only leans internal",
			Result: model.Internal,
		})
		return model.Internal, signals
	default:
		signals = append(signals, model.PlacementSignal{
			Signal: "stubbing_pattern",
			Value:  "usage " + double.Usage.String() + " gives no hint",
			Result: model.UnknownPlacement,
		})
	}

	return model.UnknownPlacement, signals
}

// matchPackagePath checks a package path against configured boundary and
// internal package patterns (substring match).
func matchPackagePath(pkg string, cfg config.Config) (model.Placement, bool) {
	for _, bp := range cfg.Placement.BoundaryPackages {
		if strings.Contains(pkg, strings.TrimSuffix(bp, "/")) {
			return model.Boundary, true
		}
	}
	for _, ip := range cfg.Placement.InternalPackages {
		if strings.Contains(pkg, strings.TrimSuffix(ip, "/")) {
			return model.Internal, true
		}
	}
	return model.UnknownPlacement, false
}

// matchTypeName checks a type name against configured boundary and internal
// patterns (suffix wildcard with * prefix) and the Repository special case.
func matchTypeName(typeName string, cfg config.Config) (model.Placement, bool) {
	for _, pattern := range cfg.Placement.Boundary {
		if matchWildcard(typeName, pattern) {
			return model.Boundary, true
		}
	}
	for _, pattern := range cfg.Placement.Internal {
		if matchWildcard(typeName, pattern) {
			return model.Internal, true
		}
	}
	// Repository special case.
	if strings.Contains(typeName, "Repository") {
		if cfg.Placement.Repository == "internal" {
			return model.Internal, true
		}
		return model.Boundary, true
	}
	return model.UnknownPlacement, false
}

// matchVerbPrefix checks if a type name starts with a use-case verb prefix
// (e.g., GetUser, UpdateProfile, RemoveItem). Returns the matched prefix or "".
func matchVerbPrefix(typeName string) string {
	for _, prefix := range android.VerbPrefixes {
		if strings.HasPrefix(typeName, prefix) && len(typeName) > len(prefix) {
			// Ensure the character after the prefix is uppercase (avoids matching "Getting" or "Setback")
			nextChar := typeName[len(prefix)]
			if nextChar >= 'A' && nextChar <= 'Z' {
				return prefix
			}
		}
	}
	return ""
}

// matchWildcard handles patterns like "*Client" (suffix match) or exact match.
func matchWildcard(name, pattern string) bool {
	if strings.HasPrefix(pattern, "*") {
		suffix := pattern[1:]
		return strings.HasSuffix(name, suffix)
	}
	return name == pattern
}
