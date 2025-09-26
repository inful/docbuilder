package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// V2Config represents the v2 configuration format for daemon mode
type V2Config struct {
	Version    string            `yaml:"version"`
	Daemon     *DaemonConfig     `yaml:"daemon,omitempty"`
	Forges     []*ForgeConfig    `yaml:"forges"`
	Filtering  *FilteringConfig  `yaml:"filtering,omitempty"`
	Versioning *VersioningConfig `yaml:"versioning,omitempty"`
	Hugo       HugoConfig        `yaml:"hugo"`
	Monitoring *MonitoringConfig `yaml:"monitoring,omitempty"`
	Output     OutputConfig      `yaml:"output"`
}

// ForgeConfig represents configuration for a specific forge instance
type ForgeConfig struct {
	Name          string                 `yaml:"name"`          // Friendly name for this forge
	Type          string                 `yaml:"type"`          // Type of forge (github, gitlab, forgejo)
	APIURL        string                 `yaml:"api_url"`       // API base URL
	BaseURL       string                 `yaml:"base_url"`      // Web base URL (for edit links)
	Organizations []string               `yaml:"organizations"` // Organizations to scan (GitHub)
	Groups        []string               `yaml:"groups"`        // Groups to scan (GitLab/Forgejo)
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
	Strategy           string   `yaml:"strategy"`              // "branches_and_tags", "branches_only", "tags_only"
	DefaultBranchOnly  bool     `yaml:"default_branch_only"`   // Only build default branch
	BranchPatterns     []string `yaml:"branch_patterns"`       // Branch patterns to include
	TagPatterns        []string `yaml:"tag_patterns"`          // Tag patterns to include
	MaxVersionsPerRepo int      `yaml:"max_versions_per_repo"` // Maximum versions to keep per repo
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
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

// LoadV2 loads v2 configuration from the specified file
func LoadV2(configPath string) (*V2Config, error) {
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

	var config V2Config
	if err := yaml.Unmarshal([]byte(expandedData), &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal v2 config: %w", err)
	}

	// Validate version
	if config.Version != "2.0" {
		return nil, fmt.Errorf("unsupported configuration version: %s (expected 2.0)", config.Version)
	}

	// Apply defaults
	if err := applyV2Defaults(&config); err != nil {
		return nil, fmt.Errorf("failed to apply defaults: %w", err)
	}

	// Validate configuration
	if err := validateV2Config(&config); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return &config, nil
}

// applyV2Defaults applies default values to v2 configuration
func applyV2Defaults(config *V2Config) error {
	// Hugo defaults
	if config.Hugo.Title == "" {
		config.Hugo.Title = "Documentation Portal"
	}
	if config.Hugo.Theme == "" {
		config.Hugo.Theme = "hextra"
	}

	// Output defaults
	if config.Output.Directory == "" {
		config.Output.Directory = "./site"
	}
	config.Output.Clean = true // Always clean in daemon mode

	// Daemon defaults
	if config.Daemon != nil {
		if config.Daemon.HTTP.DocsPort == 0 {
			config.Daemon.HTTP.DocsPort = 8080
		}
		if config.Daemon.HTTP.WebhookPort == 0 {
			config.Daemon.HTTP.WebhookPort = 8081
		}
		if config.Daemon.HTTP.AdminPort == 0 {
			config.Daemon.HTTP.AdminPort = 8082
		}
		if config.Daemon.Sync.Schedule == "" {
			config.Daemon.Sync.Schedule = "0 */4 * * *" // Every 4 hours
		}
		if config.Daemon.Sync.ConcurrentBuilds == 0 {
			config.Daemon.Sync.ConcurrentBuilds = 3
		}
		if config.Daemon.Sync.QueueSize == 0 {
			config.Daemon.Sync.QueueSize = 100
		}
		if config.Daemon.Storage.StateFile == "" {
			config.Daemon.Storage.StateFile = "./docbuilder-state.json"
		}
		if config.Daemon.Storage.RepoCacheDir == "" {
			config.Daemon.Storage.RepoCacheDir = "./repositories"
		}
		if config.Daemon.Storage.OutputDir == "" {
			config.Daemon.Storage.OutputDir = config.Output.Directory
		}
	}

	// Filtering defaults: only set RequiredPaths if filtering is nil OR RequiredPaths is nil (not explicitly empty)
	if config.Filtering == nil {
		config.Filtering = &FilteringConfig{}
	}
	// Distinguish between nil slice and explicitly empty slice: if user wrote required_paths: [] we keep it empty
	if config.Filtering.RequiredPaths == nil {
		config.Filtering.RequiredPaths = []string{"docs"}
	}
	if len(config.Filtering.IgnoreFiles) == 0 {
		config.Filtering.IgnoreFiles = []string{".docignore"}
	}

	// Versioning defaults
	if config.Versioning == nil {
		config.Versioning = &VersioningConfig{}
	}
	if config.Versioning.Strategy == "" {
		config.Versioning.Strategy = "branches_and_tags"
	}
	if config.Versioning.MaxVersionsPerRepo == 0 {
		config.Versioning.MaxVersionsPerRepo = 10
	}

	// Monitoring defaults
	if config.Monitoring == nil {
		config.Monitoring = &MonitoringConfig{}
	}
	if config.Monitoring.Metrics.Path == "" {
		config.Monitoring.Metrics.Path = "/metrics"
	}
	if config.Monitoring.Health.Path == "" {
		config.Monitoring.Health.Path = "/health"
	}
	if config.Monitoring.Logging.Level == "" {
		config.Monitoring.Logging.Level = "info"
	}
	if config.Monitoring.Logging.Format == "" {
		config.Monitoring.Logging.Format = "json"
	}

	return nil
}

// validateV2Config validates the v2 configuration
func validateV2Config(config *V2Config) error {
	// Validate forges
	if len(config.Forges) == 0 {
		return fmt.Errorf("at least one forge must be configured")
	}

	forgeNames := make(map[string]bool)
	for _, forge := range config.Forges {
		if forge.Name == "" {
			return fmt.Errorf("forge name cannot be empty")
		}
		if forgeNames[forge.Name] {
			return fmt.Errorf("duplicate forge name: %s", forge.Name)
		}
		forgeNames[forge.Name] = true

		// Validate forge type
		switch forge.Type {
		case "github", "gitlab", "forgejo":
			// Valid types
		default:
			return fmt.Errorf("unsupported forge type: %s", forge.Type)
		}

		// Validate authentication
		if forge.Auth == nil {
			return fmt.Errorf("forge %s must have authentication configured", forge.Name)
		}

		// Organizations/Groups optional: if both empty we enter auto-discovery mode (all accessible orgs/groups)
		// Previously this was a hard validation error. Allowing it improves usability for first-time setup.
		// We retain explicit lists when provided to limit scope.
		// (No action needed here; discovery layer checks emptiness and enumerates.)
	}

	// Validate versioning strategy
	if config.Versioning != nil {
		validStrategies := map[string]bool{
			"branches_and_tags": true,
			"branches_only":     true,
			"tags_only":         true,
		}
		if !validStrategies[config.Versioning.Strategy] {
			return fmt.Errorf("invalid versioning strategy: %s", config.Versioning.Strategy)
		}
	}

	return nil
}

// InitV2 creates a new v2 configuration file with example content
func InitV2(configPath string, force bool) error {
	if _, err := os.Stat(configPath); err == nil && !force {
		return fmt.Errorf("configuration file already exists: %s (use --force to overwrite)", configPath)
	}

	exampleConfig := V2Config{
		Version: "2.0",
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
				Type:          "github",
				APIURL:        "https://api.github.com",
				BaseURL:       "https://github.com",
				Organizations: []string{"your-org"},
				Auth: &AuthConfig{
					Type:  "token",
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
			Strategy:           "branches_and_tags",
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

// IsV2Config checks if a configuration file is v2 format by reading the version field
func IsV2Config(configPath string) (bool, error) {
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
