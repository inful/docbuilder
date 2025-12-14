package build

import (
	"context"
	"log/slog"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/docs"
	dberrors "git.home.luguber.info/inful/docbuilder/internal/foundation/errors"
	"git.home.luguber.info/inful/docbuilder/internal/git"
	"git.home.luguber.info/inful/docbuilder/internal/metrics"
	"git.home.luguber.info/inful/docbuilder/internal/observability"
	"git.home.luguber.info/inful/docbuilder/internal/workspace"
)

// HugoGenerator is the interface for Hugo site generation (avoids import cycle with hugo package).
type HugoGenerator interface {
	GenerateSite(docFiles []docs.DocFile) error
}

// HugoGeneratorFactory creates a HugoGenerator for a given configuration and output directory.
type HugoGeneratorFactory func(cfg any, outputDir string) HugoGenerator

// SkipEvaluator evaluates whether a build can be skipped due to no changes.
// Returns a skip report and true if skip is possible, otherwise nil and false.
type SkipEvaluator interface {
	Evaluate(repos []any) (report any, canSkip bool)
}

// SkipEvaluatorFactory creates a SkipEvaluator for a given output directory.
// The factory pattern allows lazy creation with the correct output directory.
type SkipEvaluatorFactory func(outputDir string) SkipEvaluator

