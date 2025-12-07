package daemon

import (
	"context"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/build"
	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/hugo"
)

// BuildServiceAdapter adapts the daemon's Builder interface to build.BuildService.
// This enables the daemon to use the canonical build pipeline while maintaining
// compatibility with the existing job-based architecture.
type BuildServiceAdapter struct {
	inner build.BuildService
}

// NewBuildServiceAdapter creates a new adapter wrapping a BuildService.
func NewBuildServiceAdapter(svc build.BuildService) *BuildServiceAdapter {
	return &BuildServiceAdapter{inner: svc}
}

// Build implements the Builder interface by delegating to BuildService.
func (a *BuildServiceAdapter) Build(ctx context.Context, job *BuildJob) (*hugo.BuildReport, error) {
	if job == nil {
		return nil, nil
	}

	// Extract configuration from TypedMeta
	var cfg *config.Config
	if job.TypedMeta != nil && job.TypedMeta.V2Config != nil {
		cfg = job.TypedMeta.V2Config
	}
	if cfg == nil {
		return nil, nil
	}

	// Extract output directory
	outDir := cfg.Output.Directory
	if outDir == "" {
		outDir = "./site"
	}

	// Build the request
	req := build.BuildRequest{
		Config:    cfg,
		OutputDir: outDir,
		Options: build.BuildOptions{
			SkipIfUnchanged: cfg.Build.SkipIfUnchanged,
		},
	}

	// Execute the build
	result, err := a.inner.Run(ctx, req)
	if err != nil {
		return nil, err
	}

	// Convert result to BuildReport
	report := &hugo.BuildReport{
		Repositories: result.Repositories,
		Files:        result.FilesProcessed,
		Start:        result.StartTime,
		End:          result.EndTime,
	}

	// Set outcome based on status
	switch result.Status {
	case build.BuildStatusSuccess:
		report.Outcome = hugo.OutcomeSuccess
	case build.BuildStatusFailed:
		report.Outcome = hugo.OutcomeFailed
	case build.BuildStatusSkipped:
		report.Outcome = hugo.OutcomeSuccess
		report.SkipReason = result.SkipReason
	case build.BuildStatusCancelled:
		report.Outcome = hugo.OutcomeCanceled
	}

	// Store StageDurations
	if report.StageDurations == nil {
		report.StageDurations = make(map[string]time.Duration)
	}
	report.StageDurations["total"] = result.Duration

	return report, nil
}

// ensure BuildServiceAdapter implements Builder
var _ Builder = (*BuildServiceAdapter)(nil)
