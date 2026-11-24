// Package config provides configuration types and helpers for DocBuilder, including loading,
// validation, and normalization of the unified v2 configuration format.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"git.home.luguber.info/inful/docbuilder/internal/foundation/errors"
	"gopkg.in/yaml.v3"
)

const configVersion = "2.0"

// Config represents the unified (v2) configuration format for DocBuilder, supporting both daemon and direct modes.
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
	// Lint configures the documentation linter. Optional; if absent the
	// linter uses its built-in defaults (see internal/lint.DefaultExcludes).
	Lint *LintConfig `yaml:"lint,omitempty"`
	// Search configures the search of the site. Optional; if absent the
	// defaults apply (see SearchConfig).
	Search *SearchConfig `yaml:"search,omitempty"`
	// Optional explicit repository list (direct mode) – replaces legacy v1 top‑level repositories.
	// When present, these are used directly for build/discover operations. If empty and forges are
	// configured, auto‑discovery can populate repositories dynamically.
	Repositories []Repository `yaml:"repositories,omitempty"`
}

// The providers of the site search.
const (
	// SearchProviderServer runs the search on the server. The daemon
	// keeps the index of the rendered pages and answers the queries, so
	// the visitor downloads the results and nothing else.
	SearchProviderServer = "server"
	// SearchProviderLunr runs the search in the browser. The theme ships
	// the index and the visitor's browser searches it. This is what a
	// static site needs: it has no server to ask.
	SearchProviderLunr = "lunr"
)

// SearchConfig configures the search of the site.
//
// The daemon serves the site, so the search runs there: the index never
// reaches the visitor, and the results stay fast however large the site
// is. A site that is served as a static tree - exported and handed to a
// web server - needs the browser-side index instead. The provider
// follows who builds the site, so the two never disagree.
type SearchConfig struct {
	// Provider is where the search runs: "server" or "lunr". Empty means
	// the builder decides: the daemon chooses the server, a plain build
	// chooses the browser.
	Provider string `yaml:"provider,omitempty"`
	// MaxResults is how many results a search returns. Zero means the
	// default applies.
	MaxResults int `yaml:"max_results,omitempty"`
}

// LintConfig configures the documentation linter. The Excludes list is
// the cross-forge union of git-forge-conventional files that should be
// skipped from lint and schema enforcement.
type LintConfig struct {
	// Excludes is the list of glob patterns identifying files that should
	// be skipped from lint and schema enforcement. When nil, the linter's
	// built-in DefaultExcludes applies. An explicit empty slice disables
	// all excludes.
	Excludes []string `yaml:"excludes,omitempty"`
}

// IsDaemonPublicOnlyEnabled returns true if the daemon is configured and public-only mode is enabled.
func (c *Config) IsDaemonPublicOnlyEnabled() bool {
	return c != nil && c.Daemon != nil && c.Daemon.Content.PublicOnly
}

// ForgeConfig represents configuration for a specific forge instance (e.g., GitHub, GitLab, Forgejo).
type ForgeConfig struct {
	Name          string         `yaml:"name"`          // Friendly name for this forge
	Type          ForgeType      `yaml:"type"`          // Typed forge kind
	APIURL        string         `yaml:"api_url"`       // API base URL
	BaseURL       string         `yaml:"base_url"`      // Web base URL (for edit links)
	Organizations []string       `yaml:"organizations"` // Organizations to scan (GitHub)
	Groups        []string       `yaml:"groups"`        // Groups to scan (GitLab/Forgejo)
	AutoDiscover  bool           `yaml:"auto_discover"` // Enable full auto-discovery when no org/group listed
	Auth          *AuthConfig    `yaml:"auth"`          // Authentication config
	Webhook       *WebhookConfig `yaml:"webhook"`       // Webhook configuration
	Options       map[string]any `yaml:"options"`       // Forge-specific options

	// IgnoreFiles are the files that keep a repository out of the
	// site. The daemon fills them from filtering.ignore_files: the
	// files are a setting of the site, not of one forge.
	IgnoreFiles []string `yaml:"-"`
}

// WebhookConfig represents webhook configuration for a forge, including secret, path, and events.
type WebhookConfig struct {
	Secret       string   `yaml:"secret"`        // Webhook secret for validation
	Path         string   `yaml:"path"`          // Webhook endpoint path
	Events       []string `yaml:"events"`        // Events to listen for
	RegisterAuto bool     `yaml:"register_auto"` // Auto-register webhooks
}

