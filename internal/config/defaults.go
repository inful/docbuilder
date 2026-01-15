package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// DefaultApplier applies defaults for a specific configuration domain.
type DefaultApplier interface {
	ApplyDefaults(cfg *Config) error
	Domain() string
}

// BuildDefaultApplier handles Build configuration defaults.
type BuildDefaultApplier struct{}

func (b *BuildDefaultApplier) Domain() string { return "build" }

func (b *BuildDefaultApplier) ApplyDefaults(cfg *Config) error {
	if cfg.Build.CloneConcurrency <= 0 {
		cfg.Build.CloneConcurrency = 4
	}

	// Render mode default (auto) if unspecified or invalid
	if cfg.Build.RenderMode == "" {
		cfg.Build.RenderMode = RenderModeAuto
	} else {
		if rm := NormalizeRenderMode(string(cfg.Build.RenderMode)); rm != "" {
			cfg.Build.RenderMode = rm
		} else {
			cfg.Build.RenderMode = RenderModeAuto
		}
	}

	// Forge namespacing default (auto): only add forge directory when multiple forge types present
	if cfg.Build.NamespaceForges == "" {
		cfg.Build.NamespaceForges = NamespacingAuto
	} else {
		m := NormalizeNamespacingMode(string(cfg.Build.NamespaceForges))
		if m == "" {
			cfg.Build.NamespaceForges = NamespacingAuto
		} else {
			cfg.Build.NamespaceForges = m
		}
	}

	// ShallowDepth:
	// - If omitted: default to a very shallow clone (1) since DocBuilder typically needs only current docs.
	// - If explicitly set: respect the user value (including 0 to disable).
	// - Negative coerced to 0.
	if cfg.Build.ShallowDepth < 0 {
		cfg.Build.ShallowDepth = 0
	}
	if !cfg.Build.shallowDepthSpecified && cfg.Build.ShallowDepth == 0 {
		cfg.Build.ShallowDepth = 1
	}

	// Deletion detection default: enable only if user omitted the field entirely.
	if !cfg.Build.detectDeletionsSpecified && !cfg.Build.DetectDeletions {
		cfg.Build.DetectDeletions = true
	}

	// Clone strategy default: fresh (explicit destructive clone) unless user supplied a valid strategy.
	if cfg.Build.CloneStrategy == "" {
		cfg.Build.CloneStrategy = CloneStrategyFresh
	} else {
		cs := NormalizeCloneStrategy(string(cfg.Build.CloneStrategy))
		if cs != "" {
			cfg.Build.CloneStrategy = cs
		} else {
			cfg.Build.CloneStrategy = CloneStrategyFresh // fallback
		}
	}

	if cfg.Build.MaxRetries < 0 {
		cfg.Build.MaxRetries = 0
	}
	if cfg.Build.MaxRetries == 0 { // default 2 retries (3 total attempts) unless explicitly set >0
		cfg.Build.MaxRetries = 2
	}

	if cfg.Build.RetryBackoff == "" {
		cfg.Build.RetryBackoff = RetryBackoffLinear
	} else {
		// normalize any user-provided raw string
		cfg.Build.RetryBackoff = NormalizeRetryBackoff(string(cfg.Build.RetryBackoff))
		if cfg.Build.RetryBackoff == "" { // fallback to default if unknown
			cfg.Build.RetryBackoff = RetryBackoffLinear
		}
	}

	if cfg.Build.RetryInitialDelay == "" {
		cfg.Build.RetryInitialDelay = "1s"
	}
	if cfg.Build.RetryMaxDelay == "" {
		cfg.Build.RetryMaxDelay = "30s"
	}

	return nil
}

// HugoDefaultApplier handles Hugo configuration defaults.
type HugoDefaultApplier struct{}

func (h *HugoDefaultApplier) Domain() string { return "hugo" }

func (h *HugoDefaultApplier) ApplyDefaults(cfg *Config) error {
	if cfg.Hugo.Title == "" {
		cfg.Hugo.Title = "Documentation Portal"
	}
	// Theme is always Relearn - no longer configurable
	return nil
}

// OutputDefaultApplier handles Output configuration defaults.
type OutputDefaultApplier struct{}

func (o *OutputDefaultApplier) Domain() string { return "output" }

