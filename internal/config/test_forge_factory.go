package config

import (
	"fmt"
)

// TestForgeConfigFactory provides realistic forge configurations for testing
type TestForgeConfigFactory struct {
	counter int
}

// NewTestForgeConfigFactory creates a factory for generating test forge configurations
func NewTestForgeConfigFactory() *TestForgeConfigFactory {
	return &TestForgeConfigFactory{counter: 0}
}

// CreateGitHubForge creates a realistic GitHub forge configuration
func (f *TestForgeConfigFactory) CreateGitHubForge(name string) *ForgeConfig {
	f.counter++
	return &ForgeConfig{
		Name:    fmt.Sprintf("%s-github-%d", name, f.counter),
		Type:    ForgeGitHub,
		BaseURL: "https://github.com",
		APIURL:  "https://api.github.com",
		Organizations: []string{
			fmt.Sprintf("%s-org", name),
			fmt.Sprintf("%s-enterprise", name),
		},
		Auth: &AuthConfig{
			Type:  AuthTypeToken,
			Token: fmt.Sprintf("ghp_test_token_%s_%d", name, f.counter),
		},
		Webhook: &WebhookConfig{
			Secret: fmt.Sprintf("%s-webhook-secret-%d", name, f.counter),
			Path:   fmt.Sprintf("/webhooks/%s", name),
			Events: []string{"push", "repository", "organization"},
		},
	}
}

// CreateGitLabForge creates a realistic GitLab forge configuration
func (f *TestForgeConfigFactory) CreateGitLabForge(name string) *ForgeConfig {
	f.counter++
	return &ForgeConfig{
		Name:    fmt.Sprintf("%s-gitlab-%d", name, f.counter),
		Type:    ForgeGitLab,
		BaseURL: "https://gitlab.com",
		APIURL:  "https://gitlab.com/api/v4",
		Groups: []string{
			fmt.Sprintf("%s-group", name),
			fmt.Sprintf("%s-enterprise-group", name),
		},
		Auth: &AuthConfig{
			Type:  AuthTypeToken,
			Token: fmt.Sprintf("glpat-test-token-%s-%d", name, f.counter),
		},
		Webhook: &WebhookConfig{
			Secret: fmt.Sprintf("%s-gitlab-webhook-%d", name, f.counter),
			Path:   fmt.Sprintf("/webhooks/gitlab/%s", name),
			Events: []string{"push", "merge_request", "repository"},
		},
	}
}

// CreateForgejoForge creates a realistic Forgejo forge configuration
func (f *TestForgeConfigFactory) CreateForgejoForge(name string) *ForgeConfig {
	f.counter++
	return &ForgeConfig{
		Name:    fmt.Sprintf("%s-forgejo-%d", name, f.counter),
		Type:    ForgeForgejo,
		BaseURL: fmt.Sprintf("https://%s-forge.example.com", name),
		APIURL:  fmt.Sprintf("https://%s-forge.example.com/api/v1", name),
		Organizations: []string{
			fmt.Sprintf("%s-org", name),
		},
		Auth: &AuthConfig{
			Type:  AuthTypeToken,
			Token: fmt.Sprintf("forgejo-token-%s-%d", name, f.counter),
		},
		Webhook: &WebhookConfig{
			Secret: fmt.Sprintf("%s-forgejo-webhook-%d", name, f.counter),
			Path:   fmt.Sprintf("/webhooks/forgejo/%s", name),
			Events: []string{"push", "repository"},
		},
	}
}

// CreateForgeWithAutoDiscover creates a forge configuration with auto-discovery enabled
func (f *TestForgeConfigFactory) CreateForgeWithAutoDiscover(forgeType ForgeType, name string) *ForgeConfig {
	var forge *ForgeConfig

	switch forgeType {
	case ForgeGitHub:
		forge = f.CreateGitHubForge(name)
	case ForgeGitLab:
		forge = f.CreateGitLabForge(name)
	case ForgeForgejo:
		forge = f.CreateForgejoForge(name)
	default:
		forge = f.CreateGitHubForge(name) // Default to GitHub
	}

	// Clear organizations/groups and enable auto-discovery
	forge.Organizations = nil
	forge.Groups = nil
	forge.AutoDiscover = true

	return forge
}

// CreateForgeWithOptions creates a forge configuration with custom options
func (f *TestForgeConfigFactory) CreateForgeWithOptions(forgeType ForgeType, name string, options map[string]any) *ForgeConfig {
	var forge *ForgeConfig

	switch forgeType {
	case ForgeGitHub:
		forge = f.CreateGitHubForge(name)
	case ForgeGitLab:
		forge = f.CreateGitLabForge(name)
	case ForgeForgejo:
		forge = f.CreateForgejoForge(name)
	default:
		forge = f.CreateGitHubForge(name)
	}

	forge.Options = options

	// Handle auto_discover option
	if autoDiscover, ok := options["auto_discover"]; ok {
		if enabled, isBool := autoDiscover.(bool); isBool && enabled {
			forge.Organizations = nil
			forge.Groups = nil
		}
	}

	return forge
}

// CreateConfigWithForges creates a complete Config with realistic forge configurations
func (f *TestForgeConfigFactory) CreateConfigWithForges(forges []*ForgeConfig) *Config {
	return &Config{
		Version: "2.0",
		Hugo:    HugoConfig{Title: "TestForge Documentation", Theme: string(ThemeHextra)},
		Output:  OutputConfig{Directory: "./test-output", Clean: true},
		Build: BuildConfig{
			CloneConcurrency:  2,
			MaxRetries:        3,
			RetryBackoff:      RetryBackoffLinear,
			RetryInitialDelay: "1s",
			RetryMaxDelay:     "10s",
			CloneStrategy:     CloneStrategyFresh,
		},
		Forges: forges,
		Monitoring: &MonitoringConfig{
			Metrics: MonitoringMetrics{
				Enabled: true,
				Path:    "/metrics",
			},
		},
	}
}

// CreateMultiPlatformConfig creates a configuration with multiple realistic forge platforms
func (f *TestForgeConfigFactory) CreateMultiPlatformConfig(baseName string) *Config {
	forges := []*ForgeConfig{
		f.CreateGitHubForge(baseName),
		f.CreateGitLabForge(baseName),
		f.CreateForgejoForge(baseName),
	}

	return f.CreateConfigWithForges(forges)
}

// CreateFailureScenarioForges creates forge configurations for testing failure scenarios
func (f *TestForgeConfigFactory) CreateFailureScenarioForges(baseName string) []*ForgeConfig {
	return []*ForgeConfig{
		// Valid forge
		f.CreateGitHubForge(baseName + "-valid"),

		// Forge with invalid auth
		{
			Name:          baseName + "-invalid-auth",
			Type:          ForgeGitHub,
			BaseURL:       "https://github.com",
			APIURL:        "https://api.github.com",
			Organizations: []string{baseName + "-org"},
			Auth: &AuthConfig{
				Type:  AuthTypeToken,
				Token: "", // Invalid empty token
			},
		},

		// Forge with invalid webhook config
		{
			Name:    baseName + "-invalid-webhook",
			Type:    ForgeGitLab,
			BaseURL: "https://gitlab.com",
			APIURL:  "https://gitlab.com/api/v4",
			Groups:  []string{baseName + "-group"},
			Auth: &AuthConfig{
				Type:  AuthTypeToken,
				Token: "valid-token",
			},
			Webhook: &WebhookConfig{
				Secret: "",         // Invalid empty secret
				Path:   "",         // Invalid empty path
				Events: []string{}, // Invalid empty events
			},
		},
	}
}
