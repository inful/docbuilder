package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

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

	// Apply defaults
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
	// Build defaults
	if config.Build.CloneConcurrency <= 0 {
		config.Build.CloneConcurrency = 4
	}
	// ShallowDepth: leave as-is (0 meaning disabled). Negative coerced to 0.
	if config.Build.ShallowDepth < 0 {
		config.Build.ShallowDepth = 0
	}
	// Clone strategy default: fresh (explicit destructive clone) unless user supplied a valid strategy.
	if config.Build.CloneStrategy == "" {
		config.Build.CloneStrategy = CloneStrategyFresh
	} else {
		cs := NormalizeCloneStrategy(string(config.Build.CloneStrategy))
		if cs != "" {
			config.Build.CloneStrategy = cs
		} else {
			config.Build.CloneStrategy = CloneStrategyFresh // fallback
		}
	}
	if config.Build.MaxRetries < 0 {
		config.Build.MaxRetries = 0
	}
	if config.Build.MaxRetries == 0 { // default 2 retries (3 total attempts) unless explicitly set >0
		config.Build.MaxRetries = 2
	}
	if config.Build.RetryBackoff == "" {
		config.Build.RetryBackoff = RetryBackoffLinear
	} else {
		// normalize any user-provided raw string (in case future loaders bypass yaml tag typing)
		config.Build.RetryBackoff = NormalizeRetryBackoff(string(config.Build.RetryBackoff))
		if config.Build.RetryBackoff == "" { // fallback to default if unknown
			config.Build.RetryBackoff = RetryBackoffLinear
		}
	}
	if config.Build.RetryInitialDelay == "" {
		config.Build.RetryInitialDelay = "1s"
	}
	if config.Build.RetryMaxDelay == "" {
		config.Build.RetryMaxDelay = "30s"
	}
	// Note: Build.WorkspaceDir default derived later in builder (depends on output.directory)

	// Hugo defaults
	if config.Hugo.Title == "" {
		config.Hugo.Title = "Documentation Portal"
	}
	if config.Hugo.Theme == "" {
		config.Hugo.Theme = string(ThemeHextra)
	}

	// Output defaults
	if config.Output.Directory == "" {
		config.Output.Directory = "./site"
	}
	config.Output.Clean = true // Always clean in daemon mode

	// Warn if user configured workspace_dir equal to output directory (can cause clone artifacts in final site)
	if config.Build.WorkspaceDir != "" {
		wd := filepath.Clean(config.Build.WorkspaceDir)
		od := filepath.Clean(config.Output.Directory)
		if wd == od {
			fmt.Fprintf(os.Stderr, "Warning: build.workspace_dir (%s) matches output.directory (%s); this may mix git working trees with generated site artifacts. Consider using a separate directory.\n", wd, od)
		}
	}

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
		// Only apply default if user omitted the field entirely.
		config.Versioning.Strategy = StrategyBranchesAndTags
	} else {
		orig := config.Versioning.Strategy
		norm := NormalizeVersioningStrategy(string(config.Versioning.Strategy))
		if norm != "" {
			config.Versioning.Strategy = norm
		} else {
			// Preserve original invalid value so validateConfig can raise an error.
			config.Versioning.Strategy = VersioningStrategy(orig)
		}
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
		config.Monitoring.Logging.Level = LogLevelInfo
	} else {
		lvl := NormalizeLogLevel(string(config.Monitoring.Logging.Level))
		if lvl != "" {
			config.Monitoring.Logging.Level = lvl
		}
	}
	if config.Monitoring.Logging.Format == "" {
		config.Monitoring.Logging.Format = LogFormatJSON
	} else {
		fmtVal := NormalizeLogFormat(string(config.Monitoring.Logging.Format))
		if fmtVal != "" {
			config.Monitoring.Logging.Format = fmtVal
		}
	}

	// Explicit repository defaults (paths/branch) mirroring legacy behavior
	for i := range config.Repositories {
		if len(config.Repositories[i].Paths) == 0 {
			config.Repositories[i].Paths = []string{"docs"}
		}
		if config.Repositories[i].Branch == "" {
			config.Repositories[i].Branch = "main"
		}
	}

	return nil
}