func (o *OutputDefaultApplier) ApplyDefaults(cfg *Config) error {
	if cfg.Output.Directory == "" {
		cfg.Output.Directory = "./site"
	}
	cfg.Output.Clean = true // Always clean in daemon mode

	// When base_directory is set and workspace_dir is not explicitly configured,
	// default workspace_dir to {base_directory}/workspace
	if cfg.Output.BaseDirectory != "" && cfg.Build.WorkspaceDir == "" {
		cfg.Build.WorkspaceDir = filepath.Join(cfg.Output.BaseDirectory, "workspace")
	}

	// Warn if user configured workspace_dir equal to output directory
	if cfg.Build.WorkspaceDir != "" {
		wd := filepath.Clean(cfg.Build.WorkspaceDir)
		od := filepath.Clean(cfg.Output.Directory)
		if wd == od {
			fmt.Fprintf(os.Stderr, "Warning: build.workspace_dir (%s) matches output.directory (%s); this may mix git working trees with generated site artifacts. Consider using a separate directory.\n", wd, od)
		}
	}

	return nil
}

// DaemonDefaultApplier handles Daemon configuration defaults.
type DaemonDefaultApplier struct{}

func (d *DaemonDefaultApplier) Domain() string { return "daemon" }

func (d *DaemonDefaultApplier) ApplyDefaults(cfg *Config) error {
	if cfg.Daemon == nil {
		return nil // No daemon config to apply defaults to
	}

	// Set daemon-specific build defaults
	// In daemon mode, always render by default (unless explicitly disabled)
	if cfg.Build.RenderMode == "" || cfg.Build.RenderMode == RenderModeAuto {
		cfg.Build.RenderMode = RenderModeAlways
	}

	// Enable skip evaluation by default in daemon mode for efficiency
	// This prevents unnecessary rebuilds when nothing has changed
	if !cfg.Build.SkipIfUnchanged {
		cfg.Build.SkipIfUnchanged = true
	}

	// Note: LiveReload is NOT enabled by default in daemon mode
	// It's designed for local development with file watching, not production serving
	// Enable it explicitly in config if you want live reload on rebuild events

	if cfg.Daemon.HTTP.DocsPort == 0 {
		cfg.Daemon.HTTP.DocsPort = 8080
	}
	if cfg.Daemon.HTTP.WebhookPort == 0 {
		cfg.Daemon.HTTP.WebhookPort = 8081
	}
	if cfg.Daemon.HTTP.AdminPort == 0 {
		cfg.Daemon.HTTP.AdminPort = 8082
	}
	if cfg.Daemon.HTTP.LiveReloadPort == 0 {
		cfg.Daemon.HTTP.LiveReloadPort = 8083
	}
	if cfg.Daemon.Sync.Schedule == "" {
		cfg.Daemon.Sync.Schedule = "0 */4 * * *" // Every 4 hours
	}
	if cfg.Daemon.Sync.ConcurrentBuilds == 0 {
		cfg.Daemon.Sync.ConcurrentBuilds = 3
	}
	if cfg.Daemon.Sync.QueueSize == 0 {
		cfg.Daemon.Sync.QueueSize = 100
	}
	if cfg.Daemon.Storage.StateFile == "" {
		cfg.Daemon.Storage.StateFile = "./docbuilder-state.json"
	}
	if cfg.Daemon.Storage.RepoCacheDir == "" {
		cfg.Daemon.Storage.RepoCacheDir = "./repositories"
	}
	if cfg.Daemon.Storage.OutputDir == "" {
		cfg.Daemon.Storage.OutputDir = cfg.Output.Directory
	}

	// Link verification defaults
	if cfg.Daemon.LinkVerification == nil {
		cfg.Daemon.LinkVerification = &LinkVerificationConfig{}
	}
	lv := cfg.Daemon.LinkVerification
	if !lv.Enabled {
		lv.Enabled = true // Default enabled
	}
	if lv.NATSURL == "" {
		lv.NATSURL = "nats://localhost:4222"
	}
	if lv.Subject == "" {
		lv.Subject = "docbuilder.links.broken"
	}
	if lv.KVBucket == "" {
		lv.KVBucket = "docbuilder-link-cache"
	}
	if lv.CacheTTL == "" {
		lv.CacheTTL = "24h"
	}
	if lv.CacheTTLFailures == "" {
		lv.CacheTTLFailures = "1h"
	}
	if lv.MaxConcurrent == 0 {
		lv.MaxConcurrent = 10
	}
	if lv.RequestTimeout == "" {
		lv.RequestTimeout = "10s"
	}
	if lv.RateLimitDelay == "" {
		lv.RateLimitDelay = "100ms"
	}
	// VerifyExternalOnly defaults to false (verify both internal and external)
	// SkipEditLinks defaults to true (edit links require authentication)
	if !lv.SkipEditLinks {
		lv.SkipEditLinks = true // Default enabled
	}
	if !lv.FollowRedirects {
		lv.FollowRedirects = true // Default enabled
	}
	if lv.MaxRedirects == 0 {
		lv.MaxRedirects = 3
	}

	return nil
}

