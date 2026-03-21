package config

import (
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := LoadConfig("") // no file

	if len(cfg.Placement.Boundary) == 0 {
		t.Error("expected default boundary patterns")
	}
	if len(cfg.Placement.Internal) == 0 {
		t.Error("expected default internal patterns")
	}
	if cfg.Thresholds.GodTestClass != 20 {
		t.Errorf("expected god_test_class=20, got %d", cfg.Thresholds.GodTestClass)
	}
	if cfg.Thresholds.AssertionRoulette != 10 {
		t.Errorf("expected assertion_roulette=10, got %d", cfg.Thresholds.AssertionRoulette)
	}
	if cfg.Placement.Repository != "boundary" {
		t.Errorf("expected repository=boundary, got %s", cfg.Placement.Repository)
	}
	if len(cfg.Exclude) == 0 {
		t.Error("expected default exclude patterns")
	}
}

func TestConfigOverride(t *testing.T) {
	cfg := LoadConfig("../../testdata/fixtures/test-confidence.yaml")

	// boundary list replaced entirely, not appended
	if len(cfg.Placement.Boundary) != 2 {
		t.Errorf("expected 2 boundary patterns (replaced), got %d", len(cfg.Placement.Boundary))
	}

	// internal list still defaults (not overridden)
	if len(cfg.Placement.Internal) == 0 {
		t.Error("expected default internal patterns to remain")
	}

	// threshold overridden
	if cfg.Thresholds.GodTestClass != 30 {
		t.Errorf("expected overridden threshold 30, got %d", cfg.Thresholds.GodTestClass)
	}

	// assertion_roulette keeps default (not in override file)
	if cfg.Thresholds.AssertionRoulette != 10 {
		t.Errorf("expected default assertion_roulette=10, got %d", cfg.Thresholds.AssertionRoulette)
	}
}

func TestConfigMissingFile(t *testing.T) {
	cfg := LoadConfig("/nonexistent/path.yaml")
	// Should return defaults, not error
	if cfg.Thresholds.GodTestClass != 20 {
		t.Error("expected defaults for missing file")
	}
}