// DaemonConfig represents daemon-specific configuration, including HTTP, sync, and storage settings.
type DaemonConfig struct {
	HTTP             HTTPConfig              `yaml:"http"`
	Sync             SyncConfig              `yaml:"sync"`
	Storage          StorageConfig           `yaml:"storage"`
	Content          DaemonContentConfig     `yaml:"content,omitempty"`
	BuildDebounce    *BuildDebounceConfig    `yaml:"build_debounce,omitempty"`
	LinkVerification *LinkVerificationConfig `yaml:"link_verification,omitempty"`
	Outbound         *OutboundConfig         `yaml:"outbound,omitempty"`
}

// OutboundConfig controls outbound integrations that the daemon pushes
// document data to after a successful build. All sub-configurations are
// opt-in: leaving the section out entirely disables every outbound hook.
type OutboundConfig struct {
	// Ragabast, when non-nil and Enabled, fires a per-document async ingest
	// POST to ragabast's /api/ingest/async endpoint after each non-generated
	// document is written to disk.
	Ragabast *RagabastConfig `yaml:"ragabast,omitempty"`
}

// RagabastConfig configures the outbound dispatcher that pushes each
// generated document to ragabast for async ingest. The section is
// completely absent in defaults — operators opt in by adding the block to
// their config and setting enabled: true.
type RagabastConfig struct {
	// Enabled toggles the dispatcher. Defaults to false; must be set
	// explicitly to true to construct the dispatcher.
	Enabled bool `yaml:"enabled"`
	// BaseURL is the preferred field: the ragabast host (scheme+host).
	// The dispatcher derives the upload path ({base}/api/ingest/file)
	// and the preflight path ({base}/api/documents/<uid>/fingerprint)
	// from this single value, so operators cannot accidentally POST
	// to "/" by forgetting a path component. Path components on this
	// field are stripped. Example: "https://ragabast.example.com".
	BaseURL string `yaml:"ragabast_base_url,omitempty"`
	// IngestURL is deprecated. When BaseURL is empty and IngestURL is
	// set, the dispatcher derives scheme+host from IngestURL (path is
	// discarded) and logs a deprecation warning at startup. Migrate by
	// replacing `ingest_url: https://ragabast.example.com/api/ingest/file`
	// with `ragabast_base_url: https://ragabast.example.com`.
	IngestURL string `yaml:"ingest_url"`
	// AuthTokenEnv is the name of the environment variable that holds the
	// bearer token sent in the Authorization header. The token is read from
	// os.Getenv at daemon startup — never written to disk or logs.
	// Empty token (when AuthTokenEnv is unset or the env var is empty)
	// means anonymous requests, which ragabast accepts when its own
	// auth_token config is empty.
	AuthTokenEnv string `yaml:"auth_token_env"`
	// Workers is the number of concurrent POST goroutines. Defaults to 4.
	Workers int `yaml:"workers,omitempty"`
	// QueueSize is the in-memory buffer capacity between document write
	// and HTTP POST. Defaults to 256. When the buffer is full, additional
	// documents are dropped (with a counter) rather than blocking the
	// pipeline — ragabast's uid+fingerprint dedup makes drops self-healing.
	QueueSize int `yaml:"queue_size,omitempty"`
	// Timeout is the per-POST HTTP timeout. Defaults to "10s".
	Timeout string `yaml:"timeout,omitempty"`
	// PreflightBaseURL is the ragabast base URL used to construct the
	// preflight request GET <base>/api/documents/<uid>/fingerprint. When
	// empty, the dispatcher derives the base from IngestURL's scheme+host.
	// Set to a literal "-" to disable preflight regardless of other settings.
	PreflightBaseURL string `yaml:"preflight_base_url,omitempty"`
	// PreflightEnabled toggles whether the dispatcher consults ragabast's
	// fingerprint endpoint before POSTing each document. When unset (nil),
	// preflight is treated as enabled whenever a base URL can be resolved
	// (explicit PreflightBaseURL or derivable from IngestURL). Set to
	// false explicitly to opt out.
	PreflightEnabled *bool `yaml:"preflight_enabled,omitempty"`
}

// BuildDebounceConfig controls debouncing/coalescing behavior for build requests.
//
// Durations must be valid Go duration strings (e.g. "10s", "1m").
type BuildDebounceConfig struct {
	QuietWindow string `yaml:"quiet_window,omitempty"`
	MaxDelay    string `yaml:"max_delay,omitempty"`

	// WebhookImmediate controls whether webhook-triggered build requests should bypass
	// the quiet window (but still respect "build running" coalescing).
	//
	// When unset, defaults to true to preserve prior behavior.
	WebhookImmediate *bool `yaml:"webhook_immediate,omitempty"`
}

