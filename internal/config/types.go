package config

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// Repository represents a Git repository to process (shared between v2 config and generator logic)
type Repository struct {
	URL    string            `yaml:"url"`
	Name   string            `yaml:"name"`
	Branch string            `yaml:"branch,omitempty"`
	Auth   *AuthConfig       `yaml:"auth,omitempty"`
	Paths  []string          `yaml:"paths,omitempty"` // Specific paths to docs, defaults to ["docs"] (applied in defaults)
	Tags   map[string]string `yaml:"tags,omitempty"`  // Additional metadata
}

// AuthConfig represents authentication configuration
// AuthType enumerates supported authentication methods (stringly for YAML compatibility)
type AuthType string

const (
	AuthTypeNone  AuthType = "none"
	AuthTypeSSH   AuthType = "ssh"
	AuthTypeToken AuthType = "token"
	AuthTypeBasic AuthType = "basic"
)

// AuthConfig represents authentication configuration
type AuthConfig struct {
	Type     AuthType `yaml:"type"` // ssh|token|basic|none
	Username string   `yaml:"username,omitempty"`
	Password string   `yaml:"password,omitempty"`
	Token    string   `yaml:"token,omitempty"`
	KeyPath  string   `yaml:"key_path,omitempty"`
}

// IsZero reports whether no auth method specified.
func (a *AuthConfig) IsZero() bool { return a == nil || a.Type == "" || a.Type == AuthTypeNone }

// HugoConfig represents Hugo-specific configuration (embedded in V2Config)
type HugoConfig struct {
	Theme       string            `yaml:"theme,omitempty"` // raw theme string from config; normalized via ThemeType()
	BaseURL     string            `yaml:"base_url,omitempty"`
	Title       string            `yaml:"title"`
	Description string            `yaml:"description,omitempty"`
	Params      map[string]any    `yaml:"params,omitempty"`
	Menu        map[string][]Menu `yaml:"menu,omitempty"`
}

// Theme is a typed enumeration of supported Hugo theme integrations.
type Theme string

// Theme constants to avoid magic strings across generator logic.
const (
	ThemeHextra Theme = "hextra"
	ThemeDocsy  Theme = "docsy"
)

// ThemeType returns the normalized typed theme value (lowercasing the raw string).
// Unknown themes return "" so callers can branch safely.
func (h HugoConfig) ThemeType() Theme {
	s := strings.ToLower(strings.TrimSpace(h.Theme))
	switch s {
	case string(ThemeHextra):
		return ThemeHextra
	case string(ThemeDocsy):
		return ThemeDocsy
	default:
		return ""
	}
}

// BuildConfig holds build performance tuning knobs.
// Additional fields (retry limits, timeouts, etc.) can be added iteratively without
// breaking existing configurations. All zero values trigger sensible defaults.
type BuildConfig struct {
	// CloneConcurrency caps the number of repositories cloned in parallel within a single build.
	// Defaults to 4; values <1 are coerced to 1; values larger than the repo count are bounded.
	CloneConcurrency int `yaml:"clone_concurrency,omitempty"`
	// CloneStrategy selects how the clone stage treats existing repositories. fresh (default) always reclones,
	// update attempts to incrementally update existing checkouts (or clones if missing), auto chooses update when the
	// repository directory exists and fresh otherwise. This is an initial feature gate; future strategies may include
	// mirror caching or sparse checkout.
	CloneStrategy CloneStrategy `yaml:"clone_strategy,omitempty"`
	// ShallowDepth, when >0, performs shallow clones limited to the specified number of commits (git --depth semantics).
	// 0 (default) means full history. Applies to initial clone and subsequent fetch operations (best-effort) for updates.
	ShallowDepth int `yaml:"shallow_depth,omitempty"`
	// PruneNonDocPaths when true removes top-level directories in each cloned repository that are not part of any
	// configured documentation path for that repository (and preserves .git). This reduces workspace size and speeds
	// discovery for mono-repos. Only the first path segment of each docs path is preserved (e.g. for docs/api, keeps docs/).
	// Disabled by default because it may remove supporting assets referenced by docs.
	PruneNonDocPaths bool `yaml:"prune_non_doc_paths,omitempty"`
	// PruneAllow is a list of additional top-level file or directory names to always preserve when pruning
	// non-doc paths (e.g. ["README.md", "LICENSE", "assets"]). Only consulted when PruneNonDocPaths is true.
	PruneAllow []string `yaml:"prune_allow,omitempty"`
	// PruneDeny is a list of top-level file or directory names to always remove even if they would otherwise
	// be preserved (except for .git which is never removed). Deny takes precedence over allow.
	PruneDeny []string `yaml:"prune_deny,omitempty"`
	// Retry policy fields (apply to transient build failures at stage granularity)
	MaxRetries        int              `yaml:"max_retries,omitempty"`         // total retry attempts after first attempt (default 2)
	RetryBackoff      RetryBackoffMode `yaml:"retry_backoff,omitempty"`       // fixed|linear|exponential (default linear)
	RetryInitialDelay string           `yaml:"retry_initial_delay,omitempty"` // duration string (default 1s)
	RetryMaxDelay     string           `yaml:"retry_max_delay,omitempty"`     // cap for exponential (default 30s)
	// Divergence / hygiene options (future expansion): when hard reset is true we will hard reset to origin/<branch>
	// if the local branch has diverged. CleanUntracked removes untracked files after a successful update. Both default false.
	HardResetOnDiverge bool `yaml:"hard_reset_on_diverge,omitempty"`
	CleanUntracked     bool `yaml:"clean_untracked,omitempty"`
}

