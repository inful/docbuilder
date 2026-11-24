package daemon

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/state"
)

// A daemon whose start failed halfway has started part of itself: the
// HTTP server runs, the build queue has its workers, the event store
// and the search index are open. The teardown has to run: the ports
// have to be released and the state has to be written.
func TestTheTeardownOfAFailedStartWritesTheState(t *testing.T) {
	stateDir := t.TempDir()
	sm := state.NewServiceAdapter(state.NewService(stateDir).Unwrap())

	cfg := &config.Config{
		Daemon: &config.DaemonConfig{
			HTTP:    config.HTTPConfig{DocsPort: 0, WebhookPort: 0, AdminPort: 0},
			Sync:    config.SyncConfig{Schedule: "every 4h"},
			Storage: config.StorageConfig{StateDir: stateDir},
		},
	}

	d := daemonForTest(t, daemonForTestOpts{Config: cfg, StateManager: sm})

	d.stopAfterFailedStart(t.Context())

	entries, err := os.ReadDir(stateDir)
	require.NoError(t, err)
	require.NotEmpty(t, entries, "the state of the daemon was not written")
}