// DaemonContentConfig represents daemon-specific content policies.
type DaemonContentConfig struct {
	// PublicOnly enables opt-in publishing based on per-page frontmatter `public: true`.
	PublicOnly bool `yaml:"public_only"`
}

// HTTPConfig represents HTTP server configuration for the daemon, including ports for docs, webhooks, and admin endpoints.
type HTTPConfig struct {
	DocsPort       int `yaml:"docs_port"`       // Documentation serving port
	WebhookPort    int `yaml:"webhook_port"`    // Webhook reception port
	AdminPort      int `yaml:"admin_port"`      // Admin/status endpoints port
	LiveReloadPort int `yaml:"livereload_port"` // LiveReload SSE endpoint port (separate to avoid HTTP/1.1 blocking)

	// AdminBind is the address the admin server listens on.
	//
	// The admin API can start builds and shows the forge layout, and it
	// has no password, so it is bound to the loopback interface by
	// default. Set it to 0.0.0.0 to listen on every interface - only do
	// that when something in front of the daemon (a reverse proxy, a
	// firewall) restricts who can reach the port.
	AdminBind string `yaml:"admin_bind"`
}

// SyncConfig represents synchronization configuration for repository discovery and build queueing.
type SyncConfig struct {
	Schedule         string `yaml:"schedule"`          // Cron expression for discovery
	ConcurrentBuilds int    `yaml:"concurrent_builds"` // Number of build queue workers. The daemon runs one build at a time (the build holds the staging directory), so the extra workers wait.
	QueueSize        int    `yaml:"queue_size"`        // Max queued build requests
	// BuildOnDiscovery controls whether a forge discovery run should enqueue a
	// build for discovered repositories. When unset, defaults to true.
	BuildOnDiscovery *bool `yaml:"build_on_discovery,omitempty"`
}

// StorageConfig represents storage configuration for state, repository cache, and output directories.
type StorageConfig struct {
	StateFile    string `yaml:"state_file"`     // Path to state file
	RepoCacheDir string `yaml:"repo_cache_dir"` // Directory for cached repositories
	OutputDir    string `yaml:"output_dir"`     // Output directory for generated site
	// StateDir is where the daemon keeps its state: the state file, the
	// event log and the remote-head cache. It is separate from the
	// repository cache so that a cache wipe does not take the state with
	// it, and so that walking the cache never walks the state.
	StateDir string `yaml:"state_dir,omitempty"`
	// EventRetention is how long build events are kept in the event store.
	// "0" keeps everything; otherwise the store prunes events older than
	// this duration, so the log cannot grow without bound.
	EventRetention string `yaml:"event_retention,omitempty"`
}

// FilteringConfig represents repository filtering configuration, including required paths, ignore files, and name patterns.
type FilteringConfig struct {
	RequiredPaths   []string `yaml:"required_paths"`   // Paths that must exist (e.g., "docs")
	IgnoreFiles     []string `yaml:"ignore_files"`     // Files that exclude repo (e.g., ".docignore")
	IncludePatterns []string `yaml:"include_patterns"` // Repository name patterns to include
	ExcludePatterns []string `yaml:"exclude_patterns"` // Repository name patterns to exclude
}

// VersioningConfig represents multi-version documentation configuration, including strategy and version limits.
type VersioningConfig struct {
	Enabled            bool               `yaml:"enabled"`               // Enable multi-version documentation
	Strategy           VersioningStrategy `yaml:"strategy"`              // typed: branches_and_tags|branches_only|tags_only
	DefaultBranchOnly  bool               `yaml:"default_branch_only"`   // Only build default branch
	BranchPatterns     []string           `yaml:"branch_patterns"`       // Branch patterns to include
	TagPatterns        []string           `yaml:"tag_patterns"`          // Tag patterns to include
	MaxVersionsPerRepo int                `yaml:"max_versions_per_repo"` // Maximum versions to keep per repo

	// enabledSpecified is set internally during load when the YAML
	// explicitly sets enabled. It lets the defaults enable versioning
	// only when the operator left the field out entirely: an operator
	// who wrote "enabled: false" means it.
	enabledSpecified bool `yaml:"-"`
}

// UnmarshalYAML is a custom unmarshal to detect if enabled was
// explicitly set by the operator.
func (v *VersioningConfig) UnmarshalYAML(unmarshal func(any) error) error {
	type raw VersioningConfig
	var aux raw
	if err := unmarshal(&aux); err != nil {
		return err
	}
	*v = VersioningConfig(aux)
	var m map[string]any
	if err := unmarshal(&m); err == nil {
		if _, ok := m["enabled"]; ok {
			v.enabledSpecified = true
		}
	}
	return nil
}

