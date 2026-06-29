// Package queue documents the relationship between BuildService (the
// canonical pipeline shape in internal/build) and Builder (the queue's
// narrower contract). See the type doc comment below; the adapter is the
// deliberate seam between them.
//
// What this file owns
// -------------------
//
// BuildServiceAdapter is the canonical bridge between two single-method
// interfaces that have intentionally different signatures:
//
//   - build.BuildService.Run(ctx, BuildRequest) -> *BuildResult
//     is the canonical input/output pair. BuildRequest is the CLI's
//     input shape: Config + OutputDir + flags. CLI logs reach into
//     BuildResult fields the queue contract throws away
//     (RepositoriesSkipped, Duration, OutputPath, etc.).
//
//   - queue.Builder.Build(ctx, *BuildJob) -> *BuildReport is the queue
//     shape. BuildJob is the lifecycle envelope -- priority, type,
//     status, TypedMeta carrying the embedded config -- and
//     BuildReport is the per-stage report the queue eventually
//     emits.
//
// Plan review-overlapping-functionality.md M14 called out the
// divergence and asked whether to collapse the two interfaces (option
// a) or move BuildService into the queue's package so the adapter can
// be reasoned about in one place (option b). We took option (c)
// ("accept the adapter and document it") because the divergence is
// load-bearing: it keeps queue-shaped orchestration separate from
// CLI-shaped pipeline invocation, and it isolates queue concurrency
// concerns from pipeline reporting.
//
// Future option (a) would unify by adding Config + OutputDir +
// Incremental + Options to BuildJob, deleting BuildRequest/Result, and
// making every CLI caller construct a queue-shaped job. That's a bigger
// refactor than fits in a Pass-D cleanup; this comment is the marker.
package queue

import (
	"context"
	"errors"
	"path/filepath"
	"sync"

	"git.home.luguber.info/inful/docbuilder/internal/build"
	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/hugo/models"
)

const defaultSiteDir = "./site"

// BuildServiceAdapter adapts the canonical BuildService.Run(BuildRequest) ->
// *BuildResult to the queue's narrower Builder.Build(BuildJob) ->
// *BuildReport. See the package comment above for the design rationale.
//
// Job -> Request translation pipeline:
//
//  1. Extract the embedded Config from TypedMeta (fail the build if
//     the job forgot to attach one -- this is an orchestration bug,
//     not a user input bug).
//  2. Let the job override cfg.Repositories (ADR-021 orchestration
//     flows enqueue jobs that carry a snapshot of repos; cfg alone
//     may be empty in forge mode).
//  3. Resolve OutputDir from cfg.Output.Directory plus BaseDirectory.
//  4. Set Incremental=true (the daemon is always incremental) and
//     forward SkipIfUnchanged from cfg.Build.
//  5. Run the canonical pipeline and return only BuildReport (the
//     queue's downstream contract).
//
// Compile-time assertion at the bottom of this file: *BuildServiceAdapter
// implements Builder.
type BuildServiceAdapter struct {
	inner build.BuildService
	mu    sync.Mutex
}

// NewBuildServiceAdapter creates a new adapter wrapping a BuildService.
func NewBuildServiceAdapter(svc build.BuildService) *BuildServiceAdapter {
	return &BuildServiceAdapter{inner: svc}
}

// Build implements the Builder interface by delegating to BuildService.
func (a *BuildServiceAdapter) Build(ctx context.Context, job *BuildJob) (*models.BuildReport, error) {
	if job == nil {
		return nil, errors.New("build job is nil")
	}

	// Daemon serves a single output directory. Serializing builds here prevents concurrent
	// build jobs (via BuildQueue workers) from clobbering shared staging/output paths.
	a.mu.Lock()
	defer a.mu.Unlock()

	// Extract configuration from TypedMeta
	var cfg *config.Config
	if job.TypedMeta != nil && job.TypedMeta.V2Config != nil {
		cfg = job.TypedMeta.V2Config
	}
	if cfg == nil {
		return nil, errors.New("build job has no configuration")
	}

	// If the job carries an explicit repository set, prefer it over cfg.Repositories.
	// This enables orchestration flows (ADR-021) to enqueue canonical full-site builds
	// in forge mode where cfg.Repositories may be empty.
	if job.TypedMeta != nil && len(job.TypedMeta.Repositories) > 0 {
		cfgCopy := *cfg
		cfgCopy.Repositories = job.TypedMeta.Repositories
		if len(job.TypedMeta.RepoSnapshot) > 0 {
			for i := range cfgCopy.Repositories {
				repo := &cfgCopy.Repositories[i]
				if sha, ok := job.TypedMeta.RepoSnapshot[repo.URL]; ok && sha != "" {
					repo.PinnedCommit = sha
				}
			}
		}
		cfg = &cfgCopy
	}

	// Extract output directory and combine with base_directory if set
	outDir := cfg.Output.Directory
	if outDir == "" {
		outDir = defaultSiteDir
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

	return result.Report, nil
}

// ensure BuildServiceAdapter implements Builder.
var _ Builder = (*BuildServiceAdapter)(nil)
