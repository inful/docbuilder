package daemon

import (
	"testing"

	"github.com/stretchr/testify/require"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// A daemon that is stopped before it is started closes its stores, and
// a daemon that is started again runs with closed stores: every build
// event goes into a closed store, and the queue only logs it as a
// warning. The build history and the search index stay empty.
func TestTheStoresAreOpenedAgainAfterARestart(t *testing.T) {
	stateDir := t.TempDir()
	store := &fakeEventStore{}

	d := daemonForTest(t, daemonForTestOpts{
		Config: &config.Config{Daemon: &config.DaemonConfig{
			Storage: config.StorageConfig{StateDir: stateDir},
		}},
		StateManager: newFakeDaemonStateManager(),
	})
	d.state.eventStore = store

	// The daemon is stopped before it is started: the stores are
	// closed.
	d.lifecycle.status.Store(StatusStopped)
	require.NoError(t, d.Stop(t.Context()))
	require.True(t, store.closed, "the stores were not closed")

	// The daemon is started again: the stores are opened again.
	require.NoError(t, d.reopenStores(t.Context()))
	require.False(t, d.lifecycle.storesClosed.Load(), "the stores stayed closed: the daemon writes no events")
	require.NotNil(t, d.state.eventStore, "the event store was not opened again")
	require.NotSame(t, store, d.state.eventStore, "the closed event store is still in use")
}