// DefaultBuildService is the standard implementation of BuildService.
// It orchestrates the full pipeline: workspace → git clone → discovery → hugo generation.
type DefaultBuildService struct {
	// Optional dependencies that can be injected
	workspaceFactory     func() *workspace.Manager
	gitClientFactory     func(path string) *git.Client
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
		gitClientFactory: func(path string) *git.Client {
			return git.NewClient(path)
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

// WithGitClientFactory allows injecting a custom git client factory (for testing).
func (s *DefaultBuildService) WithGitClientFactory(factory func(path string) *git.Client) *DefaultBuildService {
	s.gitClientFactory = factory
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
		stageStart := time.Now()
		ctx = observability.WithStage(ctx, "skip_evaluation")
		observability.InfoContext(ctx, "Evaluating if build can be skipped")

		evaluator := s.skipEvaluatorFactory(req.OutputDir)
		if evaluator != nil {
			// Convert repositories to []any for the generic interface
			repos := make([]any, len(req.Config.Repositories))
			for i, repo := range req.Config.Repositories {
				repos[i] = repo
			}

			if skipReport, canSkip := evaluator.Evaluate(repos); canSkip {
				observability.InfoContext(ctx, "Build skipped - no changes detected")
				result.Status = BuildStatusSkipped
				result.Skipped = true
				result.SkipReason = "no_changes"
				result.Report = skipReport
				result.EndTime = time.Now()
				result.Duration = result.EndTime.Sub(startTime)
				s.recorder.ObserveStageDuration("skip_evaluation", time.Since(stageStart))
				s.recorder.IncBuildOutcome(metrics.BuildOutcomeSkipped)
				s.recorder.ObserveBuildDuration(result.Duration)
				return result, nil
			}
		}
		s.recorder.ObserveStageDuration("skip_evaluation", time.Since(stageStart))
		observability.InfoContext(ctx, "Skip evaluation complete - proceeding with build")
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

	// Stage 2: Clone/update repositories
	stageStart = time.Now()
	ctx = observability.WithStage(ctx, "clone")
	observability.InfoContext(ctx, "Processing repositories",
		slog.Int("count", len(req.Config.Repositories)),
		slog.Bool("incremental", req.Incremental))
	gitClient := s.gitClientFactory(wsManager.GetPath())
	if err := gitClient.EnsureWorkspace(); err != nil {
		result.Status = BuildStatusFailed
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(startTime)
		s.recorder.IncStageResult("clone", metrics.ResultFatal)
		s.recorder.IncBuildOutcome(metrics.BuildOutcomeFailed)
		return result, dberrors.FileSystemError("failed to ensure git initialized").WithContext("error", err.Error()).Build()
	}

	repoPaths := make(map[string]string)
	for _, repo := range req.Config.Repositories {
		// Check for cancellation
		select {
		case <-ctx.Done():
			result.Status = BuildStatusCancelled
			result.EndTime = time.Now()
			result.Duration = result.EndTime.Sub(startTime)
			s.recorder.IncStageResult("clone", metrics.ResultCanceled)
			s.recorder.IncBuildOutcome(metrics.BuildOutcomeCanceled)
			return result, ctx.Err()
		default:
		}

		repoStart := time.Now()
		observability.InfoContext(ctx, "Processing repository",
			slog.String("name", repo.Name),
			slog.String("url", repo.URL))

		var repoPath string
		var err error

		if req.Incremental {
			repoPath, err = gitClient.UpdateRepo(repo)
		} else {
			repoPath, err = gitClient.CloneRepo(repo)
		}

		if err != nil {
			observability.ErrorContext(ctx, "Failed to process repository",
				slog.String("name", repo.Name),
				slog.String("error", err.Error()))
			// Log the error but continue with remaining repositories
			s.recorder.ObserveCloneRepoDuration(repo.Name, time.Since(repoStart), false)
			s.recorder.IncCloneRepoResult(false)
			// Track this as a skipped repository, not a fatal error
			result.RepositoriesSkipped++
			continue
		}

		s.recorder.ObserveCloneRepoDuration(repo.Name, time.Since(repoStart), true)
		s.recorder.IncCloneRepoResult(true)
		repoPaths[repo.Name] = repoPath
		observability.InfoContext(ctx, "Repository processed",
			slog.String("name", repo.Name),
			slog.String("path", repoPath))
	}
	s.recorder.ObserveStageDuration("clone", time.Since(stageStart))
	s.recorder.IncStageResult("clone", metrics.ResultSuccess)

	result.Repositories = len(repoPaths)
	if result.RepositoriesSkipped > 0 {
		observability.InfoContext(ctx, "Repository processing completed",
			slog.Int("successful", len(repoPaths)),
			slog.Int("skipped", result.RepositoriesSkipped),
			slog.Int("total", len(req.Config.Repositories)))
	} else {
		observability.InfoContext(ctx, "All repositories processed", slog.Int("count", len(repoPaths)))
	}

	// Stage 3: Discover documentation files
	stageStart = time.Now()
	ctx = observability.WithStage(ctx, "discovery")
	observability.InfoContext(ctx, "Starting documentation discovery")
	discovery := docs.NewDiscovery(req.Config.Repositories, &req.Config.Build)

	docFiles, err := discovery.DiscoverDocs(repoPaths)
	if err != nil {
		result.Status = BuildStatusFailed
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(startTime)
		s.recorder.IncStageResult("discovery", metrics.ResultFatal)
		s.recorder.IncBuildOutcome(metrics.BuildOutcomeFailed)
		return result, dberrors.BuildError("discovery failed").WithContext("error", err.Error()).Build()
	}
	s.recorder.ObserveStageDuration("discovery", time.Since(stageStart))
	s.recorder.IncStageResult("discovery", metrics.ResultSuccess)

	if len(docFiles) == 0 {
		observability.WarnContext(ctx, "No documentation files found in any repository")
		result.Status = BuildStatusSuccess
		result.FilesProcessed = 0
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(startTime)
		s.recorder.IncBuildOutcome(metrics.BuildOutcomeWarning)
		s.recorder.ObserveBuildDuration(result.Duration)
		return result, nil
	}

	result.FilesProcessed = len(docFiles)

	// Log discovery summary
	filesByRepo := discovery.GetDocFilesByRepository()
	for repoName, files := range filesByRepo {
		observability.DebugContext(ctx, "Documentation files by repository",
			slog.String("repository", repoName),
			slog.Int("files", len(files)))
	}

	// Stage 4: Generate Hugo site
	stageStart = time.Now()
	ctx = observability.WithStage(ctx, "hugo")
	observability.InfoContext(ctx, "Generating Hugo site",
		slog.String("output", req.OutputDir),
		slog.Int("files", len(docFiles)))

	if s.hugoGeneratorFactory == nil {
		result.Status = BuildStatusFailed
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(startTime)
		s.recorder.IncStageResult("hugo", metrics.ResultFatal)
		s.recorder.IncBuildOutcome(metrics.BuildOutcomeFailed)
		return result, dberrors.ConfigError("hugo generator factory required").Build()
	}

	generator := s.hugoGeneratorFactory(req.Config, req.OutputDir)

	if err := generator.GenerateSite(docFiles); err != nil {
		observability.ErrorContext(ctx, "Failed to generate Hugo site",
			slog.String("error", err.Error()))
		result.Status = BuildStatusFailed
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(startTime)
		s.recorder.ObserveStageDuration("hugo", time.Since(stageStart))
		s.recorder.IncStageResult("hugo", metrics.ResultFatal)
		s.recorder.IncBuildOutcome(metrics.BuildOutcomeFailed)
		return result, dberrors.HugoError("hugo generation failed").WithContext("error", err.Error()).Build()
	}
	s.recorder.ObserveStageDuration("hugo", time.Since(stageStart))
	s.recorder.IncStageResult("hugo", metrics.ResultSuccess)

	observability.InfoContext(ctx, "Hugo site generated successfully",
		slog.String("output", req.OutputDir))

	result.Status = BuildStatusSuccess
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(startTime)
	s.recorder.IncBuildOutcome(metrics.BuildOutcomeSuccess)
	s.recorder.ObserveBuildDuration(result.Duration)

	return result, nil
}
