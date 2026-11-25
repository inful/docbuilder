package daemon

import (
	"testing"

	"github.com/stretchr/testify/require"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// The cache of the remote heads is the shared memory of the updater
// and the build: the updater reads it to know whether a repository
// moved, and the build writes it when it fetches a repository. Two
// caches would overwrite each other's file, and the updater would not
// see what the build learned.
func TestTheDaemonSharesTheCacheOfTheRemoteHeads(t *testing.T) {
	cfg := &config.Config{
		Version: "2.0",
		Repositories: []config.Repository{{
			Name:   "org/repo",
			URL:    "https://forge.example.com/org/repo.git",
			Branch: "main",
		}},
		Output: config.OutputConfig{Directory: t.TempDir()},
		Daemon: &config.DaemonConfig{
			HTTP: config.HTTPConfig{
				DocsPort:    18190,
				WebhookPort: 18191,
				AdminPort:   18192,
			},
			Storage: config.StorageConfig{
				RepoCacheDir: t.TempDir(),
				StateDir:     t.TempDir(),
			},
			Sync: config.SyncConfig{
				Schedule:         "@every 1h",
				ConcurrentBuilds: 1,
				QueueSize:        10,
			},
		},
	}

	d, err := NewDaemonWithConfigFile(cfg, "")
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = d.Stop(t.Context())
		if d.state.eventStore != nil {
			_ = d.state.eventStore.Close()
		}
	})

	require.NotNil(t, d.build.remoteHeadCache, "the daemon has no cache of the remote heads")
	require.NotNil(t, d.build.repoUpdater, "the daemon has no updater")
	require.Same(t, d.build.remoteHeadCache, d.build.repoUpdater.cache,
		"the updater and the build use different caches")
}
