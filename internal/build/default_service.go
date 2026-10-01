package build

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/build/validation"
	appcfg "git.home.luguber.info/inful/docbuilder/internal/config"
	dberrors "git.home.luguber.info/inful/docbuilder/internal/foundation/errors"
	"git.home.luguber.info/inful/docbuilder/internal/hugo"
	"git.home.luguber.info/inful/docbuilder/internal/metrics"
	"git.home.luguber.info/inful/docbuilder/internal/observability"
	"git.home.luguber.info/inful/docbuilder/internal/workspace"
)

// DefaultBuildService is the standard implementation of BuildService.
// It orchestrates the full pipeline: workspace → git clone → discovery → hugo generation.
type DefaultBuildService struct {
	// workspaceFactory optionally overrides the workspace Manager creation.
	// Most callers should leave this nil and let NewBuildService install
	// the default (workspace.NewManager("")).
	workspaceFactory func() *workspace.Manager

	recorder metrics.Recorder
}

// NewBuildService creates a new DefaultBuildService with default factories.
func NewBuildService() *DefaultBuildService {
	return &DefaultBuildService{
		workspaceFactory: func() *workspace.Manager {
			return workspace.NewManager("")
		},
		recorder: metrics.NoopRecorder{},
	}
}

// WithWorkspaceFactory allows injecting a custom workspace factory (for testing).
func (s *DefaultBuildService) WithWorkspaceFactory(factory func() *workspace.Manager) *DefaultBuildService {
	s.workspaceFactory = factory
	return s
}

// newHugoGenerator constructs the canonical Hugo site generator. Centralized
// here so both Run and RunDirect build it the same way; tests can substitute
// behavior by reaching for hugo.Generator.WithRenderer(&stages.NoopRenderer{}).
func newHugoGenerator(cfg *appcfg.Config, outputDir string) *hugo.Generator {
	return hugo.NewGenerator(cfg, outputDir)
}

// Run executes the complete build pipeline.
func (s *DefaultBuildService) Run(ctx context.Context, req BuildRequest) (*BuildResult, error) {
	startTime := time.Now()

	result := &BuildResult{
		StartTime:  startTime,
		OutputPath: req.OutputDir,
	}

	// Add build context for observability
	buildID := startTime.Format("20060102-150405")
	ctx = observability.WithBuildID(ctx, buildID)

	// Validate request
	if req.Config == nil {
		result.Status = BuildStatusFailed
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(startTime)
		s.recorder.IncBuildOutcome(metrics.BuildOutcomeFailed)
		return result, dberrors.ConfigError("config required").Build()
	}

	if len(req.Config.Repositories) == 0 {
		observability.WarnContext(ctx, "No repositories configured for build")
		result.Status = BuildStatusSuccess
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(startTime)
		s.recorder.IncBuildOutcome(metrics.BuildOutcomeSuccess)
		s.recorder.ObserveBuildDuration(result.Duration)
		return result, nil
	}

	// Stage 0: Skip evaluation (optional — only when both SkipIfUnchanged
	// is requested and the caller supplied SkipState access).
	if req.Options.SkipIfUnchanged && req.SkipState != nil {
		skipResult := s.evaluateSkip(ctx, req, startTime)
		if skipResult != nil {
			return skipResult, nil
		}
	}

	// Stage 1: Create workspace
	stageStart := time.Now()
	ctx = observability.WithStage(ctx, "workspace")
	observability.InfoContext(ctx, "Creating build workspace")
	wsManager := s.workspaceFactory()
	if err := wsManager.Create(); err != nil {
		result.Status = BuildStatusFailed
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(startTime)
		s.recorder.IncStageResult("workspace", metrics.ResultFatal)
		s.recorder.IncBuildOutcome(metrics.BuildOutcomeFailed)
		return result, dberrors.FileSystemError("failed to create workspace").WithContext("error", err.Error()).Build()
	}
	s.recorder.ObserveStageDuration("workspace", time.Since(stageStart))
	s.recorder.IncStageResult("workspace", metrics.ResultSuccess)
	defer func() {
		if err := wsManager.Cleanup(); err != nil {
			observability.WarnContext(ctx, "Failed to cleanup workspace", slog.String("error", err.Error()))
		}
	}()

	// Stage 2+: Unified Site Generation (Clone -> Discovery -> Transform -> Hugo)
	// We delegate the heavy lifting to the natively refactored hugo.Generator pipeline.

	// Override CloneStrategy if Incremental flag is set to ensure backward compatibility
	// with callers (like CLI) that use the Incremental flag.
	if req.Incremental && req.Config.Build.CloneStrategy == appcfg.CloneStrategyFresh {
		req.Config.Build.CloneStrategy = appcfg.CloneStrategyUpdate
	}

	generator := newHugoGenerator(req.Config, req.OutputDir)
	if req.OnDocumentReady != nil {
		generator = generator.WithDocumentReady(req.OnDocumentReady)
	}
	report, err := generator.GenerateFullSite(ctx, req.Config.Repositories, wsManager.GetPath())

	result.Report = report
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(startTime)

	if err != nil {
		result.Status = BuildStatusFailed
		s.recorder.IncBuildOutcome(metrics.BuildOutcomeFailed)
		return result, err
	}

	if report == nil {
		result.Status = BuildStatusFailed
		return result, errors.New("generator returned nil report without error")
	}

	// Map report back to result primitives for legacy listeners
	result.Status = BuildStatusSuccess
	result.Repositories = report.Repositories
	result.FilesProcessed = report.Files
	result.RepositoriesSkipped = report.FailedRepositories

	s.recorder.IncBuildOutcome(metrics.BuildOutcomeSuccess)
	s.recorder.ObserveBuildDuration(result.Duration)

	return result, nil
}

