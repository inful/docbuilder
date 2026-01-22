package queue

import (
	"net/http"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/hugo/models"
	"git.home.luguber.info/inful/docbuilder/internal/services"
)

// LiveReloadHub is the minimal interface the daemon uses for live reload.
//
// This is intentionally defined here (instead of referencing internal/server/httpserver)
// to keep build queue types free of server package dependencies.
type LiveReloadHub interface {
	http.Handler
	Broadcast(hash string)
	Shutdown()
}

// BuildJobMetadata holds typed metadata for build jobs.
//
// Note: this is intentionally small and focused on build pipeline inputs/outputs.
// Additional daemon-only concerns should remain in higher layers.
type BuildJobMetadata struct {
	V2Config     *config.Config      `json:"v2_config,omitempty"`
	Repositories []config.Repository `json:"repositories,omitempty"`

	// Delta analysis
	DeltaRepoReasons map[string]string `json:"delta_repo_reasons,omitempty"`

	// State management
	StateManager services.StateManager `json:"-"`

	// Live reload
	LiveReloadHub LiveReloadHub `json:"-"`

	// Build report (populated after completion)
	BuildReport *models.BuildReport `json:"build_report,omitempty"`
}

// EnsureTypedMeta returns job.TypedMeta, initializing it if nil.
func EnsureTypedMeta(job *BuildJob) *BuildJobMetadata {
	if job.TypedMeta == nil {
		job.TypedMeta = &BuildJobMetadata{}
	}
	return job.TypedMeta
}