// validateConfig validates the configuration
func validateConfig(config *Config) error {
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

		// Normalize & validate forge type
		if forge.Type == "" { // empty is invalid
			return fmt.Errorf("unsupported forge type: %s", forge.Type)
		}
		norm := NormalizeForgeType(string(forge.Type))
		if norm == "" {
			return fmt.Errorf("unsupported forge type: %s", forge.Type)
		}
		forge.Type = norm

		// Validate authentication
		if forge.Auth == nil {
			return fmt.Errorf("forge %s must have authentication configured", forge.Name)
		}
		if forge.Auth != nil {
			switch forge.Auth.Type {
			case AuthTypeToken, AuthTypeSSH, AuthTypeBasic, AuthTypeNone, "":
				// ok; semantic checks done by individual clients
			default:
				return fmt.Errorf("forge %s: unsupported auth type: %s", forge.Name, forge.Auth.Type)
			}
			// Minimal semantic validation now (clients perform stricter checks when constructing)
			// Token presence is validated lazily by forge clients / git operations to permit env placeholders.
		}

		// Require at least one organization or group to be specified. This keeps discovery bounded
		// Validate explicit repository auth blocks (if provided)
		for _, repo := range config.Repositories {
			if repo.Auth != nil {
				switch repo.Auth.Type {
				case AuthTypeToken, AuthTypeSSH, AuthTypeBasic, AuthTypeNone, "":
					// valid
				default:
					return fmt.Errorf("repository %s: unsupported auth type: %s", repo.Name, repo.Auth.Type)
				}
				// Token emptiness allowed (environment may supply later)
				if repo.Auth.Type == AuthTypeBasic && (repo.Auth.Username == "" || repo.Auth.Password == "") {
					return fmt.Errorf("repository %s: basic auth requires username and password", repo.Name)
				}
			}
		}
		// and matches test expectations for explicit configuration (auto-discovery can be added
		// later behind a dedicated flag to avoid surprising large scans).
		if len(forge.Organizations) == 0 && len(forge.Groups) == 0 {
			return fmt.Errorf("forge %s must have at least one organization or group configured", forge.Name)
		}
	}

	// Validate versioning strategy
	if config.Versioning != nil {
		if config.Versioning.Strategy != StrategyBranchesAndTags && config.Versioning.Strategy != StrategyBranchesOnly && config.Versioning.Strategy != StrategyTagsOnly {
			return fmt.Errorf("invalid versioning strategy: %s", config.Versioning.Strategy)
		}
	}

	// Validate retry configuration
	switch config.Build.RetryBackoff {
	case RetryBackoffFixed, RetryBackoffLinear, RetryBackoffExponential:
	default:
		return fmt.Errorf("invalid retry_backoff: %s (allowed: fixed|linear|exponential)", config.Build.RetryBackoff)
	}
	// Validate clone strategy
	switch config.Build.CloneStrategy {
	case CloneStrategyFresh, CloneStrategyUpdate, CloneStrategyAuto:
	default:
		return fmt.Errorf("invalid clone_strategy: %s (allowed: fresh|update|auto)", config.Build.CloneStrategy)
	}
	if _, err := time.ParseDuration(config.Build.RetryInitialDelay); err != nil {
		return fmt.Errorf("invalid retry_initial_delay: %s: %w", config.Build.RetryInitialDelay, err)
	}
	if _, err := time.ParseDuration(config.Build.RetryMaxDelay); err != nil {
		return fmt.Errorf("invalid retry_max_delay: %s: %w", config.Build.RetryMaxDelay, err)
	}
	initDur, _ := time.ParseDuration(config.Build.RetryInitialDelay)
	maxDur, _ := time.ParseDuration(config.Build.RetryMaxDelay)
	if maxDur < initDur {
		return fmt.Errorf("retry_max_delay (%s) must be >= retry_initial_delay (%s)", config.Build.RetryMaxDelay, config.Build.RetryInitialDelay)
	}
	if config.Build.MaxRetries < 0 {
		return fmt.Errorf("max_retries cannot be negative: %d", config.Build.MaxRetries)
	}

	return nil
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

// ------------------------------------------------------------
// Backwards compatibility (temporary) – deprecated aliases.
// ------------------------------------------------------------

// V2Config is deprecated; use Config.
// Deprecated: use Config.
type V2Config = Config

// LoadV2 is deprecated; use Load.
// Deprecated: use Load.
func LoadV2(path string) (*Config, error) { return Load(path) }

// InitV2 is deprecated; use Init.
// Deprecated: use Init.
func InitV2(path string, force bool) error { return Init(path, force) }

// IsV2Config is deprecated; use IsConfigVersion.
// Deprecated: use IsConfigVersion.
func IsV2Config(path string) (bool, error) { return IsConfigVersion(path) }
