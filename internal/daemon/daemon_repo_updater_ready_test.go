package daemon

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"git.home.luguber.info/inful/docbuilder/internal/daemon/events"
)

// The readiness signal of the updater belongs to the run that
// subscribed: after a restart the signal of the last run is closed, and
// a caller that waits for it proceeds before the new run has
// subscribed.
func TestTheUpdaterGetsANewReadinessSignalAfterARestart(t *testing.T) {
	bus := events.NewBus()
	defer bus.Close()

	updater := NewRepoUpdater(bus, alwaysChangedRemoteHeadChecker{}, nil, nil)

	first := updater.Ready()
	require.NotNil(t, first)

	// The daemon is started again.
	updater.SetBus(bus)

	second := updater.Ready()
	require.NotEqual(t, first, second, "the readiness signal of the last run is still there")

	// The signal of the new run is closed when the updater subscribes.
	go updater.Run(t.Context())
	select {
	case <-second:
	case <-time.After(2 * time.Second):
		t.Fatal("the updater did not report that it subscribed")
	}
}
