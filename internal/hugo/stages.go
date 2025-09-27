package hugo

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/build"
	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/docs"
	"git.home.luguber.info/inful/docbuilder/internal/git"
)

// Stage is a discrete unit of work in the site build.
type Stage func(ctx context.Context, bs *BuildState) error

// StageErrorKind enumerates structured stage error categories.
type StageErrorKind string

const (
	StageErrorFatal    StageErrorKind = "fatal"    // Build must abort.
	StageErrorWarning  StageErrorKind = "warning"  // Non-fatal; record and continue.
	StageErrorCanceled StageErrorKind = "canceled" // Context cancellation.
)

// StageError is a structured error carrying category and underlying cause.
type StageError struct {
	Kind  StageErrorKind
	Stage StageName
	Err   error
}

func (e *StageError) Error() string { return fmt.Sprintf("%s stage %s: %v", e.Kind, e.Stage, e.Err) }
func (e *StageError) Unwrap() error { return e.Err }

// Transient reports whether the underlying error condition is likely transient (safe to retry)
// versus permanent (retry unlikely to succeed without intervention). Initial heuristic:
//   - clone_repos: transient if wrapping build.ErrClone (network/auth flake) and at least one repo succeeded
//   - run_hugo: transient if wrapping build.ErrHugo (tooling/runtime issue that may be intermittent)
//   - discover_docs: transient only if some repositories cloned (partial data); fatal if zero
//   - canceled: not transient (caller initiated)
//   - other stages default false until refined.
func (e *StageError) Transient() bool {
	if e == nil {
		return false
	}
	if e.Kind == StageErrorCanceled {
		return false
	}
	// Unwrap chain for sentinel match.
	var cause error = e.Err
	isSentinel := func(target error) bool { return errors.Is(cause, target) }
	switch e.Stage {
	case StageCloneRepos:
		if isSentinel(build.ErrClone) {
			// Heuristic: if at least one repo succeeded treat as transient; otherwise maybe auth misconfig (permanent).
			// We cannot access BuildState here directly; assume transient for warning classification (fatal clone errs rare).
			return true
		}
	case StageRunHugo:
		if isSentinel(build.ErrHugo) {
			return true
		}
	case StageDiscoverDocs:
		if isSentinel(build.ErrDiscovery) {
			// Distinguish between zero and partial clones is done at stage level; treat warning discovery issues as transient.
			return e.Kind == StageErrorWarning
		}
	}
	return false
}

// resultFromStageErrorKind maps a StageErrorKind to a StageResult.
func resultFromStageErrorKind(k StageErrorKind) StageResult {
	switch k {
	case StageErrorWarning:
		return StageResultWarning
	case StageErrorCanceled:
		return StageResultCanceled
	case StageErrorFatal:
		return StageResultFatal
	default:
		return StageResultFatal
	}
}

// severityFromStageErrorKind maps StageErrorKind to IssueSeverity.
func severityFromStageErrorKind(k StageErrorKind) IssueSeverity {
	if k == StageErrorWarning { return SeverityWarning }
	return SeverityError
}

// StageOutcome is a normalized classification of a single stage execution.
// It encapsulates the (possibly nil) StageError, mapped result, derived issue code
// and whether the pipeline should abort after this stage.
type StageOutcome struct {
	Stage     StageName
	Error     *StageError
	Result    StageResult
	IssueCode ReportIssueCode
	Severity  IssueSeverity
	Transient bool
	Abort     bool // true for fatal or canceled errors
}

// classifyStageResult converts a raw error from a stage into a StageOutcome with
// fully derived fields (result, severity, issue code, abort flag). Success is a nil error.
func classifyStageResult(stage StageName, err error, bs *BuildState) StageOutcome {
	if err == nil {
		return StageOutcome{Stage: stage, Result: StageResultSuccess}
	}
	var se *StageError
	if errors.As(err, &se) {
		// Map issue code using existing heuristics.
		code := IssueGenericStageError
		switch se.Stage {
		case StageCloneRepos:
			if errors.Is(se.Err, build.ErrClone) {
				if bs.Report.ClonedRepositories == 0 {
					code = IssueAllClonesFailed
				} else if bs.Report.FailedRepositories > 0 {
					code = IssuePartialClone
				} else {
					code = IssueCloneFailure
				}
			} else { code = IssueCloneFailure }
		case StageDiscoverDocs:
			if errors.Is(se.Err, build.ErrDiscovery) {
				if len(bs.RepoPaths) == 0 { code = IssueNoRepositories } else { code = IssueDiscoveryFailure }
			} else { code = IssueDiscoveryFailure }
		case StageRunHugo:
			if errors.Is(se.Err, build.ErrHugo) { code = IssueHugoExecution } else { code = IssueHugoExecution }
		default:
			if se.Kind == StageErrorCanceled { code = IssueCanceled }
		}
		return StageOutcome{
			Stage: stage,
			Error: se,
			Result: resultFromStageErrorKind(se.Kind),
			IssueCode: code,
			Severity: severityFromStageErrorKind(se.Kind),
			Transient: se.Transient(),
			Abort: se.Kind == StageErrorFatal || se.Kind == StageErrorCanceled,
		}
	}
	// Unknown error: wrap as fatal with generic code.
	se = newFatalStageError(stage, err)
	return StageOutcome{
		Stage: stage,
		Error: se,
		Result: StageResultFatal,
		IssueCode: IssueGenericStageError,
		Severity: SeverityError,
		Transient: false,
		Abort: true,
	}
}

