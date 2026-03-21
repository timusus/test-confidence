package analyze

import (
	"testing"

	"github.com/timusus/test-confidence/internal/model"
)

func TestClassifyScope_Snapshot(t *testing.T) {
	parsed := mustParseFixture(t, "kotlin_scope_snapshot.kt")
	defer parsed.Close()

	info := ClassifyScope(parsed)

	if info.Scope != model.ScopeSnapshot {
		t.Errorf("expected ScopeSnapshot, got %v", info.Scope)
	}
}

func TestClassifyScope_Device(t *testing.T) {
	parsed := mustParseFixture(t, "kotlin_scope_device.kt")
	defer parsed.Close()

	info := ClassifyScope(parsed)

	if info.Scope != model.ScopeDevice {
		t.Errorf("expected ScopeDevice, got %v", info.Scope)
	}
}

func TestClassifyScope_DeviceByPath(t *testing.T) {
	// Simulate a file in androidTest directory
	parsed := mustParseFixture(t, "kotlin_scope_local.kt")
	defer parsed.Close()

	// Override the path to simulate androidTest location
	parsed.Path = "src/androidTest/kotlin/com/example/UserFlowTest.kt"

	info := ClassifyScope(parsed)

	if info.Scope != model.ScopeDevice {
		t.Errorf("expected ScopeDevice for androidTest path, got %v", info.Scope)
	}
}

func TestClassifyScope_Local(t *testing.T) {
	parsed := mustParseFixture(t, "kotlin_scope_local.kt")
	defer parsed.Close()

	info := ClassifyScope(parsed)

	if info.Scope != model.ScopeLocal {
		t.Errorf("expected ScopeLocal, got %v", info.Scope)
	}
	if info.IsRobolectric {
		t.Error("expected IsRobolectric=false for plain local test")
	}
	if info.IsComposeTest {
		t.Error("expected IsComposeTest=false for plain local test")
	}
	if info.IsRoomTest {
		t.Error("expected IsRoomTest=false for plain local test")
	}
}

func TestClassifyScope_Robolectric(t *testing.T) {
	parsed := mustParseFixture(t, "kotlin_scope_robolectric.kt")
	defer parsed.Close()

	info := ClassifyScope(parsed)

	if info.Scope != model.ScopeLocal {
		t.Errorf("expected ScopeLocal (Robolectric is local), got %v", info.Scope)
	}
	if !info.IsRobolectric {
		t.Error("expected IsRobolectric=true")
	}
}

func TestClassifyScope_ComposeTest(t *testing.T) {
	parsed := mustParseFixture(t, "kotlin_scope_compose.kt")
	defer parsed.Close()

	info := ClassifyScope(parsed)

	if info.Scope != model.ScopeLocal {
		t.Errorf("expected ScopeLocal (ComposeTestRule is local), got %v", info.Scope)
	}
	if !info.IsComposeTest {
		t.Error("expected IsComposeTest=true")
	}
}

func TestClassifyScope_RoomTest(t *testing.T) {
	parsed := mustParseFixture(t, "kotlin_scope_room.kt")
	defer parsed.Close()

	info := ClassifyScope(parsed)

	if info.Scope != model.ScopeLocal {
		t.Errorf("expected ScopeLocal (Room in-memory is local), got %v", info.Scope)
	}
	if !info.IsRoomTest {
		t.Error("expected IsRoomTest=true")
	}
}

func TestClassifyScope_SnapshotOverridesDevice(t *testing.T) {
	// A file with both snapshot and device imports should be classified as snapshot
	// (snapshot has higher priority in the cascade)
	parsed := mustParseFixture(t, "kotlin_scope_snapshot.kt")
	defer parsed.Close()

	// Even if path contains androidTest, snapshot wins
	parsed.Path = "src/androidTest/kotlin/com/example/SnapshotTest.kt"

	info := ClassifyScope(parsed)

	if info.Scope != model.ScopeSnapshot {
		t.Errorf("expected ScopeSnapshot to override device path, got %v", info.Scope)
	}
}

func TestTestScope_String(t *testing.T) {
	tests := []struct {
		scope    model.TestScope
		expected string
	}{
		{model.ScopeLocal, "Local"},
		{model.ScopeDevice, "Device"},
		{model.ScopeSnapshot, "Snapshot"},
	}
	for _, tt := range tests {
		if got := tt.scope.String(); got != tt.expected {
			t.Errorf("TestScope(%d).String() = %q, want %q", tt.scope, got, tt.expected)
		}
	}
}
