package daemon

import (
	"testing"

	"github.com/stretchr/testify/require"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/git"
	"git.home.luguber.info/inful/docbuilder/internal/hugo/models"
)

// A build that failed did not bring the site up to date. The
// remote-head cache holds the head of every remote as it was seen: it
// would say that the remote did not move, and the next webhook would
// skip the build.
func TestAFailedBuildForgetsTheHeadsOfTheRemotes(t *testing.T) {
	cache, err := git.NewRemoteHeadCache(t.TempDir())
	require.NoError(t, err)
	cache.Set("https://example.com/repo.git", "main", "abc123")

	d := daemonForTest(t, daemonForTestOpts{
		Config:       &config.Config{Daemon: &config.DaemonConfig{}},
		StateManager: &fakeDaemonStateManager{},
	})
	d.build.repoUpdater = &RepoUpdater{cache: cache}

	d.updateStateAfterBuild(&models.BuildReport{Outcome: models.OutcomeFailed})

	require.Nil(t, cache.Get("https://example.com/repo.git", "main"),
		"the cache still holds the head of a remote whose build failed")
}
