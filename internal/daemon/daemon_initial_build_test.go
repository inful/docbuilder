package daemon

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/daemon/events"
)

// The daemon asks for the first build when it starts. The request goes
// through the bus, and the bus has no queue: a request that is
// published before the debouncer is subscribed is lost, and the site is
// never built.
func TestTheFirstBuildRequestWaitsForTheDebouncer(t *testing.T) {
	bus := events.NewBus()
	defer bus.Close()

	requested, unsubRequested := events.Subscribe[events.BuildRequested](bus, 4)
	defer unsubRequested()
	buildNow, unsubBuildNow := events.Subscribe[events.BuildNow](bus, 4)
	defer unsubBuildNow()

	debouncer, err := NewBuildDebouncer(bus, BuildDebouncerConfig{
		QuietWindow:  10 * time.Millisecond,
		MaxDelay:     50 * time.Millisecond,
		PollInterval: 10 * time.Millisecond,
	})
	require.NoError(t, err)

	d := daemonForTest(t, daemonForTestOpts{
		Config:           &config.Config{Daemon: &config.DaemonConfig{}},
		OrchestrationBus: bus,
		BuildDebouncer:   debouncer,
	})

	ctx := t.Context()
	go d.requestInitialBuild(ctx)

	// The debouncer is not subscribed yet: the request may not be
	// published.
	select {
	case <-requested:
		t.Fatal("the first build request was published before the debouncer was subscribed")
	case <-time.After(100 * time.Millisecond):
	}

	// The debouncer comes up: the request is published and processed.
	go func() { _ = debouncer.Run(ctx) }()

	select {
	case <-requested:
	case <-time.After(2 * time.Second):
		t.Fatal("the first build request was not published")
	}
	select {
	case <-buildNow:
	case <-time.After(2 * time.Second):
		t.Fatal("the debouncer did not turn the first build request into a build")
	}
}
