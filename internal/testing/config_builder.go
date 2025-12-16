package testing

import (
	"os"
	"path/filepath"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"gopkg.in/yaml.v3"
)

// ConfigBuilder provides a fluent interface for creating test configurations
type ConfigBuilder struct {
	config *config.Config
	t      *testing.T
}

// NewConfigBuilder creates a new configuration builder for tests
func NewConfigBuilder(t *testing.T) *ConfigBuilder {
	return &ConfigBuilder{
		config: &config.Config{
			Version: "2.0",
			Hugo: config.HugoConfig{
				Title: "Test Documentation",
			},
			Output: config.OutputConfig{
				Directory: filepath.Join(os.TempDir(), "docbuilder-test-output"),
			},
			Build: config.BuildConfig{
				CloneConcurrency: 2,
				RenderMode:       config.RenderModeAuto,
				NamespaceForges:  config.NamespacingAuto,
				CloneStrategy:    config.CloneStrategyFresh,
			},
		},
		t: t,
	}
}

// WithTitle sets the Hugo title
func (cb *ConfigBuilder) WithTitle(title string) *ConfigBuilder {
	cb.config.Hugo.Title = title
	return cb
}

// WithTheme sets the Hugo theme
func (cb *ConfigBuilder) WithTheme(theme string) *ConfigBuilder {
	return cb
}

// WithOutputDir sets the output directory
func (cb *ConfigBuilder) WithOutputDir(dir string) *ConfigBuilder {
	cb.config.Output.Directory = dir
	return cb
}

// WithGitHubForge adds a GitHub forge configuration
func (cb *ConfigBuilder) WithGitHubForge(name, token string, organizations ...string) *ConfigBuilder {
	forge := &config.ForgeConfig{
		Name:          name,
		Type:          config.ForgeGitHub,
		APIURL:        "https://api.github.com",
		BaseURL:       "https://github.com",
		Organizations: organizations,
		Auth: &config.AuthConfig{
			Type:  config.AuthTypeToken,
			Token: token,
		},
	}
	cb.config.Forges = append(cb.config.Forges, forge)
	return cb
}

// WithGitLabForge adds a GitLab forge configuration
func (cb *ConfigBuilder) WithGitLabForge(name, token string, groups ...string) *ConfigBuilder {
	forge := &config.ForgeConfig{
		Name:    name,
		Type:    config.ForgeGitLab,
		APIURL:  "https://gitlab.com/api/v4",
		BaseURL: "https://gitlab.com",
		Groups:  groups,
		Auth: &config.AuthConfig{
			Type:  config.AuthTypeToken,
			Token: token,
		},
	}
	cb.config.Forges = append(cb.config.Forges, forge)
	return cb
}

// WithForgejoForge adds a Forgejo forge configuration
func (cb *ConfigBuilder) WithForgejoForge(name, baseURL, token string, organizations ...string) *ConfigBuilder {
	forge := &config.ForgeConfig{
		Name:          name,
		Type:          config.ForgeForgejo,
		APIURL:        baseURL + "/api/v1",
		BaseURL:       baseURL,
		Organizations: organizations,
		Auth: &config.AuthConfig{
			Type:  config.AuthTypeToken,
			Token: token,
		},
	}
	cb.config.Forges = append(cb.config.Forges, forge)
	return cb
}

// WithAutoDiscovery enables auto-discovery for the last added forge
func (cb *ConfigBuilder) WithAutoDiscovery() *ConfigBuilder {
	if len(cb.config.Forges) > 0 {
		cb.config.Forges[len(cb.config.Forges)-1].AutoDiscover = true
	}
	return cb
}

// WithRepository adds an explicit repository configuration
func (cb *ConfigBuilder) WithRepository(name, url, branch string) *ConfigBuilder {
	repo := config.Repository{
		Name:   name,
		URL:    url,
		Branch: branch,
	}
	cb.config.Repositories = append(cb.config.Repositories, repo)
	return cb
}

