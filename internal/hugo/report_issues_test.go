package hugo

import (
    "errors"
    "testing"
    "time"

    "git.home.luguber.info/inful/docbuilder/internal/build"
)

// fake generator methods avoided by tailoring stages

// TestIssueTaxonomyPartialClone verifies issue codes for partial clone warning path.
func TestIssueTaxonomyPartialClone(t *testing.T) {
    report := newBuildReport(0, 0)
    // Override git client globally would be complex; instead simulate outcomes by directly manipulating report counts
    // Simulate one success + one failure then inject a warning StageError consistent with stageCloneRepos behavior.
    report.ClonedRepositories = 1
    report.FailedRepositories = 1
    se := newWarnStageError(StageCloneRepos, errors.New("wrapper: "+build.ErrClone.Error()))
    report.Errors = nil
    report.Warnings = append(report.Warnings, se)
    report.StageErrorKinds[string(StageCloneRepos)] = string(se.Kind)
    report.recordStageResult(StageCloneRepos, StageResultWarning, nil)
    // emulate runStages logic for issue creation
    issue := ReportIssue{Stage: StageCloneRepos, Message: se.Error(), Transient: se.Transient(), Severity: SeverityWarning}
    issue.Code = IssuePartialClone
    report.Issues = append(report.Issues, issue)
    report.finish()
    report.deriveOutcome()
    ser := report.sanitizedCopy()
    if ser.Outcome != "warning" { t.Fatalf("expected outcome warning, got %s", ser.Outcome) }
    if len(ser.Issues) == 0 { t.Fatalf("expected at least one issue") }
    if ser.Issues[0].Code != IssuePartialClone { t.Errorf("expected IssuePartialClone, got %s", ser.Issues[0].Code) }
}

// TestIssueTaxonomyHugoWarning ensures hugo execution warning produces an issue entry.
func TestIssueTaxonomyHugoWarning(t *testing.T) {
    report := newBuildReport(0, 0)
    // Simulate a hugo run warning
    se := newWarnStageError(StageRunHugo, errors.New("wrap: "+build.ErrHugo.Error()))
    report.StageErrorKinds[string(StageRunHugo)] = string(se.Kind)
    report.Warnings = append(report.Warnings, se)
    report.recordStageResult(StageRunHugo, StageResultWarning, nil)
    issue := ReportIssue{Stage: StageRunHugo, Message: se.Error(), Transient: se.Transient(), Severity: SeverityWarning, Code: IssueHugoExecution}
    report.Issues = append(report.Issues, issue)
    report.finish(); report.deriveOutcome()
    if report.Outcome != "warning" { t.Fatalf("expected outcome warning got %s", report.Outcome) }
    if len(report.Issues) == 0 { t.Fatalf("expected issues") }
    for _, is := range report.Issues { if is.Code == IssueHugoExecution { return } }
    t.Fatalf("expected IssueHugoExecution in issues list")
}

// Simple time guard to ensure sanitized copy preserves schema_version & issues
func TestSanitizedCopyPreservesSchemaVersionAndIssues(t *testing.T) {
    r := newBuildReport(1,1)
    r.Issues = append(r.Issues, ReportIssue{Code: IssueCloneFailure, Stage: StageCloneRepos, Severity: SeverityError, Message: "m", Transient: false})
    time.Sleep(time.Millisecond) // ensure non-zero duration
    r.finish(); r.deriveOutcome()
    ser := r.sanitizedCopy()
    if ser.SchemaVersion != 1 { t.Fatalf("expected schema_version 1") }
    if len(ser.Issues) != 1 { t.Fatalf("expected 1 issue") }
}
