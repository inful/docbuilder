// Package build provides the canonical build execution pipeline for DocBuilder.
// All execution paths (CLI, daemon, tests) should route through BuildService.
package build

import (
	"context"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// BuildService is the canonical interface for executing documentation builds.
// Both CLI and daemon/server should implement thin wrappers over this interface.
type BuildService interface {
	// Run executes a complete build pipeline: clone → discover → transform → generate.
	// Returns a BuildResult with detailed outcomes and any error encountered.
	Run(ctx context.Context, req BuildRequest) (*BuildResult, error)
}

// BuildRequest contains all inputs required to execute a documentation build.
type BuildRequest struct {
	// Config is the loaded configuration for this build.
	Config *config.Config

	// OutputDir is the target directory for the generated Hugo site.
	OutputDir string

	// Incremental enables incremental updates (git pull vs fresh clone).
	Incremental bool

	// Options provides optional build behavior modifiers.
	Options BuildOptions
}

// BuildOptions provides optional configuration for build behavior.
type BuildOptions struct {
	// Verbose enables detailed logging during the build.
	Verbose bool

	// DryRun simulates the build without writing output.
	DryRun bool

	// SkipIfUnchanged enables skip evaluation when content hasn't changed.
	SkipIfUnchanged bool

	// Concurrency controls parallel repository processing (0 = sequential).
	Concurrency int
}

// BuildResult contains the outcome of a build execution.
type BuildResult struct {
	// Status indicates overall build outcome.
	Status BuildStatus

	// Report contains detailed build metrics and diagnostics (type: *hugo.BuildReport).
	// Using any to avoid import cycles; callers should type-assert as needed.
	Report any

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

	// BuildStatusCancelled indicates the build was cancelled.
	BuildStatusCancelled BuildStatus = "cancelled"
)

// IsTerminal returns true if the status represents a final state.
func (s BuildStatus) IsTerminal() bool {
	return s == BuildStatusSuccess || s == BuildStatusFailed ||
		s == BuildStatusSkipped || s == BuildStatusCancelled
}

// IsSuccess returns true if the build completed successfully.
func (s BuildStatus) IsSuccess() bool {
	return s == BuildStatusSuccess || s == BuildStatusSkipped
}
