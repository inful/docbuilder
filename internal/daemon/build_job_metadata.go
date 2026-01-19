package daemon

import (
	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/hugo/models"
	"git.home.luguber.info/inful/docbuilder/internal/services"
)

// BuildJobMetadata represents typed metadata for build jobs.
// This struct replaces the legacy map[string]interface{} pattern
// for compile-time type safety.
type BuildJobMetadata struct {
	// Core build configuration
	V2Config     *config.Config      `json:"v2_config,omitempty"`
	Repositories []config.Repository `json:"repositories,omitempty"`

	// State management
	StateManager services.StateManager `json:"-"` // Interface, don't serialize

	// Delta analysis
	DeltaRepoReasons map[string]string `json:"delta_repo_reasons,omitempty"`

	// Metrics and monitoring
	MetricsCollector *MetricsCollector `json:"-"` // Pointer to live collector

	// Live reload
	LiveReloadHub *LiveReloadHub `json:"-"` // Pointer to live hub

	// Build report (populated after completion)
	BuildReport *models.BuildReport `json:"build_report,omitempty"`
}

// EnsureTypedMeta returns job.TypedMeta, initializing it if nil.
// This helper enables gradual migration from Metadata map to TypedMeta.
func EnsureTypedMeta(job *BuildJob) *BuildJobMetadata {
	if job.TypedMeta == nil {
		job.TypedMeta = &BuildJobMetadata{}
	}
	return job.TypedMeta
}