// WithDaemon adds daemon configuration
func (cb *ConfigBuilder) WithDaemon(docsPort, webhookPort, adminPort int) *ConfigBuilder {
	cb.config.Daemon = &config.DaemonConfig{
		HTTP: config.HTTPConfig{
			DocsPort:    docsPort,
			WebhookPort: webhookPort,
			AdminPort:   adminPort,
		},
		Sync: config.SyncConfig{
			Schedule:         "0 */6 * * *", // Every 6 hours
			ConcurrentBuilds: 2,
			QueueSize:        10,
		},
	}
	return cb
}

// WithMonitoring adds monitoring configuration
func (cb *ConfigBuilder) WithMonitoring(metricsEnabled bool) *ConfigBuilder {
	cb.config.Monitoring = &config.MonitoringConfig{
		Metrics: config.MonitoringMetrics{
			Enabled: metricsEnabled,
			Path:    "/metrics",
		},
		Health: config.MonitoringHealth{
			Path: "/health",
		},
		Logging: config.MonitoringLogging{
			Level:  "info",
			Format: "text",
		},
	}
	return cb
}

// WithVersioning adds versioning configuration
func (cb *ConfigBuilder) WithVersioning(strategy config.VersioningStrategy) *ConfigBuilder {
	cb.config.Versioning = &config.VersioningConfig{
		Strategy: strategy,
	}
	return cb
}

// Build returns the built configuration
func (cb *ConfigBuilder) Build() *config.Config {
	return cb.config
}

// BuildAndSave builds the configuration and saves it to a file
func (cb *ConfigBuilder) BuildAndSave(filePath string) *config.Config {
	// Create YAML content
	data, err := yaml.Marshal(cb.config)
	if err != nil {
		cb.t.Fatalf("Failed to marshal config: %v", err)
	}

	// Write to file with tighter permissions
	if err := os.WriteFile(filePath, data, 0o600); err != nil {
		cb.t.Fatalf("Failed to save config to %s: %v", filePath, err)
	}
	return cb.config
}

// ConfigFactory provides common configuration patterns for tests
type ConfigFactory struct {
	t *testing.T
}

// NewConfigFactory creates a new configuration factory
func NewConfigFactory(t *testing.T) *ConfigFactory {
	return &ConfigFactory{t: t}
}

// MinimalConfig creates a minimal valid configuration
func (cf *ConfigFactory) MinimalConfig() *config.Config {
	return NewConfigBuilder(cf.t).
		WithGitHubForge("test-github", "test-token", "test-org").
		Build()
}

// MultiForgeConfig creates a configuration with multiple forges
func (cf *ConfigFactory) MultiForgeConfig() *config.Config {
	return NewConfigBuilder(cf.t).
		WithGitHubForge("github", "github-token", "github-org").
		WithGitLabForge("gitlab", "gitlab-token", "gitlab-group").
		WithForgejoForge("forgejo", "https://codeberg.org", "forgejo-token", "forgejo-org").
		Build()
}

// DaemonConfig creates a configuration suitable for daemon mode
func (cf *ConfigFactory) DaemonConfig() *config.Config {
	return NewConfigBuilder(cf.t).
		WithGitHubForge("github", "github-token", "github-org").
		WithDaemon(8080, 8081, 8082).
		WithMonitoring(true).
		WithVersioning(config.StrategyBranchesAndTags).
		Build()
}

// AutoDiscoveryConfig creates a configuration with auto-discovery enabled
func (cf *ConfigFactory) AutoDiscoveryConfig() *config.Config {
	return NewConfigBuilder(cf.t).
		WithGitHubForge("github", "github-token").
		WithAutoDiscovery().
		Build()
}

// ValidationTestConfig creates a configuration for validation testing
func (cf *ConfigFactory) ValidationTestConfig(invalidField string) *config.Config {
	builder := NewConfigBuilder(cf.t)

	switch invalidField {
	case "empty_forges":
		// Return config with no forges (should fail validation)
		return builder.Build()
	case "invalid_auth_type":
		config := builder.WithGitHubForge("test", "token", "org").Build()
		config.Forges[0].Auth.Type = "invalid"
		return config
	case "negative_ports":
		return builder.
			WithGitHubForge("test", "token", "org").
			WithDaemon(-1, -1, -1).
			Build()
	default:
		cf.t.Fatalf("Unknown invalid field: %s", invalidField)
		return nil
	}
}
