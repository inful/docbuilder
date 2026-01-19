package stages

import (
	"errors"

	"git.home.luguber.info/inful/docbuilder/internal/hugo/models"
)

// StageOutcome normalized result of stage execution.
type StageOutcome struct {
	Stage     models.StageName
	Error     *models.StageError
	Result    models.StageResult
	IssueCode models.ReportIssueCode
	Severity  models.IssueSeverity
	Transient bool
	Abort     bool
}

// resultFromStageErrorKind maps a StageErrorKind to a StageResult.
func resultFromStageErrorKind(k models.StageErrorKind) models.StageResult {
	switch k {
	case models.StageErrorWarning:
		return models.StageResultWarning
	case models.StageErrorCanceled:
		return models.StageResultCanceled
	case models.StageErrorFatal:
		return models.StageResultFatal
	default:
		return models.StageResultFatal
	}
}

// severityFromStageErrorKind maps StageErrorKind to IssueSeverity.
func severityFromStageErrorKind(k models.StageErrorKind) models.IssueSeverity {
	if k == models.StageErrorWarning {
		return models.SeverityWarning
	}
	return models.SeverityError
}

// ClassifyStageResult converts a raw error from a stage into a StageOutcome.
func ClassifyStageResult(stage models.StageName, err error, bs *models.BuildState) StageOutcome {
	if err == nil {
		return StageOutcome{Stage: stage, Result: models.StageResultSuccess}
	}

	var se *models.StageError
	if !errors.As(err, &se) {
		// Not a StageError - treat as fatal
		se = models.NewFatalStageError(stage, err)
		return buildFatalOutcome(stage, se)
	}

	// Check for cancellation first - applies to all stages
	if se.Kind == models.StageErrorCanceled {
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
		Abort:     se.Kind == models.StageErrorFatal || se.Kind == models.StageErrorCanceled,
	}
}

// classifyIssueCode determines the issue code based on stage type and error.
func classifyIssueCode(se *models.StageError, bs *models.BuildState) models.ReportIssueCode {
	switch se.Stage {
	case models.StageCloneRepos:
		return classifyCloneIssue(se, bs)
	case models.StageDiscoverDocs:
		return classifyDiscoveryIssue(se, bs)
	case models.StageRunHugo:
		return classifyHugoIssue(se)
	case models.StagePrepareOutput, models.StageGenerateConfig, models.StageLayouts, models.StageCopyContent, models.StageIndexes, models.StagePostProcess:
		// These stages use generic issue codes
		return models.IssueGenericStageError
	default:
		return models.IssueGenericStageError
	}
}

// classifyCloneIssue classifies clone stage errors.
func classifyCloneIssue(se *models.StageError, bs *models.BuildState) models.ReportIssueCode {
	if !errors.Is(se.Err, models.ErrClone) {
		return models.IssueCloneFailure
	}

	if bs.Report.ClonedRepositories == 0 {
		return models.IssueAllClonesFailed
	}

	if bs.Report.FailedRepositories > 0 {
		return models.IssuePartialClone
	}

	return models.IssueCloneFailure
}

// classifyDiscoveryIssue classifies discovery stage errors.
func classifyDiscoveryIssue(se *models.StageError, bs *models.BuildState) models.ReportIssueCode {
	if !errors.Is(se.Err, models.ErrDiscovery) {
		return models.IssueDiscoveryFailure
	}

	if len(bs.Git.RepoPaths) == 0 {
		return models.IssueNoRepositories
	}

	return models.IssueDiscoveryFailure
}

// classifyHugoIssue classifies Hugo stage errors.
func classifyHugoIssue(se *models.StageError) models.ReportIssueCode {
	return models.IssueHugoExecution
}

// buildFatalOutcome creates an outcome for fatal errors.
func buildFatalOutcome(stage models.StageName, se *models.StageError) StageOutcome {
	return StageOutcome{
		Stage:     stage,
		Error:     se,
		Result:    models.StageResultFatal,
		IssueCode: models.IssueGenericStageError,
		Severity:  models.SeverityError,
		Transient: false,
		Abort:     true,
	}
}

// buildCanceledOutcome creates an outcome for canceled stages.
func buildCanceledOutcome(stage models.StageName, se *models.StageError) StageOutcome {
	return StageOutcome{
		Stage:     stage,
		Error:     se,
		Result:    resultFromStageErrorKind(se.Kind),
		IssueCode: models.IssueCanceled,
		Severity:  severityFromStageErrorKind(se.Kind),
		Transient: se.Transient(),
		Abort:     true,
	}
}
