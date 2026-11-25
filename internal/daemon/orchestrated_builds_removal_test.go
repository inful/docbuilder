package daemon

import (
	"testing"

	"github.com/stretchr/testify/require"

	"git.home.luguber.info/inful/docbuilder/internal/build/queue"
	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/daemon/events"
)

// The pages of a removed repository are still published until a build
// takes them away. The build that follows the removal has to run even
// when no repository is left.
func TestTheBuildAfterARemovalRunsWithoutARepository(t *testing.T) {
	d := daemonForTest(t, daemonForTestOpts{
		Config: &config.Config{Daemon: &config.DaemonConfig{}},
	})
	d.lifecycle.status.Store(StatusRunning)
	d.build.buildQueue = queue.NewBuildQueue(4, 1, &stubBuilder{})

	// A repository was removed.
	d.build.afterRemoval.Store(true)

	d.enqueueOrchestratedBuild(t.Context(), events.BuildNow{JobID: "job-1", LastReason: "discovery"})

	require.Equal(t, 1, d.build.buildQueue.Length(), "the build that takes the pages away was not asked for")
}

// Without a removal the build is skipped: there is nothing to build.
func TestTheBuildWithoutARepositoryIsSkipped(t *testing.T) {
	d := daemonForTest(t, daemonForTestOpts{
		Config: &config.Config{Daemon: &config.DaemonConfig{}},
	})
	d.lifecycle.status.Store(StatusRunning)
	d.build.buildQueue = queue.NewBuildQueue(4, 1, &stubBuilder{})

	d.enqueueOrchestratedBuild(t.Context(), events.BuildNow{JobID: "job-1", LastReason: "discovery"})

	require.Equal(t, 0, d.build.buildQueue.Length(), "a build without a repository was asked for")
}
