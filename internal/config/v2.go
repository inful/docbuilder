package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config represents the unified (v2) configuration format for daemon and direct modes.
type Config struct {
	Version    string            `yaml:"version"`
	Daemon     *DaemonConfig     `yaml:"daemon,omitempty"`
	Build      BuildConfig       `yaml:"build,omitempty"`
	Forges     []*ForgeConfig    `yaml:"forges"`
	Filtering  *FilteringConfig  `yaml:"filtering,omitempty"`
	Versioning *VersioningConfig `yaml:"versioning,omitempty"`
	Hugo       HugoConfig        `yaml:"hugo"`
	Monitoring *MonitoringConfig `yaml:"monitoring,omitempty"`
	Output     OutputConfig      `yaml:"output"`
	// Optional explicit repository list (direct mode) – replaces legacy v1 top‑level repositories.
	// When present, these are used directly for build/discover operations. If empty and forges are
	// configured, auto‑discovery can populate repositories dynamically.
	Repositories []Repository `yaml:"repositories,omitempty"`
}

// ForgeConfig represents configuration for a specific forge instance
type ForgeConfig struct {
	Name          string                 `yaml:"name"`          // Friendly name for this forge
	Type          ForgeType              `yaml:"type"`          // Typed forge kind
	APIURL        string                 `yaml:"api_url"`       // API base URL
	BaseURL       string                 `yaml:"base_url"`      // Web base URL (for edit links)
	Organizations []string               `yaml:"organizations"` // Organizations to scan (GitHub)
	Groups        []string               `yaml:"groups"`        // Groups to scan (GitLab/Forgejo)
	AutoDiscover  bool                   `yaml:"auto_discover"` // Enable full auto-discovery when no org/group listed
	Auth          *AuthConfig            `yaml:"auth"`          // Authentication config
	Webhook       *WebhookConfig         `yaml:"webhook"`       // Webhook configuration
	Options       map[string]interface{} `yaml:"options"`       // Forge-specific options
}

// WebhookConfig represents webhook configuration for a forge
type WebhookConfig struct {
	Secret       string   `yaml:"secret"`        // Webhook secret for validation
	Path         string   `yaml:"path"`          // Webhook endpoint path
	Events       []string `yaml:"events"`        // Events to listen for
	RegisterAuto bool     `yaml:"register_auto"` // Auto-register webhooks
}

// DaemonConfig represents daemon-specific configuration
type DaemonConfig struct {
	HTTP    HTTPConfig    `yaml:"http"`
	Sync    SyncConfig    `yaml:"sync"`
	Storage StorageConfig `yaml:"storage"`
}

// HTTPConfig represents HTTP server configuration
type HTTPConfig struct {
	DocsPort    int `yaml:"docs_port"`    // Documentation serving port
	WebhookPort int `yaml:"webhook_port"` // Webhook reception port
	AdminPort   int `yaml:"admin_port"`   // Admin/status endpoints port
}

// SyncConfig represents synchronization configuration
type SyncConfig struct {
	Schedule         string `yaml:"schedule"`          // Cron expression for discovery
	ConcurrentBuilds int    `yaml:"concurrent_builds"` // Max parallel repository builds
	QueueSize        int    `yaml:"queue_size"`        // Max queued build requests
}

// StorageConfig represents storage configuration
type StorageConfig struct {
	StateFile    string `yaml:"state_file"`     // Path to state file
	RepoCacheDir string `yaml:"repo_cache_dir"` // Directory for cached repositories
	OutputDir    string `yaml:"output_dir"`     // Output directory for generated site
}

// FilteringConfig represents repository filtering configuration
type FilteringConfig struct {
	RequiredPaths   []string `yaml:"required_paths"`   // Paths that must exist (e.g., "docs")
	IgnoreFiles     []string `yaml:"ignore_files"`     // Files that exclude repo (e.g., ".docignore")
	IncludePatterns []string `yaml:"include_patterns"` // Repository name patterns to include
	ExcludePatterns []string `yaml:"exclude_patterns"` // Repository name patterns to exclude
}

// VersioningConfig represents multi-version documentation configuration
type VersioningConfig struct {
	Strategy           VersioningStrategy `yaml:"strategy"`              // typed: branches_and_tags|branches_only|tags_only
	DefaultBranchOnly  bool               `yaml:"default_branch_only"`   // Only build default branch
	BranchPatterns     []string           `yaml:"branch_patterns"`       // Branch patterns to include
	TagPatterns        []string           `yaml:"tag_patterns"`          // Tag patterns to include
	MaxVersionsPerRepo int                `yaml:"max_versions_per_repo"` // Maximum versions to keep per repo
}

// MonitoringConfig represents monitoring and observability configuration
type MonitoringConfig struct {
	Metrics MonitoringMetrics `yaml:"metrics"`
	Health  MonitoringHealth  `yaml:"health"`
	Logging MonitoringLogging `yaml:"logging"`
}

// MonitoringMetrics represents metrics configuration
type MonitoringMetrics struct {
	Enabled bool   `yaml:"enabled"`
	Path    string `yaml:"path"`
}

// MonitoringHealth represents health check configuration
type MonitoringHealth struct {
	Path string `yaml:"path"`
}

// MonitoringLogging represents logging configuration
type MonitoringLogging struct {
	Level  LogLevel  `yaml:"level"`
	Format LogFormat `yaml:"format"`
}

