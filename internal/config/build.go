package config

import "strings"

// BuildConfig holds build performance tuning knobs and retry/cleanup options.
type BuildConfig struct {
	CloneConcurrency   int              `yaml:"clone_concurrency,omitempty"`
	CloneStrategy      CloneStrategy    `yaml:"clone_strategy,omitempty"`
	ShallowDepth       int              `yaml:"shallow_depth,omitempty"`
	PruneNonDocPaths   bool             `yaml:"prune_non_doc_paths,omitempty"`
	PruneAllow         []string         `yaml:"prune_allow,omitempty"`
	PruneDeny          []string         `yaml:"prune_deny,omitempty"`
	MaxRetries         int              `yaml:"max_retries,omitempty"`
	RetryBackoff       RetryBackoffMode `yaml:"retry_backoff,omitempty"`
	RetryInitialDelay  string           `yaml:"retry_initial_delay,omitempty"`
	RetryMaxDelay      string           `yaml:"retry_max_delay,omitempty"`
	HardResetOnDiverge bool             `yaml:"hard_reset_on_diverge,omitempty"`
	CleanUntracked     bool             `yaml:"clean_untracked,omitempty"`
	WorkspaceDir       string           `yaml:"workspace_dir,omitempty"`
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
