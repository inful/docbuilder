package build

import (
	"context"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/build/validation"
	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/docs"
	"git.home.luguber.info/inful/docbuilder/internal/hugo/models"
)

// BuildService is the canonical interface for executing documentation builds.
// Both CLI and daemon/server should implement thin wrappers over this interface.
type BuildService interface {
	// Run executes a complete build pipeline: clone → discover → transform → generate.
	// Returns a BuildResult with detailed outcomes and any error encountered.
	Run(ctx context.Context, req BuildRequest) (*BuildResult, error)

	// RunDirect executes the build pipeline from already-discovered doc files,
	// skipping the clone and discovery stages. Used by callers that have
	// produced the file list themselves (e.g. docbuilder preview, or
	// docbuilder build -d against a local directory).
	RunDirect(ctx context.Context, req DirectBuildRequest) (*BuildResult, error)
}

// BuildRequest contains all inputs required to execute a documentation build.
type BuildRequest struct {
	// Config is the loaded configuration for this build.
	Config *config.Config

	// OutputDir is the target directory for the generated Hugo site.
	OutputDir string

	// Incremental enables incremental updates (git pull vs fresh clone).
	Incremental bool

	// SkipState optionally enables skip-evaluation by providing access to
	// the persisted build state. nil disables skip-evaluation regardless
	// of Options.SkipIfUnchanged. The daemon passes its state manager
	// here; the CLI omits it.
	SkipState validation.SkipStateAccess

	// OnDocumentReady, when non-nil, is installed on the constructed
	// hugo.Generator and invoked once per non-generated document
	// immediately after it has been written to disk. The daemon uses
	// this to push documents to its outbound dispatcher; nil (CLI) is
	// the common case.
	OnDocumentReady func(content []byte, path string)

	// Options provides optional build behavior modifiers.
	Options BuildOptions
}

// DirectBuildRequest is for callers that have already discovered the doc
// files (e.g. docbuilder preview, or docbuilder build -d against a local
// directory). It skips the clone and discovery stages and goes straight
// to the content pipeline.
type DirectBuildRequest struct {
	// Config is the loaded configuration for this build.
	Config *config.Config

	// OutputDir is the target directory for the generated Hugo site.
	OutputDir string

	// DocFiles are the already-discovered documentation files to process.
	DocFiles []docs.DocFile

	// Options provides optional build behavior modifiers.
	Options BuildOptions
}

// BuildOptions provides optional configuration for build behavior.
type BuildOptions struct {
	// Verbose enables detailed logging during the build.
	Verbose bool

	// SkipIfUnchanged enables skip evaluation when content hasn't changed.
	SkipIfUnchanged bool
}

// BuildResult contains the outcome of a build execution.
type BuildResult struct {
	// Status indicates overall build outcome.
	Status BuildStatus

	// Report contains detailed build metrics and diagnostics.
	Report *models.BuildReport

	// OutputPath is the final output directory (may differ from request).
	OutputPath string

	// Repositories is the count of processed repositories.
	Repositories int

	// RepositoriesSkipped is the count of repositories that failed to clone/process.
	RepositoriesSkipped int

	// FilesProcessed is the count of documentation files handled.
	FilesProcessed int

	// Duration is the total build execution time.
	Duration time.Duration

	// StartTime is when the build started.
	StartTime time.Time

	// EndTime is when the build completed.
	EndTime time.Time

	// Skipped indicates the build was skipped due to no changes.
	Skipped bool

	// SkipReason explains why the build was skipped (if Skipped is true).
	SkipReason string
}

// BuildStatus represents the outcome of a build execution.
type BuildStatus string

const (
	// BuildStatusSuccess indicates the build completed successfully.
	BuildStatusSuccess BuildStatus = "success"

	// BuildStatusFailed indicates the build encountered an error.
	BuildStatusFailed BuildStatus = "failed"

	// BuildStatusSkipped indicates the build was skipped (e.g., no changes).
	BuildStatusSkipped BuildStatus = "skipped"

	// BuildStatusCancelled indicates the build was canceled.
	BuildStatusCancelled BuildStatus = "canceled"
)

// IsSuccess returns true if the build completed successfully.
func (s BuildStatus) IsSuccess() bool {
	return s == BuildStatusSuccess || s == BuildStatusSkipped
}