// RunDirect executes the build pipeline from already-discovered doc files.
// It mirrors Run for the parts that exist in the direct path: validate,
// build the generator, hand off to GenerateSiteWithReportContext, then
// translate the report into a BuildResult.
//
// Direct builds cannot be skipped (the caller has the doc files in hand,
// not a repo state to diff). Workspace creation is also unnecessary —
// the direct path doesn't clone.
func (s *DefaultBuildService) RunDirect(ctx context.Context, req DirectBuildRequest) (*BuildResult, error) {
	startTime := time.Now()

	result := &BuildResult{
		StartTime:  startTime,
		OutputPath: req.OutputDir,
	}

	if req.Config == nil {
		result.Status = BuildStatusFailed
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(startTime)
		s.recorder.IncBuildOutcome(metrics.BuildOutcomeFailed)
		return result, dberrors.ConfigError("config required").Build()
	}

	generator := newHugoGenerator(req.Config, req.OutputDir)
	report, err := generator.GenerateSiteWithReportContext(ctx, req.DocFiles)
	result.Report = report
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(startTime)

	if err != nil {
		result.Status = BuildStatusFailed
		s.recorder.IncBuildOutcome(metrics.BuildOutcomeFailed)
		return result, err
	}

	if report == nil {
		result.Status = BuildStatusFailed
		return result, errors.New("generator returned nil report without error")
	}

	result.Status = BuildStatusSuccess
	result.Repositories = report.Repositories
	result.FilesProcessed = report.Files
	result.RepositoriesSkipped = report.FailedRepositories

	s.recorder.IncBuildOutcome(metrics.BuildOutcomeSuccess)
	s.recorder.ObserveBuildDuration(result.Duration)

	return result, nil
}

// evaluateSkip performs skip evaluation and returns a result if build should be skipped.
// Returns nil if build should proceed.
func (s *DefaultBuildService) evaluateSkip(ctx context.Context, req BuildRequest, startTime time.Time) *BuildResult {
	stageStart := time.Now()
	ctx = observability.WithStage(ctx, "skip_evaluation")
	observability.InfoContext(ctx, "Evaluating if build can be skipped")

	generator := newHugoGenerator(req.Config, req.OutputDir)
	evaluator := validation.NewSkipEvaluator(req.OutputDir, req.SkipState, generator)

	skipReport, canSkip := evaluator.Evaluate(ctx, req.Config.Repositories)
	s.recorder.ObserveStageDuration("skip_evaluation", time.Since(stageStart))

	if !canSkip {
		observability.InfoContext(ctx, "Skip evaluation complete - proceeding with build")
		return nil
	}

	// Build should be skipped
	observability.InfoContext(ctx, "Build skipped - no changes detected")
	result := &BuildResult{
		Status:     BuildStatusSkipped,
		Skipped:    true,
		SkipReason: "no_changes",
		Report:     skipReport,
		EndTime:    time.Now(),
	}
	result.Duration = result.EndTime.Sub(startTime)
	s.recorder.IncBuildOutcome(metrics.BuildOutcomeSkipped)
	s.recorder.ObserveBuildDuration(result.Duration)
	return result
}
