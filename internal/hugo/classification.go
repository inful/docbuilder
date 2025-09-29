package hugo

// (Phase 1 extraction) Stage error & classification logic split from stages.go to reduce file size.
// Keeping within the same package (no subpackage yet) for an incremental, non-breaking refactor.

import (
	"errors"
	"fmt"

	"git.home.luguber.info/inful/docbuilder/internal/build"
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
	if errors.As(err, &se) {
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
			Stage: stage, Error: se, Result: resultFromStageErrorKind(se.Kind), IssueCode: code,
			Severity: severityFromStageErrorKind(se.Kind), Transient: se.Transient(), Abort: se.Kind == StageErrorFatal || se.Kind == StageErrorCanceled,
		}
	}
	se = newFatalStageError(stage, err)
	return StageOutcome{Stage: stage, Error: se, Result: StageResultFatal, IssueCode: IssueGenericStageError, Severity: SeverityError, Transient: false, Abort: true}
}

// Helper constructors
func newFatalStageError(stage StageName, err error) *StageError {
	return &StageError{Kind: StageErrorFatal, Stage: stage, Err: err}
}
func newWarnStageError(stage StageName, err error) *StageError {
	return &StageError{Kind: StageErrorWarning, Stage: stage, Err: err}
}
func newCanceledStageError(stage StageName, err error) *StageError {
	return &StageError{Kind: StageErrorCanceled, Stage: stage, Err: err}
}
