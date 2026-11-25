package daemon

import (
	"testing"

	"github.com/stretchr/testify/require"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// The event store and the search index are closed when the daemon
// stops, and they are opened only in the constructor: a daemon that is
// started again after a stop wrote no events and no search index, and
// the endpoints of the history answered nothing.
func TestTheStoresStayOpenAfterTheStop(t *testing.T) {
	store := &fakeEventStore{}

	d := daemonForTest(t, daemonForTestOpts{
		Config: &config.Config{Daemon: &config.DaemonConfig{}},
	})
	d.state.eventStore = store

	require.NoError(t, d.Stop(t.Context()))
	require.False(t, store.closed, "the event store was closed: the daemon writes no events after a restart")
}