// CloneStrategy enumerates strategies for handling existing repository directories.
type CloneStrategy string

const (
	CloneStrategyFresh  CloneStrategy = "fresh"
	CloneStrategyUpdate CloneStrategy = "update"
	CloneStrategyAuto   CloneStrategy = "auto"
)

// NormalizeCloneStrategy canonicalizes user input returning empty string if unknown.
func NormalizeCloneStrategy(raw string) CloneStrategy {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case string(CloneStrategyFresh):
		return CloneStrategyFresh
	case string(CloneStrategyUpdate):
		return CloneStrategyUpdate
	case string(CloneStrategyAuto):
		return CloneStrategyAuto
	default:
		return ""
	}
}

// ForgeType enumerates supported forge providers.
type ForgeType string

const (
	ForgeGitHub  ForgeType = "github"
	ForgeGitLab  ForgeType = "gitlab"
	ForgeForgejo ForgeType = "forgejo"
)

// NormalizeForgeType canonicalizes a forge type string (case-insensitive) or returns empty if unknown.
func NormalizeForgeType(raw string) ForgeType {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case string(ForgeGitHub):
		return ForgeGitHub
	case string(ForgeGitLab):
		return ForgeGitLab
	case string(ForgeForgejo):
		return ForgeForgejo
	default:
		return ""
	}
}

// VersioningStrategy enumerates supported multi-version selection strategies.
type VersioningStrategy string

const (
	StrategyBranchesAndTags VersioningStrategy = "branches_and_tags"
	StrategyBranchesOnly    VersioningStrategy = "branches_only"
	StrategyTagsOnly        VersioningStrategy = "tags_only"
)

// NormalizeVersioningStrategy returns a canonical typed strategy or empty string if unknown.
func NormalizeVersioningStrategy(raw string) VersioningStrategy {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case string(StrategyBranchesAndTags):
		return StrategyBranchesAndTags
	case string(StrategyBranchesOnly):
		return StrategyBranchesOnly
	case string(StrategyTagsOnly):
		return StrategyTagsOnly
	default:
		return ""
	}
}

// RetryBackoffMode enumerates supported backoff strategies for retries.
type RetryBackoffMode string

const (
	RetryBackoffFixed       RetryBackoffMode = "fixed"
	RetryBackoffLinear      RetryBackoffMode = "linear"
	RetryBackoffExponential RetryBackoffMode = "exponential"
)

// NormalizeRetryBackoff converts arbitrary user input (case-insensitive) into a typed mode, returning empty string for unknown.
func NormalizeRetryBackoff(raw string) RetryBackoffMode {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case string(RetryBackoffFixed):
		return RetryBackoffFixed
	case string(RetryBackoffLinear):
		return RetryBackoffLinear
	case string(RetryBackoffExponential):
		return RetryBackoffExponential
	default:
		return ""
	}
}

// Menu represents a Hugo menu item
type Menu struct {
	Name   string `yaml:"name"`
	URL    string `yaml:"url"`
	Weight int    `yaml:"weight,omitempty"`
}

// LogLevel enumerates supported logging levels (subset; mapping to slog or zap later).
type LogLevel string

const (
	LogLevelDebug LogLevel = "debug"
	LogLevelInfo  LogLevel = "info"
	LogLevelWarn  LogLevel = "warn"
	LogLevelError LogLevel = "error"
)

func NormalizeLogLevel(raw string) LogLevel {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case string(LogLevelDebug):
		return LogLevelDebug
	case string(LogLevelInfo):
		return LogLevelInfo
	case string(LogLevelWarn):
		return LogLevelWarn
	case string(LogLevelError):
		return LogLevelError
	default:
		return ""
	}
}

// LogFormat enumerates supported log output formats.
type LogFormat string

const (
	LogFormatJSON LogFormat = "json"
	LogFormatText LogFormat = "text"
)

func NormalizeLogFormat(raw string) LogFormat {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case string(LogFormatJSON):
		return LogFormatJSON
	case string(LogFormatText):
		return LogFormatText
	default:
		return ""
	}
}

// OutputConfig represents output configuration
type OutputConfig struct {
	Directory string `yaml:"directory"`
	Clean     bool   `yaml:"clean"` // Clean output directory before build
}

// loadEnvFile loads environment variables from .env/.env.local files (shared with v2 loader)
func loadEnvFile() error {
	envPaths := []string{".env", ".env.local"}
	for _, envPath := range envPaths {
		if err := loadSingleEnvFile(envPath); err == nil {
			fmt.Fprintf(os.Stderr, "Loaded environment variables from %s\n", envPath)
			return nil
		}
	}
	return fmt.Errorf("no .env file found")
}

// loadSingleEnvFile loads environment variables from a single file
func loadSingleEnvFile(filename string) error {
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return err
	}
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if len(value) >= 2 {
			if (value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'') {
				value = value[1 : len(value)-1]
			}
		}
		if os.Getenv(key) == "" {
			os.Setenv(key, value)
		}
	}
	return scanner.Err()
}