// MonitoringConfig represents monitoring and observability configuration, including metrics, health, and logging.
type MonitoringConfig struct {
	Metrics MonitoringMetrics `yaml:"metrics"`
	Health  MonitoringHealth  `yaml:"health"`
	Logging MonitoringLogging `yaml:"logging"`
}

// MonitoringMetrics represents configuration for metrics collection and endpoint path.
type MonitoringMetrics struct {
	Enabled bool   `yaml:"enabled"`
	Path    string `yaml:"path"`
}

// MonitoringHealth represents configuration for health check endpoints.
type MonitoringHealth struct {
	Path string `yaml:"path"`
}

// MonitoringLogging represents configuration for logging level and format.
type MonitoringLogging struct {
	Level  LogLevel  `yaml:"level"`
	Format LogFormat `yaml:"format"`
}

// LinkVerificationConfig represents configuration for automated link validation in daemon mode.
type LinkVerificationConfig struct {
	Enabled            bool   `yaml:"enabled"`              // Enable link verification (default: true)
	NATSURL            string `yaml:"nats_url"`             // NATS server URL
	Subject            string `yaml:"subject"`              // Subject for broken link events
	KVBucket           string `yaml:"kv_bucket"`            // KV bucket name for link cache
	CacheTTL           string `yaml:"cache_ttl"`            // TTL for successful link checks (e.g., "24h")
	CacheTTLFailures   string `yaml:"cache_ttl_failures"`   // TTL for failed link checks (e.g., "1h")
	MaxConcurrent      int    `yaml:"max_concurrent"`       // Max concurrent link checks
	RequestTimeout     string `yaml:"request_timeout"`      // HTTP request timeout
	RateLimitDelay     string `yaml:"rate_limit_delay"`     // Delay between requests
	VerifyExternalOnly bool   `yaml:"verify_external_only"` // Only verify external links
	// SkipEditLinks is a pointer: "not set" means that the edit links
	// are skipped, and an explicit false means that they are checked.
	// A plain bool cannot tell the two apart, and the default turned
	// every false into a true.
	SkipEditLinks *bool `yaml:"skip_edit_links"` // Skip edit links (require auth, default: true)
	// FollowRedirects is a pointer: "not set" means that the redirects
	// are followed, and an explicit false means that they are not. A
	// plain bool cannot tell the two apart, and the default turned
	// every false into a true.
	FollowRedirects *bool `yaml:"follow_redirects"` // Follow HTTP redirects
	MaxRedirects    int   `yaml:"max_redirects"`    // Maximum redirects to follow
}

// LoadResult contains the outcome of a Load operation, including warnings.
type LoadResult struct {
	Warnings []string
}

// Load reads and validates a configuration file (version 2.x), expanding environment variables and applying normalization and defaults.
// Load is a pure library function: it does not print, load .env files, or modify process state.
func Load(configPath string) (*Config, error) {
	_, cfg, err := LoadWithResult(configPath)
	return cfg, err
}

// LoadWithResult reads and validates a configuration file, returning warnings separately.
func LoadWithResult(configPath string) (*LoadResult, *Config, error) {
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, nil, errors.NewError(errors.CategoryConfig, "configuration file not found").
			WithContext("path", configPath).
			Build()
	}

	// #nosec G304 - configPath is from CLI argument, user-controlled
	data, err := os.ReadFile(filepath.Clean(configPath))
	if err != nil {
		return nil, nil, errors.WrapError(err, errors.CategoryConfig, "failed to read config file").
			WithContext("path", configPath).
			Build()
	}

	// Expand environment variables in the YAML content
	expandedData := expandEnv(string(data))

	config, err := unmarshalStrict([]byte(expandedData))
	if err != nil {
		return nil, nil, errors.WrapError(err, errors.CategoryConfig, "failed to unmarshal v2 config").Build()
	}

	// Validate version
	if config.Version != configVersion {
		return nil, nil, errors.NewError(errors.CategoryConfig, "unsupported configuration version").
			WithContext("actual", config.Version).
			WithContext("expected", configVersion).
			Build()
	}

	result := &LoadResult{Warnings: []string{}}

	// Normalization pass (case-fold enumerations, bounds, early coercions)
	if nres, nerr := NormalizeConfig(config); nerr != nil {
		return nil, nil, errors.WrapError(nerr, errors.CategoryConfig, "config normalization failed").Build()
	} else if nres != nil && len(nres.Warnings) > 0 {
		for _, w := range nres.Warnings {
			result.Warnings = append(result.Warnings, fmt.Sprintf("config normalization: %s", w))
		}
	}
	// Apply defaults (after normalization so canonical values drive defaults)
	if err := applyDefaults(config); err != nil {
		return nil, nil, errors.WrapError(err, errors.CategoryConfig, "failed to apply defaults").Build()
	}

	// Validate configuration
	if err := validateConfig(config); err != nil {
		return nil, nil, errors.WrapError(err, errors.CategoryConfig, "configuration validation failed").Build()
	}

	return result, config, nil
}

