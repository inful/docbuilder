package config

import (
	"git.home.luguber.info/inful/docbuilder/internal/foundation/normalization"
)

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
	RenderMode         RenderMode       `yaml:"render_mode,omitempty"`      // auto|always|never (source of truth for Hugo execution)
	DetectDeletions    bool             `yaml:"detect_deletions,omitempty"` // enable unchanged repo deletion scan during partial recomposition
	LiveReload         bool             `yaml:"live_reload,omitempty"`      // enable SSE livereload endpoint & script (development only)
	// detectDeletionsSpecified is set internally during load when the YAML explicitly sets detect_deletions.
	// This lets defaults apply (true) only when user omitted the field entirely.
	detectDeletionsSpecified bool `yaml:"-"`
}

// Custom unmarshal to detect if detect_deletions was explicitly set by user.
func (b *BuildConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type raw BuildConfig
	var aux raw
	if err := unmarshal(&aux); err != nil {
		return err
	}
	// Copy back
	*b = BuildConfig(aux)
	// Heuristic: if the YAML explicitly mentioned the key, the unmarshaller will have set the bool (even if false).
	// We can't directly know presence, so we re-unmarshal into a generic map to check.
	var m map[string]interface{}
	if err := unmarshal(&m); err == nil {
		if _, ok := m["detect_deletions"]; ok {
			b.detectDeletionsSpecified = true
		}
	}
	return nil
}

// (type BuildConfig is defined above to satisfy typeDefFirst linter)

// NamespacingMode controls whether forge-level directory names are included in content paths.
type NamespacingMode string

const (
	NamespacingAuto   NamespacingMode = "auto"   // include forge only when >1 distinct forge types present
	NamespacingAlways NamespacingMode = "always" // always prefix with forge (when available)
	NamespacingNever  NamespacingMode = "never"  // never prefix (even if ambiguous across forges)
)

var namespacingModeNormalizer = normalization.NewNormalizer(map[string]NamespacingMode{
	"auto":   NamespacingAuto,
	"always": NamespacingAlways,
	"never":  NamespacingNever,
}, "")

// NormalizeNamespacingMode canonicalizes user input returning empty string if unknown.
func NormalizeNamespacingMode(raw string) NamespacingMode {
	return namespacingModeNormalizer.Normalize(raw)
}

// CloneStrategy enumerates strategies for handling existing repository directories.
type CloneStrategy string

const (
	CloneStrategyFresh  CloneStrategy = "fresh"
	CloneStrategyUpdate CloneStrategy = "update"
	CloneStrategyAuto   CloneStrategy = "auto"
)

var cloneStrategyNormalizer = normalization.NewNormalizer(map[string]CloneStrategy{
	"fresh":  CloneStrategyFresh,
	"update": CloneStrategyUpdate,
	"auto":   CloneStrategyAuto,
}, "")

// NormalizeCloneStrategy canonicalizes user input returning empty string if unknown.
func NormalizeCloneStrategy(raw string) CloneStrategy {
	return cloneStrategyNormalizer.Normalize(raw)
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

var renderModeNormalizer = normalization.NewNormalizer(map[string]RenderMode{
	"auto":   RenderModeAuto,
	"always": RenderModeAlways,
	"never":  RenderModeNever,
}, "")

// NormalizeRenderMode canonicalizes user input returning empty string if unknown.
func NormalizeRenderMode(raw string) RenderMode {
	return renderModeNormalizer.Normalize(raw)
}
