package config

import "testing"

// TestAutoDiscoverValidation ensures that empty organizations/groups are only allowed when options.auto_discover=true.
func TestAutoDiscoverValidation(t *testing.T) {
	base := Config{
		Version: "2.0",
		Hugo:    HugoConfig{Title: "t", Theme: string(ThemeHextra)},
		Output:  OutputConfig{Directory: "./out", Clean: true},
		Build:   BuildConfig{CloneConcurrency: 1, MaxRetries: 1, RetryBackoff: RetryBackoffLinear, RetryInitialDelay: "1s", RetryMaxDelay: "2s", CloneStrategy: CloneStrategyFresh},
		Forges:  []*ForgeConfig{},
	}

	withForge := func(fc *ForgeConfig) *Config { c := base; c.Forges = []*ForgeConfig{fc}; return &c }

	forgeNoScopes := &ForgeConfig{Name: "f1", Type: ForgeGitHub, Auth: &AuthConfig{Type: AuthTypeToken, Token: "x"}}
	if err := validateConfig(withForge(forgeNoScopes)); err == nil {
		t.Fatalf("expected error when no org/group and auto_discover unset")
	}

	forgeAuto := &ForgeConfig{Name: "f2", Type: ForgeGitHub, Auth: &AuthConfig{Type: AuthTypeToken, Token: "x"}, Options: map[string]any{"auto_discover": true}}
	if err := validateConfig(withForge(forgeAuto)); err != nil {
		t.Fatalf("unexpected error with options.auto_discover=true: %v", err)
	}

	forgeAutoTop := &ForgeConfig{Name: "f4", Type: ForgeGitHub, Auth: &AuthConfig{Type: AuthTypeToken, Token: "x"}, AutoDiscover: true}
	if err := validateConfig(withForge(forgeAutoTop)); err != nil {
		t.Fatalf("unexpected error with top-level auto_discover: %v", err)
	}

	forgeFalse := &ForgeConfig{Name: "f3", Type: ForgeGitHub, Auth: &AuthConfig{Type: AuthTypeToken, Token: "x"}, Options: map[string]any{"auto_discover": false}}
	if err := validateConfig(withForge(forgeFalse)); err == nil {
		t.Fatalf("expected error with auto_discover=false")
	}
}