// applyDefaults applies default values to a Config after normalization.
func applyDefaults(config *Config) error {
	applier := NewDefaultApplier()
	return applier.ApplyDefaults(config)
}

// validateConfig validates a Config for required fields and constraints.
func validateConfig(config *Config) error {
	return ValidateConfig(config)
}

// Init writes an example configuration file (version 2.0) to the given path. If force is false, it will not overwrite existing files.
func Init(configPath string, force bool) error {
	if _, err := os.Stat(configPath); err == nil && !force {
		return errors.NewError(errors.CategoryConfig, "configuration file already exists").
			WithContext("path", configPath).
			WithContext("hint", "use --force to overwrite").
			Build()
	}

	exampleConfig := Config{
		Version: configVersion,
		Build:   BuildConfig{CloneConcurrency: 4, MaxRetries: 2, RetryBackoff: RetryBackoffLinear, RetryInitialDelay: "1s", RetryMaxDelay: defaultDuration30s},
		Daemon: &DaemonConfig{
			HTTP: HTTPConfig{
				DocsPort:       8080,
				WebhookPort:    8081,
				AdminPort:      8082,
				LiveReloadPort: 8083,
			},
			Sync: SyncConfig{
				Schedule:         defaultSyncScheduleEvery4h,
				ConcurrentBuilds: 3,
				QueueSize:        100,
			},
			Storage: StorageConfig{
				StateFile:    "./docbuilder-state.json",
				RepoCacheDir: "./repositories",
				OutputDir:    defaultOutputDir,
			},
			LinkVerification: &LinkVerificationConfig{
				Enabled:            false,
				NATSURL:            "nats://localhost:4222",
				Subject:            "docbuilder.links.broken",
				KVBucket:           "docbuilder-link-cache",
				CacheTTL:           "24h",
				CacheTTLFailures:   "1h",
				MaxConcurrent:      10,
				RequestTimeout:     defaultDuration10s,
				RateLimitDelay:     "100ms",
				VerifyExternalOnly: false,
				SkipEditLinks:      new(true),
				FollowRedirects:    new(true),
				MaxRedirects:       3,
			},
		},
		Forges: []*ForgeConfig{
			{
				Name:          "company-github",
				Type:          ForgeGitHub,
				APIURL:        "https://api.github.com",
				BaseURL:       "https://github.com",
				Organizations: []string{"your-org"},
				Auth: &AuthConfig{ //nolint:gosec // example placeholder; populated via env var expansion
					Type:  AuthTypeToken,
					Token: "${GITHUB_TOKEN}",
				},
				Webhook: &WebhookConfig{ //nolint:gosec // example placeholder; populated via env var expansion
					Secret: "${GITHUB_WEBHOOK_SECRET}",
					Path:   "/webhooks/github",
					Events: []string{"push", "repository"},
				},
			},
		},
		Filtering: &FilteringConfig{
			RequiredPaths: []string{defaultRequiredPathDocs},
			IgnoreFiles:   []string{defaultIgnoreFileDocignore},
		},
		Versioning: &VersioningConfig{
			Enabled:            true,
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
		},
		Monitoring: &MonitoringConfig{
			Metrics: MonitoringMetrics{
				Enabled: true,
				Path:    defaultMetricsPath,
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
			Directory: defaultOutputDir,
			Clean:     true,
		},
		// Example explicit repositories block (optional; forge discovery may also be used)
		Repositories: []Repository{
			{
				URL:    "https://github.com/example/repo1.git",
				Name:   "repo1",
				Branch: "main",
				Paths:  []string{defaultRequiredPathDocs},
			},
		},
	}

	data, err := yaml.Marshal(&exampleConfig)
	if err != nil {
		return errors.WrapError(err, errors.CategoryConfig, "failed to marshal example config").Build()
	}

	// #nosec G306 -- example config file for documentation purposes
	if err := os.WriteFile(configPath, data, 0o644); err != nil {
		return errors.WrapError(err, errors.CategoryConfig, "failed to write example config file").
			WithContext("path", configPath).
			Build()
	}

	return nil
}

// Historical note: legacy alias names (V2Config, LoadV2, InitV2, IsV2Config) were removed during cleanup.
