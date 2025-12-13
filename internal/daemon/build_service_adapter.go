package daemon

import (
	"context"
	"path/filepath"
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

	// For discovery builds, use the discovered repositories instead of config file repos
	if job.Type == BuildTypeDiscovery && job.TypedMeta != nil && len(job.TypedMeta.Repositories) > 0 {
		// Create a copy of the config to avoid modifying the original
		cfgCopy := *cfg
		cfgCopy.Repositories = job.TypedMeta.Repositories
		cfg = &cfgCopy
	}

	// Extract output directory and combine with base_directory if set
	outDir := cfg.Output.Directory
	if outDir == "" {
		outDir = "./site"
	}
	// If base_directory is set and outDir is relative, combine them
	if cfg.Output.BaseDirectory != "" && !filepath.IsAbs(outDir) {
		outDir = filepath.Join(cfg.Output.BaseDirectory, outDir)
	}

	// Build the request
	req := build.BuildRequest{
		Config:      cfg,
		OutputDir:   outDir,
		Incremental: true, // Daemon mode uses incremental updates to leverage remote HEAD cache
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