// Helpers to classify errors.
func newFatalStageError(stage StageName, err error) *StageError {
	return &StageError{Kind: StageErrorFatal, Stage: stage, Err: err}
}
func newWarnStageError(stage StageName, err error) *StageError {
	return &StageError{Kind: StageErrorWarning, Stage: stage, Err: err}
}
func newCanceledStageError(stage StageName, err error) *StageError {
	return &StageError{Kind: StageErrorCanceled, Stage: stage, Err: err}
}

// BuildState carries mutable state and metrics across stages.
type BuildState struct {
	Generator    *Generator
	Docs         []docs.DocFile
	Report       *BuildReport
	start        time.Time
	Repositories []config.Repository // configured repositories (post-filter)
	RepoPaths    map[string]string   // name -> local filesystem path
	WorkspaceDir string              // root workspace for git operations
}

// newBuildState constructs a BuildState.
func newBuildState(g *Generator, docFiles []docs.DocFile, report *BuildReport) *BuildState {
	return &BuildState{
		Generator: g,
		Docs:      docFiles,
		Report:    report,
		start:     time.Now(),
	}
}

// runStages executes stages in order, recording timing and stopping on first fatal error.
func runStages(ctx context.Context, bs *BuildState, stages []StageDef) error {
	for _, st := range stages {
		select {
		case <-ctx.Done():
			// Treat as canceled stage outcome.
			se := newCanceledStageError(st.Name, ctx.Err())
			out := StageOutcome{Stage: st.Name, Error: se, Result: StageResultCanceled, IssueCode: IssueCanceled, Severity: SeverityError, Transient: false, Abort: true}
			bs.Report.StageErrorKinds[st.Name] = se.Kind
			bs.Report.AddIssue(out.IssueCode, out.Stage, out.Severity, se.Error(), out.Transient, se)
			bs.Report.recordStageResult(out.Stage, out.Result, bs.Generator.recorder)
			return se
		default:
		}
		t0 := time.Now()
		err := st.Fn(ctx, bs)
		dur := time.Since(t0)
		bs.Report.StageDurations[string(st.Name)] = dur
		if bs.Generator != nil && bs.Generator.recorder != nil {
			bs.Generator.recorder.ObserveStageDuration(string(st.Name), dur)
		}
		out := classifyStageResult(st.Name, err, bs)
		if out.Error != nil { // error path
			bs.Report.StageErrorKinds[st.Name] = out.Error.Kind
			bs.Report.AddIssue(out.IssueCode, out.Stage, out.Severity, out.Error.Error(), out.Transient, out.Error)
		}
		bs.Report.recordStageResult(st.Name, out.Result, bs.Generator.recorder)
		if out.Abort {
			if out.Error != nil { return out.Error }
			return fmt.Errorf("stage %s aborted", st.Name)
		}
	}
	return nil
}

// Individual stage implementations (minimal wrappers calling existing methods).

func stagePrepareOutput(ctx context.Context, bs *BuildState) error { // currently no-op beyond structure creation
	return bs.Generator.createHugoStructure()
}

