package daemon

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"git.home.luguber.info/inful/docbuilder/internal/build"
	"git.home.luguber.info/inful/docbuilder/internal/build/queue"
	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/git"
)

// errWorkspace is the error of a build that fails before it writes a
// report: the workspace of the build cannot be made.
var errWorkspace = errors.New("failed to create workspace")

// reportlessBuildService is a build that fails before it writes a
// report: the workspace of the build cannot be made.
type reportlessBuildService struct{}

func (reportlessBuildService) Run(context.Context, build.BuildRequest) (*build.BuildResult, error) {
	return &build.BuildResult{Status: build.BuildStatusFailed}, errWorkspace
}

func (reportlessBuildService) RunDirect(context.Context, build.DirectBuildRequest) (*build.BuildResult, error) {
	return &build.BuildResult{Status: build.BuildStatusFailed}, errWorkspace
}

// A build that failed before it wrote a report did not bring the site
// up to date: the daemon still said that the old commit was current,
// and the next webhook skipped the build. The state of the daemon is
// updated even when there is no report.
func TestTheFailedBuildUpdatesTheStateOfTheDaemon(t *testing.T) {
	cache, err := git.NewRemoteHeadCache("")
	require.NoError(t, err)
	cache.Set("https://example.invalid/repo.git", "main", "abc123")

	updater := NewRepoUpdater(nil, nil, cache, nil)

	d := daemonForTest(t, daemonForTestOpts{
		Config:       &config.Config{Daemon: &config.DaemonConfig{}},
		BuildService: reportlessBuildService{},
		RepoUpdater:  updater,
		StateManager: newFakeDaemonStateManager(),
	})

	job := &queue.BuildJob{ID: "job-1"}
	queue.EnsureTypedMeta(job).V2Config = &config.Config{Daemon: &config.DaemonConfig{}}

	_, err = d.Build(t.Context(), job)
	require.Error(t, err, "the build failed")

	require.Nil(t, cache.Get("https://example.invalid/repo.git", "main"),
		"the state of the daemon still says that the old commit is current")
}