// (Deprecated comment retained for context) Previous: LoadV2 loads v2 configuration from the specified file.
// Load loads a configuration file (version 2.x).
func Load(configPath string) (*Config, error) {
	// Load .env file if it exists
	if err := loadEnvFile(); err != nil {
		// Don't fail if .env doesn't exist, just log it
		fmt.Fprintf(os.Stderr, "Note: .env file not found or couldn't be loaded: %v\n", err)
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("configuration file not found: %s", configPath)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Expand environment variables in the YAML content
	expandedData := os.ExpandEnv(string(data))

	var config Config
	if err := yaml.Unmarshal([]byte(expandedData), &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal v2 config: %w", err)
	}

	// Validate version
	if config.Version != "2.0" {
		return nil, fmt.Errorf("unsupported configuration version: %s (expected 2.0)", config.Version)
	}

	// Normalization pass (case-fold enumerations, bounds, early coercions)
	if nres, nerr := NormalizeConfig(&config); nerr != nil {
		return nil, fmt.Errorf("normalize: %w", nerr)
	} else if nres != nil && len(nres.Warnings) > 0 {
		for _, w := range nres.Warnings {
			fmt.Fprintf(os.Stderr, "config normalization: %s\n", w)
		}
	}
	// Apply defaults (after normalization so canonical values drive defaults)
	if err := applyDefaults(&config); err != nil {
		return nil, fmt.Errorf("failed to apply defaults: %w", err)
	}

	// Validate configuration
	if err := validateConfig(&config); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return &config, nil
}

// applyDefaults applies default values to configuration
func applyDefaults(config *Config) error {
	applier := NewDefaultApplier()
	return applier.ApplyDefaults(config)
}

// validateConfig validates the configuration
func validateConfig(config *Config) error {
	return ValidateConfig(config)
}

// (Deprecated comment retained) Previously: InitV2 creates a new v2 configuration file with example content.
// Init writes an example configuration file (version 2.0).
func Init(configPath string, force bool) error {
	if _, err := os.Stat(configPath); err == nil && !force {
		return fmt.Errorf("configuration file already exists: %s (use --force to overwrite)", configPath)
	}

	exampleConfig := Config{
		Version: "2.0",
		Build:   BuildConfig{CloneConcurrency: 4, MaxRetries: 2, RetryBackoff: RetryBackoffLinear, RetryInitialDelay: "1s", RetryMaxDelay: "30s"},
		Daemon: &DaemonConfig{
			HTTP: HTTPConfig{
				DocsPort:    8080,
				WebhookPort: 8081,
				AdminPort:   8082,
			},
			Sync: SyncConfig{
				Schedule:         "0 */4 * * *",
				ConcurrentBuilds: 3,
				QueueSize:        100,
			},
			Storage: StorageConfig{
				StateFile:    "./docbuilder-state.json",
				RepoCacheDir: "./repositories",
				OutputDir:    "./site",
			},
		},
		Forges: []*ForgeConfig{
			{
				Name:          "company-github",
				Type:          ForgeGitHub,
				APIURL:        "https://api.github.com",
				BaseURL:       "https://github.com",
				Organizations: []string{"your-org"},
				Auth: &AuthConfig{
					Type:  AuthTypeToken,
					Token: "${GITHUB_TOKEN}",
				},
				Webhook: &WebhookConfig{
					Secret: "${GITHUB_WEBHOOK_SECRET}",
					Path:   "/webhooks/github",
					Events: []string{"push", "repository"},
				},
			},
		},
		Filtering: &FilteringConfig{
			RequiredPaths: []string{"docs"},
			IgnoreFiles:   []string{".docignore"},
		},
		Versioning: &VersioningConfig{
			Strategy:           StrategyBranchesAndTags,
			DefaultBranchOnly:  false,
			BranchPatterns:     []string{"main", "master", "develop"},
			TagPatterns:        []string{"v*.*.*"},
			MaxVersionsPerRepo: 10,
		},
		Hugo: HugoConfig{
			Title:       "Company Documentation Portal",
			Description: "Aggregated documentation from all engineering projects",
			BaseURL:     "https://docs.company.com",
			Theme:       "hextra",
		},
		Monitoring: &MonitoringConfig{
			Metrics: MonitoringMetrics{
				Enabled: true,
				Path:    "/metrics",
			},
			Health: MonitoringHealth{
				Path: "/health",
			},
			Logging: MonitoringLogging{
				Level:  "info",
				Format: "json",
			},
		},
		Output: OutputConfig{
			Directory: "./site",
			Clean:     true,
		},
		// Example explicit repositories block (optional; forge discovery may also be used)
		Repositories: []Repository{
			{
				URL:    "https://github.com/example/repo1.git",
				Name:   "repo1",
				Branch: "main",
				Paths:  []string{"docs"},
			},
		},
	}

	data, err := yaml.Marshal(&exampleConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal v2 config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write v2 config file: %w", err)
	}

	return nil
}

// (Deprecated) Previously: IsV2Config checks if a configuration file is v2 format by reading the version field.
// IsConfigVersion returns true if the config file version field starts with 2.
func IsConfigVersion(configPath string) (bool, error) {
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return false, fmt.Errorf("configuration file not found: %s", configPath)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return false, fmt.Errorf("failed to read config file: %w", err)
	}

	// Expand environment variables
	expandedData := os.ExpandEnv(string(data))

	// Try to parse just the version field
	var versionCheck struct {
		Version string `yaml:"version"`
	}

	if err := yaml.Unmarshal([]byte(expandedData), &versionCheck); err != nil {
		// If it fails to parse, assume v1
		return false, nil
	}

	return strings.HasPrefix(versionCheck.Version, "2."), nil
}

// Deprecated aliases removed (V2Config, LoadV2, InitV2, IsV2Config) as part of legacy cleanup.
