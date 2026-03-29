package config

import (
	"github.com/timusus/test-confidence/internal/model"
	"github.com/timusus/test-confidence/internal/platform/android"
	"github.com/timusus/test-confidence/internal/platform/ios"
)

func DefaultConfig() Config {
	return DefaultConfigForLanguage(model.Kotlin)
}

// DefaultConfigForLanguage returns the default configuration with
// language-specific placement patterns.
func DefaultConfigForLanguage(lang model.Language) Config {
	cfg := Config{
		Placement: PlacementConfig{
			Repository: "boundary",
		},
		Thresholds: ThresholdConfig{
			GodTestClass:      20,
			AssertionRoulette: 10,
		},
		Exclude: []string{"**/generated/**", "**/build/**", "**/buildSrc/**"},
	}

	switch lang {
	case model.Swift:
		cfg.Placement.Boundary = ios.BoundaryTypeDefaults
		cfg.Placement.Internal = ios.InternalTypeDefaults
		cfg.Placement.BoundaryPackages = ios.BoundaryPackageDefaults
		cfg.Placement.InternalPackages = ios.InternalPackageDefaults
	default:
		cfg.Placement.Boundary = android.BoundaryTypeDefaults
		cfg.Placement.Internal = android.InternalTypeDefaults
		cfg.Placement.BoundaryPackages = android.BoundaryPackageDefaults
		cfg.Placement.InternalPackages = android.InternalPackageDefaults
	}

	return cfg
}
