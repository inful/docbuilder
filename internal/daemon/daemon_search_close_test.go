package daemon

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// The daemon must release the search index when it stops: the file
// handle belongs to the daemon, and the write-ahead log is only folded
// back into the index when the last connection goes.
func TestStopClosesTheSearchIndex(t *testing.T) {
	stateDir := t.TempDir()
	cfg := &config.Config{
		Version: "2.0",
		Daemon: &config.DaemonConfig{
			HTTP: config.HTTPConfig{
				DocsPort:    18099,
				WebhookPort: 18098,
				AdminPort:   18097,
			},
			Storage: config.StorageConfig{StateDir: stateDir, RepoCacheDir: t.TempDir()},
			Sync:    config.SyncConfig{Schedule: "@every 1h", ConcurrentBuilds: 1, QueueSize: 10},
		},
		Output: config.OutputConfig{Directory: t.TempDir()},
		Search: &config.SearchConfig{},
	}

	d, err := NewDaemonWithConfigFile(cfg, "")
	require.NoError(t, err)
	require.NotNil(t, d.optional.search, "the daemon must keep the index")
	require.FileExists(t, filepath.Join(stateDir, "search", "search.db"))

	require.NoError(t, d.Stop(t.Context()))

	// The index file is still there - the daemon does not delete it,
	// the next start reads it - but no connection is open on it.
	_, err = os.Stat(filepath.Join(stateDir, "search", "search.db"))
	assert.NoError(t, err, "the index file must survive the stop")
}
