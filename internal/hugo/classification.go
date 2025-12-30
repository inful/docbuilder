package hugo

// (Phase 1 extraction) Stage error & classification logic split from stages.go to reduce file size.
// Keeping within the same package (no subpackage yet) for an incremental, non-breaking refactor.

import (
	"errors"
	"fmt"

	"git.home.luguber.info/inful/docbuilder/internal/build"
	gitpkg "git.home.luguber.info/inful/docbuilder/internal/git"
)

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

// Transient reports whether the underlying error condition is likely transient.
// Heuristics kept identical to pre-refactor version for behavioral stability.
func (e *StageError) Transient() bool {
	if e == nil {
		return false
	}
	if e.Kind == StageErrorCanceled {
		return false
	}
	cause := e.Err
	isSentinel := func(target error) bool { return errors.Is(cause, target) }
	switch e.Stage {
	case StageCloneRepos:
		if isSentinel(build.ErrClone) {
			return true
		}
		// Typed transient git errors
		if errors.As(cause, new(*gitpkg.RateLimitError)) || errors.As(cause, new(*gitpkg.NetworkTimeoutError)) {
			return true
		}
	case StageRunHugo:
		if isSentinel(build.ErrHugo) {
			return true
		}
	case StageDiscoverDocs:
		if isSentinel(build.ErrDiscovery) {
			return e.Kind == StageErrorWarning
		}
	}
	return false
}

// StageOutcome normalized result of stage execution.
type StageOutcome struct {
	Stage     StageName
	Error     *StageError
	Result    StageResult
	IssueCode ReportIssueCode
	Severity  IssueSeverity
	Transient bool
	Abort     bool
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

// classifyStageResult converts a raw error from a stage into a StageOutcome.
func classifyStageResult(stage StageName, err error, bs *BuildState) StageOutcome {
	if err == nil {
		return StageOutcome{Stage: stage, Result: StageResultSuccess}
	}

	var se *StageError
	if !errors.As(err, &se) {
		// Not a StageError - treat as fatal
		se = newFatalStageError(stage, err)
		return buildFatalOutcome(stage, se)
	}

	// Check for cancellation first - applies to all stages
	if se.Kind == StageErrorCanceled {
		return buildCanceledOutcome(stage, se)
	}

	// Classify by stage type
	code := classifyIssueCode(se, bs)

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

// classifyIssueCode determines the issue code based on stage type and error.
func classifyIssueCode(se *StageError, bs *BuildState) ReportIssueCode {
	switch se.Stage {
	case StageCloneRepos:
		return classifyCloneIssue(se, bs)
	case StageDiscoverDocs:
		return classifyDiscoveryIssue(se, bs)
	case StageRunHugo:
		return classifyHugoIssue(se)
	default:
		return IssueGenericStageError
	}
}

// classifyCloneIssue classifies clone stage errors.
func classifyCloneIssue(se *StageError, bs *BuildState) ReportIssueCode {
	if !errors.Is(se.Err, build.ErrClone) {
		return IssueCloneFailure
	}

	if bs.Report.ClonedRepositories == 0 {
		return IssueAllClonesFailed
	}

	if bs.Report.FailedRepositories > 0 {
		return IssuePartialClone
	}

	return IssueCloneFailure
}

// classifyDiscoveryIssue classifies discovery stage errors.
func classifyDiscoveryIssue(se *StageError, bs *BuildState) ReportIssueCode {
	if !errors.Is(se.Err, build.ErrDiscovery) {
		return IssueDiscoveryFailure
	}

	if len(bs.Git.RepoPaths) == 0 {
		return IssueNoRepositories
	}

	return IssueDiscoveryFailure
}

// classifyHugoIssue classifies Hugo stage errors.
func classifyHugoIssue(se *StageError) ReportIssueCode {
	return IssueHugoExecution
}

// buildFatalOutcome creates an outcome for fatal errors.
func buildFatalOutcome(stage StageName, se *StageError) StageOutcome {
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

// buildCanceledOutcome creates an outcome for canceled stages.
func buildCanceledOutcome(stage StageName, se *StageError) StageOutcome {
	return StageOutcome{
		Stage:     stage,
		Error:     se,
		Result:    resultFromStageErrorKind(se.Kind),
		IssueCode: IssueCanceled,
		Severity:  severityFromStageErrorKind(se.Kind),
		Transient: se.Transient(),
		Abort:     true,
	}
}

// Helper constructors.
func newFatalStageError(stage StageName, err error) *StageError {
	return &StageError{Kind: StageErrorFatal, Stage: stage, Err: err}
}

func newWarnStageError(stage StageName, err error) *StageError {
	return &StageError{Kind: StageErrorWarning, Stage: stage, Err: err}
}

func newCanceledStageError(stage StageName, err error) *StageError {
	return &StageError{Kind: StageErrorCanceled, Stage: stage, Err: err}
}
