package hugo

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// runStages executes stages in order, recording timing and stopping on first fatal error.
// Extracted from stages.go (Phase 1 refactor) with no semantic changes.
func runStages(ctx context.Context, bs *BuildState, stages []StageDef) error {
	for _, st := range stages {
		select {
		case <-ctx.Done():
			se := newCanceledStageError(st.Name, ctx.Err())
			out := StageOutcome{Stage: st.Name, Error: se, Result: StageResultCanceled, IssueCode: IssueCanceled, Severity: SeverityError, Transient: false, Abort: true}
			bs.Report.StageErrorKinds[st.Name] = se.Kind
			bs.Report.AddIssue(out.IssueCode, out.Stage, out.Severity, se.Error(), out.Transient, se)
			bs.Report.recordStageResult(out.Stage, out.Result, bs.Generator.recorder)
			if bs.Generator != nil && bs.Generator.observer != nil { bs.Generator.observer.OnStageComplete(st.Name, 0, StageResultCanceled) }
			return se
		default:
		}
		if bs.Generator != nil && bs.Generator.observer != nil { bs.Generator.observer.OnStageStart(st.Name) }
		t0 := time.Now()
		err := st.Fn(ctx, bs)
		dur := time.Since(t0)
		bs.Report.StageDurations[string(st.Name)] = dur
		out := classifyStageResult(st.Name, err, bs)
		if out.Error != nil { // error path
			bs.Report.StageErrorKinds[st.Name] = out.Error.Kind
			bs.Report.AddIssue(out.IssueCode, out.Stage, out.Severity, out.Error.Error(), out.Transient, out.Error)
		}
		bs.Report.recordStageResult(st.Name, out.Result, bs.Generator.recorder)
		if bs.Generator != nil && bs.Generator.observer != nil { bs.Generator.observer.OnStageComplete(st.Name, dur, out.Result) }
		if out.Abort {
			if out.Error != nil {
				return out.Error
			}
			return fmt.Errorf("stage %s aborted", st.Name)
		}
		if st.Name == StageCloneRepos && bs.AllReposUnchanged { // early skip optimization
			if bs.Generator != nil && bs.Generator.existingSiteValidForSkip() {
				slog.Info("Early build exit: no repository HEAD changes and existing site valid; skipping remaining stages")
				bs.Report.SkipReason = "no_changes"
				bs.Report.deriveOutcome()
				bs.Report.finish()
				return nil
			}
			slog.Info("Repository heads unchanged but output invalid/missing; proceeding with full build")
		}
	}
	if bs.Generator != nil && bs.Generator.observer != nil { bs.Generator.observer.OnBuildComplete(bs.Report) }
	return nil
}
