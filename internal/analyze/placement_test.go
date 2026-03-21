package analyze

import (
	"testing"

	"github.com/timusus/test-confidence/internal/config"
	"github.com/timusus/test-confidence/internal/model"
)

func TestPlacementAnalyzer(t *testing.T) {
	cfg := config.DefaultConfig()
	ctx := &model.ProjectContext{
		DIBoundaryTypes: map[string]bool{"UserRepository": true},
		TypePackages: map[string]string{
			"ApiClient":   "com.example.network",
			"UserUseCase": "com.example.domain",
			"RetrofitApi": "retrofit2.api",
		},
		ThirdPartyPkgs: []string{"retrofit2", "okhttp3", "androidx.room"},
	}

	tests := []struct {
		name     string
		double   model.TestDouble
		expected model.Placement
	}{
		{
			name:     "third-party import → boundary",
			double:   model.TestDouble{TypeName: "RetrofitApi", Usage: model.SetupOnly},
			expected: model.Boundary, // "retrofit2.api" starts with "retrofit2"
		},
		{
			name:     "DI module boundary",
			double:   model.TestDouble{TypeName: "UserRepository", Usage: model.Both},
			expected: model.Boundary,
		},
		{
			name:     "package path → boundary",
			double:   model.TestDouble{TypeName: "ApiClient", Usage: model.SetupOnly},
			expected: model.Boundary, // "com.example.network" contains "network/"
		},
		{
			name:     "package path → internal",
			double:   model.TestDouble{TypeName: "UserUseCase", Usage: model.SetupOnly},
			expected: model.Internal, // "com.example.domain" contains "domain/"
		},
		{
			name:     "type name pattern → boundary",
			double:   model.TestDouble{TypeName: "PaymentClient", Usage: model.SetupOnly},
			expected: model.Boundary, // matches "*Client"
		},
		{
			name:     "type name pattern → internal",
			double:   model.TestDouble{TypeName: "OrderViewModel", Usage: model.SetupOnly},
			expected: model.Internal, // matches "*ViewModel"
		},
		{
			name:     "repository special case → boundary",
			double:   model.TestDouble{TypeName: "OrderRepository", Usage: model.SetupOnly},
			expected: model.Boundary, // cfg.Placement.Repository == "boundary"
		},
		{
			name:     "stubbing pattern → boundary hint",
			double:   model.TestDouble{TypeName: "SomeHelper", Usage: model.SetupOnly},
			expected: model.Boundary, // setup-only leans boundary
		},
		{
			name:     "verification usage → internal hint",
			double:   model.TestDouble{TypeName: "SomeService", Usage: model.Verification},
			expected: model.Internal, // verification-only leans internal
		},
		{
			name:     "no signals → unknown",
			double:   model.TestDouble{TypeName: "SomeThing", Usage: model.Both},
			expected: model.UnknownPlacement,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			placement, signals := ClassifyPlacement(tt.double, ctx, cfg)
			if placement != tt.expected {
				t.Errorf("expected %v, got %v (signals: %v)", tt.expected, placement, signals)
			}
		})
	}
}

func TestPlacementSignalsRecorded(t *testing.T) {
	cfg := config.DefaultConfig()
	ctx := &model.ProjectContext{
		DIBoundaryTypes: map[string]bool{},
		TypePackages:    map[string]string{},
		ThirdPartyPkgs:  []string{},
	}

	double := model.TestDouble{TypeName: "SomeThing", Usage: model.Both}
	_, signals := ClassifyPlacement(double, ctx, cfg)

	if len(signals) == 0 {
		t.Error("expected signals to be recorded even when no match is found")
	}

	// Check that all signal types are present
	signalTypes := map[string]bool{}
	for _, s := range signals {
		signalTypes[s.Signal] = true
	}

	expectedSignals := []string{"import_origin", "di_module", "package_path", "type_name", "stubbing_pattern"}
	for _, expected := range expectedSignals {
		if !signalTypes[expected] {
			t.Errorf("missing signal type: %s", expected)
		}
	}
}

func TestStripTestDoubleAffixes(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"MockDeeplinkHelper", "DeeplinkHelper"},
		{"MockDeeplinkHelperProtocol", "DeeplinkHelper"},
		{"StubApiClient", "ApiClient"},
		{"FakeUserRepository", "UserRepository"},
		{"TestNetworkManager", "NetworkManager"},
		{"SpyAnalyticsTracker", "AnalyticsTracker"},
		{"UserViewModel", "UserViewModel"},
		{"Mocker", "Mocker"},
		{"MockApplicationConfigurationStore", "ApplicationConfigurationStore"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := stripTestDoubleAffixes(tt.input)
			if result != tt.expected {
				t.Errorf("stripTestDoubleAffixes(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestPlacementMockPrefixStripping(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.ApplyPlatformDefaults(model.IOS)
	ctx := &model.ProjectContext{
		DIBoundaryTypes: map[string]bool{},
		TypePackages:    map[string]string{},
		ThirdPartyPkgs:  []string{},
	}

	tests := []struct {
		name     string
		double   model.TestDouble
		expected model.Placement
	}{
		{
			name:     "MockDeeplinkHelper is internal (Helper pattern)",
			double:   model.TestDouble{TypeName: "MockDeeplinkHelper", Usage: model.Both},
			expected: model.Internal,
		},
		{
			name:     "MockApplicationConfigurationStore is internal (Store pattern)",
			double:   model.TestDouble{TypeName: "MockApplicationConfigurationStore", Usage: model.Both},
			expected: model.Internal,
		},
		{
			name:     "MockDeeplinkHelperProtocol is internal (Helper after stripping)",
			double:   model.TestDouble{TypeName: "MockDeeplinkHelperProtocol", Usage: model.Both},
			expected: model.Internal,
		},
		{
			name:     "MockApiClient is boundary (Client pattern)",
			double:   model.TestDouble{TypeName: "MockApiClient", Usage: model.Both},
			expected: model.Boundary,
		},
		{
			name:     "MockNetworkCoordinator is internal (Coordinator pattern)",
			double:   model.TestDouble{TypeName: "MockNetworkCoordinator", Usage: model.Both},
			expected: model.Internal,
		},
		{
			name:     "MockUserPresenter is internal (Presenter pattern)",
			double:   model.TestDouble{TypeName: "MockUserPresenter", Usage: model.Both},
			expected: model.Internal,
		},
		{
			name:     "MockAuthRouter is internal (Router pattern)",
			double:   model.TestDouble{TypeName: "MockAuthRouter", Usage: model.Both},
			expected: model.Internal,
		},
		{
			name:     "MockNavigator is internal (Navigator pattern)",
			double:   model.TestDouble{TypeName: "MockNavigator", Usage: model.Both},
			expected: model.Internal,
		},
		{
			name:     "MockEventHandler is internal (Handler pattern)",
			double:   model.TestDouble{TypeName: "MockEventHandler", Usage: model.Both},
			expected: model.Internal,
		},
		{
			name:     "MockAuthService is internal (Service pattern, iOS)",
			double:   model.TestDouble{TypeName: "MockAuthService", Usage: model.Both},
			expected: model.Internal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			placement, signals := ClassifyPlacement(tt.double, ctx, cfg)
			if placement != tt.expected {
				t.Errorf("expected %v, got %v (signals: %v)", tt.expected, placement, signals)
			}
		})
	}
}
