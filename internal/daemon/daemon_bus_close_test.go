package daemon

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/daemon/events"
)

// The bus is closed after the workers: a worker that publishes while it
// stops must not find a closed bus, and the event it publishes would be
// lost.
func TestTheBusIsClosedAfterTheWorkersStop(t *testing.T) {
	bus := events.NewBus()
	defer bus.Close()

	d := daemonForTest(t, daemonForTestOpts{
		Config:           &config.Config{Daemon: &config.DaemonConfig{}},
		OrchestrationBus: bus,
	})

	// The worker holds the stop of the daemon.
	release := make(chan struct{})
	started := make(chan struct{})
	d.goWorker("blocked", func() {
		close(started)
		<-release
	})
	<-started

	stopDone := make(chan error, 1)
	go func() { stopDone <- d.Stop(t.Context()) }()

	// The stop waits for the worker: the bus must stay open.
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if err := bus.Publish(t.Context(), events.BuildRequested{}); err != nil {
			close(release)
			t.Fatalf("the bus was closed while the worker was running: %v", err)
		}
		time.Sleep(5 * time.Millisecond)
	}

	// The worker is done: the stop finishes and the bus is closed.
	close(release)
	select {
	case err := <-stopDone:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("the daemon did not stop")
	}

	require.Error(t, bus.Publish(t.Context(), events.BuildRequested{}), "the bus is closed after the stop")
}
