package build

import (
	"context"
	"errors"
	"log/slog"
	"time"

	appcfg "git.home.luguber.info/inful/docbuilder/internal/config"
	dberrors "git.home.luguber.info/inful/docbuilder/internal/foundation/errors"
	"git.home.luguber.info/inful/docbuilder/internal/hugo"
	"git.home.luguber.info/inful/docbuilder/internal/metrics"
	"git.home.luguber.info/inful/docbuilder/internal/observability"
	"git.home.luguber.info/inful/docbuilder/internal/state"
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

// WithRecorder injects the recorder of the build.
//
// Without it every metric of the build went to the no-op recorder, and
// the metrics of the build stayed at zero.
func (s *DefaultBuildService) WithRecorder(recorder metrics.Recorder) *DefaultBuildService {
	if recorder != nil {
		s.recorder = recorder
	}
	return s
}

// WithWorkspaceFactory allows injecting a custom workspace factory (for testing).
func (s *DefaultBuildService) WithWorkspaceFactory(factory func() *workspace.Manager) *DefaultBuildService {
	s.workspaceFactory = factory
	return s
}

// newHugoGenerator constructs the canonical Hugo site generator. Centralized
// here so both Run and RunDirect build it the same way; tests can substitute
// behavior by reaching for hugo.Generator.WithRenderer(&stages.NoopRenderer{}).
func newHugoGenerator(cfg *appcfg.Config, outputDir string, sm state.RepositoryMetadataWriter, recorder metrics.Recorder) *hugo.Generator {
	gen := hugo.NewGenerator(cfg, outputDir)
	if sm != nil {
		gen = gen.WithStateManager(sm)
	}
	if recorder != nil {
		gen = gen.SetRecorder(recorder)
	}
	return gen
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

	// Stage 1: Create workspace. The factory is responsible for choosing
	// persistent vs ephemeral; BuildRequest.KeepWorkspace only signals
	// "preserve staging dir on failure" to the generator.
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

	generator := newHugoGenerator(req.Config, req.OutputDir, req.StateManager, s.recorder)
	if req.OnDocumentReady != nil {
		generator = generator.WithDocumentReady(req.OnDocumentReady)
	}
	if req.KeepWorkspace {
		generator = generator.WithKeepStaging(true)
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

	// The pipeline skips the build itself when nothing moved: the
	// repositories are unchanged, the configuration is the same and the
	// previous output is still valid. Report that as a skip.
	if report.SkipReason != "" {
		result.Status = BuildStatusSkipped
		result.Skipped = true
		result.SkipReason = report.SkipReason
		s.recorder.IncBuildOutcome(metrics.BuildOutcomeSkipped)
		s.recorder.ObserveBuildDuration(result.Duration)
		return result, nil
	}

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

	generator := newHugoGenerator(req.Config, req.OutputDir, nil, s.recorder)
	if req.KeepStaging {
		generator = generator.WithKeepStaging(true)
	}
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
