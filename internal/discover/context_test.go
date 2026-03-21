package discover

import (
	"testing"

	"github.com/timusus/test-confidence/internal/config"
)

func TestBuildProjectContext(t *testing.T) {
	cfg := config.DefaultConfig()
	ctx, err := BuildProjectContext("../../testdata/projects/android-simple", cfg)
	if err != nil {
		t.Fatal(err)
	}

	// DI boundary types from @Binds
	if !ctx.DIBoundaryTypes["UserRepository"] {
		t.Error("expected UserRepository as DI boundary type")
	}
	if !ctx.DIBoundaryTypes["OrderRepository"] {
		t.Error("expected OrderRepository as DI boundary type")
	}

	// Fake types from class names starting with Fake
	if !ctx.FakeTypes["Database"] {
		t.Error("expected Database in FakeTypes (from FakeDatabase class)")
	}

	// TypePackages should have entries from imports
	if len(ctx.TypePackages) == 0 {
		t.Error("expected type-to-package mappings")
	}

	// ThirdPartyPkgs should be populated from config/defaults
	if len(ctx.ThirdPartyPkgs) == 0 {
		t.Error("expected third-party package prefixes")
	}
}
