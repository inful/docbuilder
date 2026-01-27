package daemon

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/daemon/events"
	"git.home.luguber.info/inful/docbuilder/internal/git"
	"git.home.luguber.info/inful/docbuilder/internal/state"
)

func TestDaemon_handleRepoRemoved_PrunesStateAndCache(t *testing.T) {
	tmp := t.TempDir()
	repoCacheDir := filepath.Join(tmp, "repo-cache")
	require.NoError(t, os.MkdirAll(repoCacheDir, 0o750))

	const (
		repoURL  = "https://example.com/r2.git"
		repoName = "r2"
	)

	// Seed a fake cached clone directory.
	repoDir := filepath.Join(repoCacheDir, repoName)
	require.NoError(t, os.MkdirAll(repoDir, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "README.md"), []byte("hi"), 0o600))

	cache, err := git.NewRemoteHeadCache("")
	require.NoError(t, err)
	cache.Set(repoURL, "main", "deadbeef")
	cache.Set(repoURL, "dev", "beadfeed")
	require.NotNil(t, cache.Get(repoURL, "main"))
	require.NotNil(t, cache.Get(repoURL, "dev"))

	svcResult := state.NewService(tmp)
	require.True(t, svcResult.IsOk())
	sm := state.NewServiceAdapter(svcResult.Unwrap())
	sm.EnsureRepositoryState(repoURL, repoName, "main")
	require.NotNil(t, sm.GetRepository(repoURL))

	d := &Daemon{
		config:       &config.Config{Daemon: &config.DaemonConfig{Storage: config.StorageConfig{RepoCacheDir: repoCacheDir}}},
		stateManager: sm,
		repoUpdater:  &RepoUpdater{cache: cache},
	}

	d.handleRepoRemoved(events.RepoRemoved{RepoURL: repoURL, RepoName: repoName})

	require.Nil(t, sm.GetRepository(repoURL))
	require.Nil(t, cache.Get(repoURL, "main"))
	require.Nil(t, cache.Get(repoURL, "dev"))
	_, statErr := os.Stat(repoDir)
	require.Error(t, statErr)
	require.True(t, os.IsNotExist(statErr))
}

func TestDaemon_handleRepoRemoved_DoesNotDeleteOutsideRepoCacheDir(t *testing.T) {
	tmp := t.TempDir()
	repoCacheDir := filepath.Join(tmp, "repo-cache")
	require.NoError(t, os.MkdirAll(repoCacheDir, 0o750))

	outside := filepath.Join(tmp, "outside.txt")
	require.NoError(t, os.WriteFile(outside, []byte("keep"), 0o600))

	svcResult := state.NewService(tmp)
	require.True(t, svcResult.IsOk())
	sm := state.NewServiceAdapter(svcResult.Unwrap())
	d := &Daemon{
		config:       &config.Config{Daemon: &config.DaemonConfig{Storage: config.StorageConfig{RepoCacheDir: repoCacheDir}}},
		stateManager: sm,
		repoUpdater:  &RepoUpdater{cache: &git.RemoteHeadCache{}},
	}

	d.handleRepoRemoved(events.RepoRemoved{RepoURL: "https://example.com/evil.git", RepoName: "../outside.txt"})

	// Should not delete anything outside the repo cache directory.
	_, err := os.Stat(outside)
	require.NoError(t, err)
}