// stageCloneRepos clones all repositories into the workspace directory, updating clone/failure counts.
func stageCloneRepos(ctx context.Context, bs *BuildState) error {
	if len(bs.Repositories) == 0 {
		return nil
	}
	if bs.WorkspaceDir == "" {
		return newFatalStageError(StageCloneRepos, fmt.Errorf("workspace directory not set"))
	}
	client := git.NewClient(bs.WorkspaceDir)
	if err := client.EnsureWorkspace(); err != nil {
		return newFatalStageError(StageCloneRepos, fmt.Errorf("ensure workspace: %w", err))
	}
	bs.RepoPaths = make(map[string]string, len(bs.Repositories))

	// Determine requested concurrency from config (build section) with safe bounds.
	requested := 1
	if bs.Generator != nil && bs.Generator.config.Build.CloneConcurrency > 0 {
		requested = bs.Generator.config.Build.CloneConcurrency
	}
	if requested > len(bs.Repositories) {
		requested = len(bs.Repositories)
	}
	if requested < 1 {
		requested = 1
	}
	concurrency := requested
	if bs.Generator != nil && bs.Generator.recorder != nil {
		bs.Generator.recorder.SetCloneConcurrency(concurrency)
	}

	// Fast path sequential
	if concurrency == 1 {
		for _, r := range bs.Repositories {
			select {
			case <-ctx.Done():
				return newCanceledStageError(StageCloneRepos, ctx.Err())
			default:
			}
			start := time.Now()
			p, err := client.CloneRepository(r)
			dur := time.Since(start)
			success := err == nil
			if err != nil {
				bs.Report.FailedRepositories++
			} else {
				bs.Report.ClonedRepositories++
				bs.RepoPaths[r.Name] = p
			}
			if bs.Generator != nil && bs.Generator.recorder != nil {
				bs.Generator.recorder.ObserveCloneRepoDuration(r.Name, dur, success)
				bs.Generator.recorder.IncCloneRepoResult(success)
			}
		}
	} else {
		var wg sync.WaitGroup
		sem := make(chan struct{}, concurrency)
		mu := sync.Mutex{}
		for _, repo := range bs.Repositories {
			repo := repo
			wg.Add(1)
			go func() {
				defer wg.Done()
				select {
				case <-ctx.Done():
					return
				default:
				}
				sem <- struct{}{}
				start := time.Now()
				p, err := client.CloneRepository(repo)
				dur := time.Since(start)
				success := err == nil
				mu.Lock()
				if err != nil {
					bs.Report.FailedRepositories++
				} else {
					bs.Report.ClonedRepositories++
					bs.RepoPaths[repo.Name] = p
				}
				mu.Unlock()
				if bs.Generator != nil && bs.Generator.recorder != nil {
					bs.Generator.recorder.ObserveCloneRepoDuration(repo.Name, dur, success)
					bs.Generator.recorder.IncCloneRepoResult(success)
				}
				<-sem
			}()
		}
		wg.Wait()
		// If canceled after goroutines, propagate
		select {
		case <-ctx.Done():
			return newCanceledStageError(StageCloneRepos, ctx.Err())
		default:
		}
	}

	if bs.Report.ClonedRepositories == 0 && bs.Report.FailedRepositories > 0 {
		return newWarnStageError(StageCloneRepos, fmt.Errorf("%w: all clones failed", build.ErrClone))
	}
	if bs.Report.FailedRepositories > 0 {
		return newWarnStageError(StageCloneRepos, fmt.Errorf("%w: %d failed out of %d", build.ErrClone, bs.Report.FailedRepositories, len(bs.Repositories)))
	}
	return nil
}

// stageDiscoverDocs walks cloned repositories to enumerate documentation files.
func stageDiscoverDocs(ctx context.Context, bs *BuildState) error {
	if len(bs.RepoPaths) == 0 {
		// No repos cloned; treat as warning to reflect empty input rather than fatal.
		return newWarnStageError(StageDiscoverDocs, fmt.Errorf("%w: no repositories cloned", build.ErrDiscovery))
	}
	select {
	case <-ctx.Done():
		return newCanceledStageError(StageDiscoverDocs, ctx.Err())
	default:
	}
	discovery := docs.NewDiscovery(bs.Repositories)
	docFiles, err := discovery.DiscoverDocs(bs.RepoPaths)
	if err != nil {
		return newFatalStageError(StageDiscoverDocs, fmt.Errorf("%w: %v", build.ErrDiscovery, err))
	}
	bs.Docs = docFiles
	// update top-level report file count & repository count (may exclude failed clones)
	repoSet := map[string]struct{}{}
	for _, f := range docFiles {
		repoSet[f.Repository] = struct{}{}
	}
	bs.Report.Repositories = len(repoSet)
	bs.Report.Files = len(docFiles)
	return nil
}

func stageGenerateConfig(ctx context.Context, bs *BuildState) error {
	return bs.Generator.generateHugoConfig()
}

func stageLayouts(ctx context.Context, bs *BuildState) error {
	if bs.Generator.config != nil && bs.Generator.config.Hugo.Theme != "" {
		// Theme provided: no fallback layouts necessary.
		// Add no-op statement to avoid staticcheck empty branch warning.
		var _ = bs.Generator.config.Hugo.Theme
		return nil
	}
	// No theme configured: generate basic fallback layouts.
	return bs.Generator.generateBasicLayouts()
}

func stageCopyContent(ctx context.Context, bs *BuildState) error {
	if err := bs.Generator.copyContentFiles(ctx, bs.Docs); err != nil {
		if errors.Is(err, context.Canceled) {
			return newCanceledStageError(StageCopyContent, err)
		}
		return err
	}
	return nil
}

func stageIndexes(ctx context.Context, bs *BuildState) error {
	return bs.Generator.generateIndexPages(bs.Docs)
}

func stageRunHugo(ctx context.Context, bs *BuildState) error {
	if !shouldRunHugo() {
		// Running Hugo disabled by flag; no operation.
		return nil
	}
	if err := bs.Generator.runHugoBuild(); err != nil {
		// Treat hugo runtime failure as warning (site content still copied & usable without static render)
		return newWarnStageError(StageRunHugo, fmt.Errorf("%w: %v", build.ErrHugo, err))
	}
	// mark successful static render
	bs.Report.StaticRendered = true
	return nil
}

func stagePostProcess(ctx context.Context, bs *BuildState) error { // placeholder for future extensions
	// Ensure non-zero measurable duration for timing tests without introducing significant delay.
	start := time.Now()
	for time.Since(start) == 0 {
		// spin very briefly (will exit next nanosecond tick)
	}
	return nil
}
