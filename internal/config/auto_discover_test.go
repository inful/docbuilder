package config

import "testing"

// TestAutoDiscoverValidation ensures that empty organizations/groups are only allowed when options.auto_discover=true.
func TestAutoDiscoverValidation(t *testing.T) {
	base := Config{
		Version: "2.0",
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

// TestAutoDiscoverValidationWithTestForgeFactory demonstrates enhanced validation testing with realistic forge data.
func TestAutoDiscoverValidationWithTestForgeFactory(t *testing.T) {
	factory := NewTestForgeConfigFactory()

	t.Run("AutoDiscoverEnabled", func(t *testing.T) {
		// Test auto-discovery with realistic forge configurations across platforms
		platforms := []ForgeType{ForgeGitHub, ForgeGitLab, ForgeForgejo}

		for _, platform := range platforms {
			forge := factory.CreateForgeWithAutoDiscover(platform, "auto-test")
			config := factory.CreateConfigWithForges([]*ForgeConfig{forge})

			if err := validateConfig(config); err != nil {
				t.Errorf("Auto-discovery forge %s should be valid: %v", platform, err)
			}

			// Validate that auto-discovery fields are properly set
			if len(forge.Organizations) > 0 || len(forge.Groups) > 0 {
				t.Errorf("Auto-discovery forge %s should have empty organizations/groups", platform)
			}
			if !forge.AutoDiscover {
				t.Errorf("Auto-discovery forge %s should have AutoDiscover=true", platform)
			}
		}
	})

	t.Run("AutoDiscoverDisabledWithScopes", func(t *testing.T) {
		// Test forges with explicit organizations/groups (should be valid)
		platforms := []ForgeType{ForgeGitHub, ForgeGitLab, ForgeForgejo}

		for _, platform := range platforms {
			var forge *ForgeConfig
			switch platform {
			case ForgeGitHub:
				forge = factory.CreateGitHubForge("scoped-test")
			case ForgeGitLab:
				forge = factory.CreateGitLabForge("scoped-test")
			case ForgeForgejo:
				forge = factory.CreateForgejoForge("scoped-test")
			}

			config := factory.CreateConfigWithForges([]*ForgeConfig{forge})

			if err := validateConfig(config); err != nil {
				t.Errorf("Forge %s with explicit scopes should be valid: %v", platform, err)
			}

			// Validate that scopes are properly set
			hasScopes := len(forge.Organizations) > 0 || len(forge.Groups) > 0
			if !hasScopes {
				t.Errorf("Forge %s should have organizations or groups defined", platform)
			}
		}
	})

	t.Run("AutoDiscoverOptionsHandling", func(t *testing.T) {
		// Test auto_discover option handling
		testCases := []struct {
			name      string
			options   map[string]any
			shouldErr bool
		}{
			{"auto_discover_true", map[string]any{"auto_discover": true}, false},
			{"auto_discover_false", map[string]any{"auto_discover": false}, true},
			{"auto_discover_missing", map[string]any{}, true},
			{"other_options", map[string]any{"rate_limit": 5000, "timeout": "30s"}, true},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				forge := factory.CreateForgeWithOptions(ForgeGitHub, "options-test", tc.options)

				// Remove organizations for options testing
				if _, hasAutoDiscover := tc.options["auto_discover"]; !hasAutoDiscover || tc.options["auto_discover"] == false {
					forge.Organizations = nil
					forge.Groups = nil
					forge.AutoDiscover = false
				}

				config := factory.CreateConfigWithForges([]*ForgeConfig{forge})
				err := validateConfig(config)

				if tc.shouldErr && err == nil {
					t.Errorf("Expected validation error for options %v", tc.options)
				}
				if !tc.shouldErr && err != nil {
					t.Errorf("Unexpected validation error for options %v: %v", tc.options, err)
				}
			})
		}
	})

	t.Run("MultiPlatformAutoDiscover", func(t *testing.T) {
		// Test multi-platform configuration with mixed auto-discovery settings
		githubForge := factory.CreateForgeWithAutoDiscover(ForgeGitHub, "multi-github")
		gitlabForge := factory.CreateGitLabForge("multi-gitlab") // With explicit groups
		forgejoForge := factory.CreateForgeWithAutoDiscover(ForgeForgejo, "multi-forgejo")

		config := factory.CreateConfigWithForges([]*ForgeConfig{githubForge, gitlabForge, forgejoForge})

		if err := validateConfig(config); err != nil {
			t.Errorf("Multi-platform config with mixed auto-discovery should be valid: %v", err)
		}

		// Validate each forge has correct settings
		if !githubForge.AutoDiscover || len(githubForge.Organizations) > 0 {
			t.Error("GitHub forge should have auto-discovery enabled and no organizations")
		}
		if len(gitlabForge.Groups) == 0 || gitlabForge.AutoDiscover {
			t.Error("GitLab forge should have explicit groups and no auto-discovery")
		}
		if !forgejoForge.AutoDiscover || len(forgejoForge.Organizations) > 0 {
			t.Error("Forgejo forge should have auto-discovery enabled and no organizations")
		}

		t.Logf("✓ Multi-platform validation: GitHub(auto=%v), GitLab(groups=%d), Forgejo(auto=%v)",
			githubForge.AutoDiscover, len(gitlabForge.Groups), forgejoForge.AutoDiscover)
	})
}
