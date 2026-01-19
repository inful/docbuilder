// Package daemon provides the daemon-mode build queue and orchestration.
//
// The daemon uses BuildServiceAdapter (wrapping build.DefaultBuildService) as the
// primary Builder implementation. The Builder interface is used by BuildQueue
// and allows for alternative implementations (distributed builders, dry-run, etc.).
package daemon

import (
	"context"

	"git.home.luguber.info/inful/docbuilder/internal/hugo/models"
)

// Builder defines an abstraction for executing a build job and returning a BuildReport.
// It decouples queue execution from the concrete site generation pipeline, enabling
// future swapping (e.g., distributed builders, parallel clone variants, dry-run builder).
//
// The primary implementation is BuildServiceAdapter (see build_service_adapter.go).
// Legacy implementation SiteBuilder was removed in Dec 2025.
type Builder interface {
	Build(ctx context.Context, job *BuildJob) (*models.BuildReport, error)
}
