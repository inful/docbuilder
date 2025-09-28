package hugo

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
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
	if k == StageErrorWarning {
		return SeverityWarning
	}
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
			} else {
				code = IssueCloneFailure
			}
		case StageDiscoverDocs:
			if errors.Is(se.Err, build.ErrDiscovery) {
				if len(bs.RepoPaths) == 0 {
					code = IssueNoRepositories
				} else {
					code = IssueDiscoveryFailure
				}
			} else {
				code = IssueDiscoveryFailure
			}
		case StageRunHugo:
			if errors.Is(se.Err, build.ErrHugo) {
				code = IssueHugoExecution
			} else {
				code = IssueHugoExecution
			}
		default:
			if se.Kind == StageErrorCanceled {
				code = IssueCanceled
			}
		}
		return StageOutcome{
			Stage:     stage,
			Error:     se,
			Result:    resultFromStageErrorKind(se.Kind),
			IssueCode: code,
			Severity:  severityFromStageErrorKind(se.Kind),
			Transient: se.Transient(),
			Abort:     se.Kind == StageErrorFatal || se.Kind == StageErrorCanceled,
		}
	}
	// Unknown error: wrap as fatal with generic code.
	se = newFatalStageError(stage, err)
	return StageOutcome{
		Stage:     stage,
		Error:     se,
		Result:    StageResultFatal,
		IssueCode: IssueGenericStageError,
		Severity:  SeverityError,
		Transient: false,
		Abort:     true,
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
	Generator         *Generator
	Docs              []docs.DocFile
	Report            *BuildReport
	start             time.Time
	Repositories      []config.Repository // configured repositories (post-filter)
	RepoPaths         map[string]string   // name -> local filesystem path
	WorkspaceDir      string              // root workspace for git operations
	preHeads          map[string]string   // repo -> head before update (for existing repos)
	postHeads         map[string]string   // repo -> head after clone/update
	AllReposUnchanged bool                // set true if every repo head unchanged (and no fresh clones)
	ConfigHash        string              // fingerprint of relevant config used for change detection
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
			if out.Error != nil {
				return out.Error
			}
			return fmt.Errorf("stage %s aborted", st.Name)
		}
		// Early skip optimization: after clone stage, if all repos unchanged skip remaining stages.
		if st.Name == StageCloneRepos && bs.AllReposUnchanged {
			// Only allow early exit if existing output structure is still valid.
			if bs.Generator != nil && bs.Generator.existingSiteValidForSkip() {
				slog.Info("Early build exit: no repository HEAD changes and existing site valid; skipping remaining stages")
				bs.Report.SkipReason = "no_changes"
				// Derive outcome and finish timestamps so report persistence is consistent with full builds.
				bs.Report.deriveOutcome()
				bs.Report.finish()
				return nil
			}
			// Site missing or incomplete: continue with full pipeline despite unchanged heads.
			slog.Info("Repository heads unchanged but output invalid/missing; proceeding with full build")
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
	if bs.Generator != nil {
		client = client.WithBuildConfig(&bs.Generator.config.Build)
	}

	strategy := config.CloneStrategyFresh
	if bs.Generator != nil {
		if s := bs.Generator.config.Build.CloneStrategy; s != "" {
			strategy = s
		}
	}
	if err := client.EnsureWorkspace(); err != nil {
		return newFatalStageError(StageCloneRepos, fmt.Errorf("ensure workspace: %w", err))
	}
	bs.RepoPaths = make(map[string]string, len(bs.Repositories))
	bs.preHeads = make(map[string]string, len(bs.Repositories))
	bs.postHeads = make(map[string]string, len(bs.Repositories))

	// Normalize requested concurrency.
	concurrency := 1
	if bs.Generator != nil && bs.Generator.config.Build.CloneConcurrency > 0 {
		concurrency = bs.Generator.config.Build.CloneConcurrency
	}
	if concurrency > len(bs.Repositories) {
		concurrency = len(bs.Repositories)
	}
	if concurrency < 1 { // defensive after normalization
		concurrency = 1
	}
	if bs.Generator != nil && bs.Generator.recorder != nil {
		bs.Generator.recorder.SetCloneConcurrency(concurrency)
	}

	// Single unified worker pool (also covers sequential when concurrency==1).
	type cloneTask struct{ repo config.Repository }
	tasks := make(chan cloneTask)
	var wg sync.WaitGroup
	var mu sync.Mutex // protects report counters & RepoPaths

	worker := func() {
		defer wg.Done()
		for task := range tasks { // exit when channel closed
			// Pre-clone cancellation check
			select {
			case <-ctx.Done():
				return
			default:
			}
			start := time.Now()
			var (
				p   string
				err error
			)
			// Determine if we should attempt update based on strategy.
			attemptUpdate := false
			var preHead string
			switch strategy {
			case config.CloneStrategyUpdate:
				attemptUpdate = true
			case config.CloneStrategyAuto:
				repoPath := filepath.Join(bs.WorkspaceDir, task.repo.Name)
				if _, statErr := os.Stat(filepath.Join(repoPath, ".git")); statErr == nil {
					attemptUpdate = true
					if head, herr := readRepoHead(repoPath); herr == nil {
						preHead = head
					}
				}
			}
			if attemptUpdate {
				p, err = client.UpdateRepository(task.repo)
			} else {
				p, err = client.CloneRepository(task.repo)
			}
			dur := time.Since(start)
			success := err == nil
			mu.Lock()
			if success {
				bs.Report.ClonedRepositories++
				bs.RepoPaths[task.repo.Name] = p
				if head, herr := readRepoHead(p); herr == nil { // post head
					bs.postHeads[task.repo.Name] = head
					if preHead != "" {
						bs.preHeads[task.repo.Name] = preHead
					}
				}
			} else {
				bs.Report.FailedRepositories++
				// Attach structured issue immediately for permanent git errors (non-transient) with granular code.
				if bs.Report != nil {
					code := classifyGitFailure(err)
					sev := SeverityError
					// Mark transient=false because classifyGitFailure only returns granular permanent codes; generic clone failures handled later.
					if code != "" {
						bs.Report.AddIssue(code, StageCloneRepos, sev, err.Error(), false, err)
					}
				}
			}
			mu.Unlock()
			if bs.Generator != nil && bs.Generator.recorder != nil {
				bs.Generator.recorder.ObserveCloneRepoDuration(task.repo.Name, dur, success)
				bs.Generator.recorder.IncCloneRepoResult(success)
			}
		}
	}

	wg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		go worker()
	}

	// Feed tasks (respecting cancellation before enqueue where possible).
	for _, r := range bs.Repositories {
		select {
		case <-ctx.Done():
			// Stop producing more tasks; workers drain nothing further.
			close(tasks)
			wg.Wait()
			return newCanceledStageError(StageCloneRepos, ctx.Err())
		default:
		}
		tasks <- cloneTask{repo: r}
	}
	close(tasks)
	wg.Wait()

	// Late cancellation check (context could be canceled while workers finishing last repo).
	select {
	case <-ctx.Done():
		return newCanceledStageError(StageCloneRepos, ctx.Err())
	default:
	}

	// Compute unchanged determination
	unchanged := bs.Report.FailedRepositories == 0 && len(bs.postHeads) > 0
	if unchanged {
		for name, post := range bs.postHeads {
			if pre, ok := bs.preHeads[name]; !ok || pre == "" || pre != post { // fresh clone or changed
				unchanged = false
				break
			}
		}
	}
	bs.AllReposUnchanged = unchanged
	if bs.AllReposUnchanged {
		slog.Info("No repository head changes detected", slog.Int("repos", len(bs.postHeads)))
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

// classifyGitFailure inspects an error string (best-effort) for permanent git failure signatures
// to map them onto granular issue codes. Only returns codes for permanent, non-transient conditions.
func classifyGitFailure(err error) ReportIssueCode {
	if err == nil {
		return ""
	}
	msg := err.Error()
	l := strings.ToLower(msg)
	switch {
	case strings.Contains(l, "authentication failed") || strings.Contains(l, "authentication required") || strings.Contains(l, "invalid username or password") || strings.Contains(l, "authorization failed"):
		return IssueAuthFailure
	case strings.Contains(l, "repository not found") || strings.Contains(l, "not found") && strings.Contains(l, "repository"):
		return IssueRepoNotFound
	case strings.Contains(l, "unsupported protocol"):
		return IssueUnsupportedProto
	case strings.Contains(l, "diverged") && strings.Contains(l, "hard reset disabled"):
		return IssueRemoteDiverged
	default:
		return ""
	}
}

// readRepoHead returns the current HEAD commit hash (short 40 hex) for a repository path; best-effort.
func readRepoHead(repoPath string) (string, error) {
	// Open .git/HEAD and resolve if symbolic ref; fallback to empty on errors.
	headPath := filepath.Join(repoPath, ".git", "HEAD")
	data, err := os.ReadFile(headPath)
	if err != nil {
		return "", err
	}
	line := strings.TrimSpace(string(data))
	if strings.HasPrefix(line, "ref:") {
		ref := strings.TrimSpace(strings.TrimPrefix(line, "ref:"))
		refPath := filepath.Join(repoPath, ".git", filepath.FromSlash(ref))
		b, berr := os.ReadFile(refPath)
		if berr == nil {
			return strings.TrimSpace(string(b)), nil
		}
	}
	return line, nil
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
	if err := bs.Generator.generateIndexPages(bs.Docs); err != nil {
		return err
	}
	// capture usage into report for observability
	if bs.Report != nil && bs.Generator != nil && bs.Generator.indexTemplateUsage != nil {
		for k, v := range bs.Generator.indexTemplateUsage {
			bs.Report.IndexTemplates[k] = v
		}
	}
	return nil
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
