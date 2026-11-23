package stages

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/hugo/models"
)

// RunStages executes stages in order, recording timing and stopping on first fatal error.
func RunStages(ctx context.Context, bs *models.BuildState, stages []models.StageDef) error {
	for _, st := range stages {
		select {
		case <-ctx.Done():
			se := models.NewCanceledStageError(st.Name, ctx.Err())
			out := StageOutcome{Stage: st.Name, Error: se, Result: models.StageResultCanceled, IssueCode: models.IssueCanceled, Severity: models.SeverityError, Transient: false, Abort: true}
			bs.Report.StageErrorKinds[st.Name] = se.Kind
			bs.Report.AddIssue(out.IssueCode, out.Stage, out.Severity, se.Error(), out.Transient, se)
			bs.Report.RecordStageResult(out.Stage, out.Result, bs.Generator.Recorder())
			if bs.Generator != nil && bs.Generator.Observer() != nil {
				bs.Generator.Observer().OnStageComplete(st.Name, 0, models.StageResultCanceled)
			}
			return se
		default:
		}

		if bs.Generator != nil && bs.Generator.Observer() != nil {
			bs.Generator.Observer().OnStageStart(st.Name)
		}

		t0 := time.Now()
		err := st.Fn(ctx, bs)
		dur := time.Since(t0)

		bs.Report.StageDurations[string(st.Name)] = dur

		out := ClassifyStageResult(st.Name, err, bs)

		if out.Error != nil { // error path
			bs.Report.StageErrorKinds[st.Name] = out.Error.Kind
			bs.Report.AddIssue(out.IssueCode, out.Stage, out.Severity, out.Error.Error(), out.Transient, out.Error)
		}

		bs.Report.RecordStageResult(st.Name, out.Result, bs.Generator.Recorder())

		if bs.Generator != nil && bs.Generator.Observer() != nil {
			bs.Generator.Observer().OnStageComplete(st.Name, dur, out.Result)
		}

		if out.Abort {
			if out.Error != nil {
				return out.Error
			}
			return fmt.Errorf("stage %s aborted", st.Name)
		}

		// A repository that could not be cloned is not in the list of the
		// heads, so it does not count as changed. The build has to run:
		// the site is missing the pages of that repository.
		if st.Name == models.StageCloneRepos && bs.Git.AllReposUnchanged && bs.Report.FailedRepositories == 0 {
			if bs.Generator != nil && bs.Generator.SkipIfUnchanged() && bs.Generator.ExistingSiteValidForSkip() && repositoriesMatchTheState(bs) {
				slog.Info("Early build exit: no repository HEAD changes and existing site valid; skipping remaining stages")
				bs.Report.SkipReason = "no_changes"
				bs.Report.DeriveOutcome()
				bs.Report.Finish()
				return nil
			}
			slog.Info("Repository heads unchanged but output invalid/missing; proceeding with full build")
		}
	}

	if bs.Generator != nil && bs.Generator.Observer() != nil {
		bs.Generator.Observer().OnBuildComplete(bs.Report)
	}

	return nil
}

// repositoriesMatchTheState reports whether the repositories of the build
// are the ones of the last build.
//
// A repository that was removed from the configuration is not in the
// list of the heads any more, so the heads alone do not see it: the build
// would be skipped and the pages of the removed repository would stay
// published forever.
func repositoriesMatchTheState(bs *models.BuildState) bool {
	if bs.Generator == nil {
		return true
	}
	reader, ok := bs.Generator.StateManager().(interface{ RepositoryURLs() []string })
	if !ok {
		return true
	}

	known := make(map[string]struct{}, len(bs.Git.Repositories))
	for i := range bs.Git.Repositories {
		known[bs.Git.Repositories[i].URL] = struct{}{}
	}
	for _, url := range reader.RepositoryURLs() {
		if _, found := known[url]; !found {
			return false
		}
	}
	return true
}
