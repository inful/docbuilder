package daemon

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/daemon/events"
)

// The request to check the repository waits for room in the queue of
// the updater instead of being dropped: the forge does not send the
// webhook again, and the commit it announced was published only at the
// next sync of the daemon.
func TestTheRequestToCheckTheRepositoryWaitsForRoom(t *testing.T) {
	bus := events.NewBus()
	defer bus.Close()

	received, unsubscribe := events.Subscribe[events.RepoUpdateRequested](bus, 1)
	defer unsubscribe()

	d := daemonForTest(t, daemonForTestOpts{
		Config:           &config.Config{Daemon: &config.DaemonConfig{}},
		OrchestrationBus: bus,
	})

	// The queue of the updater is full.
	require.NoError(t, bus.Publish(t.Context(), events.RepoUpdateRequested{JobID: "first"}))

	// The request waits for room.
	d.retryRepoUpdateInBackground(t.Context(), events.RepoUpdateRequested{JobID: "second"})

	<-received
	select {
	case evt := <-received:
		require.Equal(t, "second", evt.JobID)
	case <-time.After(10 * time.Second):
		t.Fatal("the request was not delivered")
	}
}
