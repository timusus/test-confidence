package config

import (
	"os"

	"github.com/timusus/test-confidence/internal/model"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Placement  PlacementConfig `yaml:"placement"`
	Thresholds ThresholdConfig `yaml:"thresholds"`
	Exclude    []string        `yaml:"exclude"`

	// placementOverrides tracks which placement fields were explicitly set by the user.
	// Fields not in this set will be replaced when ApplyLanguageDefaults is called.
	placementOverrides map[string]bool
}

type PlacementConfig struct {
	Boundary         []string `yaml:"boundary"`
	Internal         []string `yaml:"internal"`
	Repository       string   `yaml:"repository"`
	BoundaryPackages []string `yaml:"boundary_packages"`
	InternalPackages []string `yaml:"internal_packages"`
}

type ThresholdConfig struct {
	GodTestClass      int `yaml:"god_test_class"`
	AssertionRoulette int `yaml:"assertion_roulette"`
}

// ApplyLanguageDefaults replaces default placement values with language-specific
// defaults for any field that was not explicitly overridden by the user config.
func (c *Config) ApplyLanguageDefaults(lang model.Language) {
	platCfg := DefaultConfigForLanguage(lang)

	if !c.placementOverrides["boundary"] {
		c.Placement.Boundary = platCfg.Placement.Boundary
	}
	if !c.placementOverrides["internal"] {
		c.Placement.Internal = platCfg.Placement.Internal
	}
	if !c.placementOverrides["boundary_packages"] {
		c.Placement.BoundaryPackages = platCfg.Placement.BoundaryPackages
	}
	if !c.placementOverrides["internal_packages"] {
		c.Placement.InternalPackages = platCfg.Placement.InternalPackages
	}
}

// LoadConfig loads configuration from the given path, merging with defaults
// using replace-per-key semantics. If path is empty or the file doesn't exist,
// returns DefaultConfig().
func LoadConfig(path string) Config {
	defaults := DefaultConfig()

	if path == "" {
		return defaults
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return defaults
	}

	// Parse into a raw map to detect which keys were actually provided.
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return defaults
	}

	// Parse into a typed override struct.
	var override Config
	if err := yaml.Unmarshal(data, &override); err != nil {
		return defaults
	}

	// Track which placement fields were user-specified.
	overrides := map[string]bool{}

	// Merge placement fields with replace-per-key semantics.
	if placementRaw, ok := raw["placement"].(map[string]interface{}); ok {
		if _, ok := placementRaw["boundary"]; ok {
			defaults.Placement.Boundary = override.Placement.Boundary
			overrides["boundary"] = true
		}
		if _, ok := placementRaw["internal"]; ok {
			defaults.Placement.Internal = override.Placement.Internal
			overrides["internal"] = true
		}
		if _, ok := placementRaw["repository"]; ok {
			defaults.Placement.Repository = override.Placement.Repository
			overrides["repository"] = true
		}
		if _, ok := placementRaw["boundary_packages"]; ok {
			defaults.Placement.BoundaryPackages = override.Placement.BoundaryPackages
			overrides["boundary_packages"] = true
		}
		if _, ok := placementRaw["internal_packages"]; ok {
			defaults.Placement.InternalPackages = override.Placement.InternalPackages
			overrides["internal_packages"] = true
		}
	}

	defaults.placementOverrides = overrides

	// Merge thresholds: non-zero values override defaults.
	if override.Thresholds.GodTestClass != 0 {
		defaults.Thresholds.GodTestClass = override.Thresholds.GodTestClass
	}
	if override.Thresholds.AssertionRoulette != 0 {
		defaults.Thresholds.AssertionRoulette = override.Thresholds.AssertionRoulette
	}

	// Merge exclude: replace if provided.
	if _, ok := raw["exclude"]; ok {
		defaults.Exclude = override.Exclude
	}

	return defaults
}
