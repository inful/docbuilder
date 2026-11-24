package daemon

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"git.home.luguber.info/inful/docbuilder/internal/build"
	"git.home.luguber.info/inful/docbuilder/internal/build/queue"
	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/hugo/models"
)

// failingBuildService fails every build and fills the report, the way
// the real service does.
type failingBuildService struct {
	report *models.BuildReport
}

func (s *failingBuildService) Run(_ context.Context, _ build.BuildRequest) (*build.BuildResult, error) {
	return &build.BuildResult{Report: s.report}, errBuildFailed
}

func (s *failingBuildService) RunDirect(_ context.Context, _ build.DirectBuildRequest) (*build.BuildResult, error) {
	return &build.BuildResult{Report: s.report}, errBuildFailed
}

// errBuildFailed is the failure of the build service.
var errBuildFailed = buildError("the build failed")

type buildError string

func (e buildError) Error() string { return string(e) }

// The report of a failed build says what failed: the queue tells a
// transient error from a permanent one by it, and it retries the
// transient ones. Thrown away, the retries never happened.
func TestTheReportOfAFailedBuildIsNotThrownAway(t *testing.T) {
	report := &models.BuildReport{
		Outcome: models.OutcomeFailed,
		Errors:  []error{&models.StageError{Kind: models.StageErrorFatal, Stage: models.StageCloneRepos, Err: models.ErrClone}},
	}

	d := daemonForTest(t, daemonForTestOpts{
		Config: &config.Config{Daemon: &config.DaemonConfig{}},
	})
	d.build.buildSvc = &failingBuildService{report: report}

	got, err := d.Build(t.Context(), &queue.BuildJob{
		ID:   "job-1",
		Type: queue.BuildTypeManual,
		TypedMeta: &queue.BuildJobMetadata{
			V2Config:     &config.Config{Daemon: &config.DaemonConfig{}},
			Repositories: nil,
		},
	})
	require.Error(t, err)
	require.NotNil(t, got, "the report of the failed build was thrown away")
	require.Len(t, got.Errors, 1)
}
