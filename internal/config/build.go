package config

import "strings"

// BuildConfig holds build performance tuning knobs and retry/cleanup options.
type BuildConfig struct {
	CloneConcurrency   int              `yaml:"clone_concurrency,omitempty"`
	CloneStrategy      CloneStrategy    `yaml:"clone_strategy,omitempty"`
	NamespaceForges    NamespacingMode  `yaml:"namespace_forges,omitempty"` // auto|always|never (governs forge directory prefixing)
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
	SkipIfUnchanged    bool             `yaml:"skip_if_unchanged,omitempty"`
	RenderMode         RenderMode       `yaml:"render_mode,omitempty"` // auto|always|never (preferred over legacy env DOCBUILDER_RUN_HUGO / DOCBUILDER_SKIP_HUGO)
}

// NamespacingMode controls whether forge-level directory names are included in content paths.
type NamespacingMode string

const (
	NamespacingAuto   NamespacingMode = "auto"   // include forge only when >1 distinct forge types present
	NamespacingAlways NamespacingMode = "always" // always prefix with forge (when available)
	NamespacingNever  NamespacingMode = "never"  // never prefix (even if ambiguous across forges)
)

// NormalizeNamespacingMode canonicalizes user input returning empty string if unknown.
func NormalizeNamespacingMode(raw string) NamespacingMode {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case string(NamespacingAuto):
		return NamespacingAuto
	case string(NamespacingAlways):
		return NamespacingAlways
	case string(NamespacingNever):
		return NamespacingNever
	default:
		return ""
	}
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

// RenderMode controls whether the external Hugo binary is invoked after scaffold generation.
// auto: (default) legacy environment variable behavior is honored.
// always: always attempt to run hugo (unless binary missing).
// never: never run hugo (generate scaffold only).
type RenderMode string

const (
	RenderModeAuto   RenderMode = "auto"
	RenderModeAlways RenderMode = "always"
	RenderModeNever  RenderMode = "never"
)

// NormalizeRenderMode canonicalizes user input returning empty string if unknown.
func NormalizeRenderMode(raw string) RenderMode {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case string(RenderModeAuto):
		return RenderModeAuto
	case string(RenderModeAlways):
		return RenderModeAlways
	case string(RenderModeNever):
		return RenderModeNever
	default:
		return ""
	}
}
