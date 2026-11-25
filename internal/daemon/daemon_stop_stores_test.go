package daemon

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/eventstore"
)

// A daemon that is stopped without being started still holds the event
// store and the search index: they are opened when the daemon is built,
// and nobody closes them otherwise.
func TestTheDaemonThatIsStoppedWithoutBeingStartedClosesItsStores(t *testing.T) {
	store := &fakeEventStore{}
	d := daemonForTest(t, daemonForTestOpts{
		Config: &config.Config{Daemon: &config.DaemonConfig{}},
	})
	// A daemon that was built and never started.
	d.lifecycle.status.Store(StatusStopped)
	d.state.eventStore = store

	require.NoError(t, d.Stop(t.Context()))
	require.True(t, store.closed, "the event store was not closed")
}

// fakeEventStore is an event store that remembers that it was closed.
type fakeEventStore struct {
	closed bool
}

func (f *fakeEventStore) Append(context.Context, string, string, []byte, map[string]string) error {
	return nil
}

func (f *fakeEventStore) GetByBuildID(context.Context, string) ([]eventstore.Event, error) {
	return nil, nil
}

func (f *fakeEventStore) GetRange(context.Context, time.Time, time.Time) ([]eventstore.Event, error) {
	return nil, nil
}

func (f *fakeEventStore) GetRecent(context.Context, int) ([]eventstore.Event, error) {
	return nil, nil
}

func (f *fakeEventStore) Prune(context.Context, time.Time) (int64, error) {
	return 0, nil
}

func (f *fakeEventStore) Close() error {
	f.closed = true
	return nil
}
