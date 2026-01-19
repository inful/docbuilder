package build

import (
	"context"
	"errors"
	"log/slog"
	"time"

	appcfg "git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/docs"
	dberrors "git.home.luguber.info/inful/docbuilder/internal/foundation/errors"
	"git.home.luguber.info/inful/docbuilder/internal/hugo/models"
	"git.home.luguber.info/inful/docbuilder/internal/metrics"
	"git.home.luguber.info/inful/docbuilder/internal/observability"
	"git.home.luguber.info/inful/docbuilder/internal/workspace"
)

// HugoGenerator is the interface for Hugo site generation (avoids import cycle with hugo package).
type HugoGenerator interface {
	GenerateSite(docFiles []docs.DocFile) error
	GenerateFullSite(ctx context.Context, repositories []appcfg.Repository, workspaceDir string) (*models.BuildReport, error)
}

// HugoGeneratorFactory creates a HugoGenerator for a given configuration and output directory.
type HugoGeneratorFactory func(cfg *appcfg.Config, outputDir string) HugoGenerator

// SkipEvaluator evaluates whether a build can be skipped due to no changes.
// Returns a skip report and true if skip is possible, otherwise nil and false.
type SkipEvaluator interface {
	Evaluate(ctx context.Context, repos []appcfg.Repository) (report *models.BuildReport, canSkip bool)
}

// SkipEvaluatorFactory creates a SkipEvaluator for a given output directory.
// The factory pattern allows lazy creation with the correct output directory.
type SkipEvaluatorFactory func(outputDir string) SkipEvaluator

// DefaultBuildService is the standard implementation of BuildService.
// It orchestrates the full pipeline: workspace → git clone → discovery → hugo generation.
type DefaultBuildService struct {
	// Optional dependencies that can be injected
	workspaceFactory     func() *workspace.Manager
	hugoGeneratorFactory HugoGeneratorFactory
	skipEvaluatorFactory SkipEvaluatorFactory
	recorder             metrics.Recorder
}

// NewBuildService creates a new DefaultBuildService with default factories.
func NewBuildService() *DefaultBuildService {
	return &DefaultBuildService{
		workspaceFactory: func() *workspace.Manager {
			return workspace.NewManager("")
		},
		recorder: metrics.NoopRecorder{},
		// hugoGeneratorFactory must be set via WithHugoGeneratorFactory to avoid import cycle
	}
}

// WithWorkspaceFactory allows injecting a custom workspace factory (for testing).
func (s *DefaultBuildService) WithWorkspaceFactory(factory func() *workspace.Manager) *DefaultBuildService {
	s.workspaceFactory = factory
	return s
}

// WithHugoGeneratorFactory sets the factory for creating Hugo generators.
func (s *DefaultBuildService) WithHugoGeneratorFactory(factory HugoGeneratorFactory) *DefaultBuildService {
	s.hugoGeneratorFactory = factory
	return s
}

// WithSkipEvaluatorFactory sets the factory for creating skip evaluators.
// When set and Options.SkipIfUnchanged is true, the service will check
// if the build can be skipped before executing the full pipeline.
func (s *DefaultBuildService) WithSkipEvaluatorFactory(factory SkipEvaluatorFactory) *DefaultBuildService {
	s.skipEvaluatorFactory = factory
	return s
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

	// Stage 0: Skip evaluation (optional)
	if req.Options.SkipIfUnchanged && s.skipEvaluatorFactory != nil {
		skipResult := s.evaluateSkip(ctx, req, startTime)
		if skipResult != nil {
			return skipResult, nil
		}
	} else {
		s.logSkipEvaluationDisabled(ctx, req, s.skipEvaluatorFactory)
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
	if s.hugoGeneratorFactory == nil {
		result.Status = BuildStatusFailed
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(startTime)
		s.recorder.IncBuildOutcome(metrics.BuildOutcomeFailed)
		return result, dberrors.ConfigError("hugo generator factory required").Build()
	}

	// Override CloneStrategy if Incremental flag is set to ensure backward compatibility
	// with callers (like CLI) that use the Incremental flag.
	if req.Incremental && req.Config.Build.CloneStrategy == appcfg.CloneStrategyFresh {
		req.Config.Build.CloneStrategy = appcfg.CloneStrategyUpdate
	}

	generator := s.hugoGeneratorFactory(req.Config, req.OutputDir)
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

// evaluateSkip performs skip evaluation and returns a result if build should be skipped.
// Returns nil if build should proceed.
func (s *DefaultBuildService) evaluateSkip(ctx context.Context, req BuildRequest, startTime time.Time) *BuildResult {
	stageStart := time.Now()
	ctx = observability.WithStage(ctx, "skip_evaluation")
	observability.InfoContext(ctx, "Evaluating if build can be skipped")

	evaluator := s.skipEvaluatorFactory(req.OutputDir)
	if evaluator == nil {
		observability.WarnContext(ctx, "Skip evaluator factory returned nil - skipping evaluation disabled")
		s.recorder.ObserveStageDuration("skip_evaluation", time.Since(stageStart))
		observability.InfoContext(ctx, "Skip evaluation complete - proceeding with build")
		return nil
	}

	skipReport, canSkip := evaluator.Evaluate(ctx, req.Config.Repositories)
	if !canSkip {
		s.recorder.ObserveStageDuration("skip_evaluation", time.Since(stageStart))
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
	s.recorder.ObserveStageDuration("skip_evaluation", time.Since(stageStart))
	s.recorder.IncBuildOutcome(metrics.BuildOutcomeSkipped)
	s.recorder.ObserveBuildDuration(result.Duration)
	return result
}

// logSkipEvaluationDisabled logs why skip evaluation is disabled.
func (s *DefaultBuildService) logSkipEvaluationDisabled(ctx context.Context, req BuildRequest, factory func(string) SkipEvaluator) {
	if !req.Options.SkipIfUnchanged {
		observability.DebugContext(ctx, "Skip evaluation disabled - SkipIfUnchanged=false")
	}
	if factory == nil {
		observability.WarnContext(ctx, "Skip evaluator factory not configured - cannot evaluate skip conditions")
	}
}