// FilteringDefaultApplier handles Filtering configuration defaults.
type FilteringDefaultApplier struct{}

func (f *FilteringDefaultApplier) Domain() string { return "filtering" }

func (f *FilteringDefaultApplier) ApplyDefaults(cfg *Config) error {
	if cfg.Filtering == nil {
		cfg.Filtering = &FilteringConfig{}
	}

	// Distinguish between nil slice and explicitly empty slice
	if cfg.Filtering.RequiredPaths == nil {
		cfg.Filtering.RequiredPaths = []string{"docs"}
	}
	if len(cfg.Filtering.IgnoreFiles) == 0 {
		cfg.Filtering.IgnoreFiles = []string{".docignore"}
	}

	return nil
}

// VersioningDefaultApplier handles Versioning configuration defaults.
type VersioningDefaultApplier struct{}

func (v *VersioningDefaultApplier) Domain() string { return "versioning" }

func (v *VersioningDefaultApplier) ApplyDefaults(cfg *Config) error {
	if cfg.Versioning == nil {
		cfg.Versioning = &VersioningConfig{}
	}

	// If strategy is explicitly provided, implicitly enable versioning
	if cfg.Versioning.Strategy != "" && !cfg.Versioning.Enabled {
		cfg.Versioning.Enabled = true
	}

	// Apply defaults regardless of enabled status for test consistency
	// This ensures defaults are always available even when versioning is disabled
	if cfg.Versioning.Strategy == "" {
		cfg.Versioning.Strategy = StrategyBranchesAndTags
	} else {
		orig := cfg.Versioning.Strategy
		norm := NormalizeVersioningStrategy(string(cfg.Versioning.Strategy))
		if norm != "" {
			cfg.Versioning.Strategy = norm
		} else {
			// Preserve original invalid value so validateConfig can raise an error
			cfg.Versioning.Strategy = orig
		}
	}

	if cfg.Versioning.MaxVersionsPerRepo == 0 {
		cfg.Versioning.MaxVersionsPerRepo = 10
	}

	return nil
}

// MonitoringDefaultApplier handles Monitoring configuration defaults.
type MonitoringDefaultApplier struct{}

func (m *MonitoringDefaultApplier) Domain() string { return "monitoring" }

func (m *MonitoringDefaultApplier) ApplyDefaults(cfg *Config) error {
	if cfg.Monitoring == nil {
		cfg.Monitoring = &MonitoringConfig{}
	}

	if cfg.Monitoring.Metrics.Path == "" {
		cfg.Monitoring.Metrics.Path = "/metrics"
	}
	if cfg.Monitoring.Health.Path == "" {
		cfg.Monitoring.Health.Path = "/health"
	}
	if cfg.Monitoring.Logging.Level == "" {
		cfg.Monitoring.Logging.Level = LogLevelInfo
	} else {
		lvl := NormalizeLogLevel(string(cfg.Monitoring.Logging.Level))
		if lvl != "" {
			cfg.Monitoring.Logging.Level = lvl
		}
	}
	if cfg.Monitoring.Logging.Format == "" {
		cfg.Monitoring.Logging.Format = LogFormatJSON
	} else {
		fmtVal := NormalizeLogFormat(string(cfg.Monitoring.Logging.Format))
		if fmtVal != "" {
			cfg.Monitoring.Logging.Format = fmtVal
		}
	}

	return nil
}

// RepositoryDefaultApplier handles Repository configuration defaults.
type RepositoryDefaultApplier struct{}

func (r *RepositoryDefaultApplier) Domain() string { return "repositories" }

func (r *RepositoryDefaultApplier) ApplyDefaults(cfg *Config) error {
	for i := range cfg.Repositories {
		if len(cfg.Repositories[i].Paths) == 0 {
			cfg.Repositories[i].Paths = []string{"docs"}
		}
		if cfg.Repositories[i].Branch == "" {
			cfg.Repositories[i].Branch = "main"
		}
	}

	return nil
}
