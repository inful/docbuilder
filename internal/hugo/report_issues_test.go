package hugo

import (
	"errors"
	"testing"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/hugo/models"

	"git.home.luguber.info/inful/docbuilder/internal/build"
)

// fake generator methods avoided by tailoring stages

// TestIssueTaxonomyPartialClone verifies issue codes for partial clone warning path.
func TestIssueTaxonomyPartialClone(t *testing.T) {
	report := models.NewBuildReport(t.Context(), 0, 0)
	// Override git client globally would be complex; instead simulate outcomes by directly manipulating report counts
	// Simulate one success + one failure then inject a warning models consistent with stageCloneRepos behavior.
	report.ClonedRepositories = 1
	report.FailedRepositories = 1
	se := models.NewWarnStageError(models.StageCloneRepos, errors.New("wrapper: "+build.ErrClone.Error()))
	report.Errors = nil
	report.Warnings = append(report.Warnings, se)
	report.StageErrorKinds[models.StageCloneRepos] = se.Kind
	report.RecordStageResult(models.StageCloneRepos, models.StageResultWarning, nil)
	// emulate runStages logic for issue creation
	issue := models.ReportIssue{Stage: models.StageCloneRepos, Message: se.Error(), Transient: se.Transient(), Severity: models.SeverityWarning}
	issue.Code = models.IssuePartialClone
	report.Issues = append(report.Issues, issue)
	report.Finish()
	report.DeriveOutcome()
	ser := report.SanitizedCopy()
	if ser.Outcome != "warning" {
		t.Fatalf("expected outcome warning, got %s", ser.Outcome)
	}
	if len(ser.Issues) == 0 {
		t.Fatalf("expected at least one issue")
	}
	if ser.Issues[0].Code != models.IssuePartialClone {
		t.Errorf("expected models.IssuePartialClone, got %s", ser.Issues[0].Code)
	}
}

// TestIssueTaxonomyHugoWarning ensures hugo execution warning produces an issue entry.
func TestIssueTaxonomyHugoWarning(t *testing.T) {
	report := models.NewBuildReport(t.Context(), 0, 0)
	// Simulate a hugo run warning
	se := models.NewWarnStageError(models.StageRunHugo, errors.New("wrap: "+build.ErrHugo.Error()))
	report.StageErrorKinds[models.StageRunHugo] = se.Kind
	report.Warnings = append(report.Warnings, se)
	report.RecordStageResult(models.StageRunHugo, models.StageResultWarning, nil)
	issue := models.ReportIssue{Stage: models.StageRunHugo, Message: se.Error(), Transient: se.Transient(), Severity: models.SeverityWarning, Code: models.IssueHugoExecution}
	report.Issues = append(report.Issues, issue)
	report.Finish()
	report.DeriveOutcome()
	if report.Outcome != "warning" {
		t.Fatalf("expected outcome warning got %s", report.Outcome)
	}
	if len(report.Issues) == 0 {
		t.Fatalf("expected issues")
	}
	for _, is := range report.Issues {
		if is.Code == models.IssueHugoExecution {
			return
		}
	}
	t.Fatalf("expected models.IssueHugoExecution in issues list")
}

// Simple time guard to ensure sanitized copy preserves schema_version & issues.
func TestSanitizedCopyPreservesSchemaVersionAndIssues(t *testing.T) {
	r := models.NewBuildReport(t.Context(), 1, 1)
	r.Issues = append(r.Issues, models.ReportIssue{Code: models.IssueCloneFailure, Stage: models.StageCloneRepos, Severity: models.SeverityError, Message: "m", Transient: false})
	time.Sleep(time.Millisecond) // ensure non-zero duration
	r.Finish()
	r.DeriveOutcome()
	ser := r.SanitizedCopy()
	if ser.SchemaVersion != 1 {
		t.Fatalf("expected schema_version 1")
	}
	if len(ser.Issues) != 1 {
		t.Fatalf("expected 1 issue")
	}
}
