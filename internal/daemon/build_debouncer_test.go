package daemon

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/daemon/events"
	"github.com/stretchr/testify/require"
)

func TestBuildDebouncer_BurstCoalescesToSingleBuild(t *testing.T) {
	bus := events.NewBus()
	defer bus.Close()

	var running atomic.Bool
	debouncer, err := NewBuildDebouncer(bus, BuildDebouncerConfig{
		QuietWindow:       25 * time.Millisecond,
		MaxDelay:          200 * time.Millisecond,
		CheckBuildRunning: running.Load,
		PollInterval:      10 * time.Millisecond,
	})
	require.NoError(t, err)

	buildNowCh, unsub := events.Subscribe[events.BuildNow](bus, 10)
	defer unsub()

	ctx := t.Context()

	go func() { _ = debouncer.Run(ctx) }()

	select {
	case <-debouncer.Ready():
	case <-time.After(250 * time.Millisecond):
		t.Fatal("timed out waiting for debouncer ready")
	}

	for range 5 {
		require.NoError(t, bus.Publish(context.Background(), events.BuildRequested{Reason: "test"}))
		time.Sleep(5 * time.Millisecond)
	}

	select {
	case got := <-buildNowCh:
		require.GreaterOrEqual(t, got.RequestCount, 1)
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for BuildNow")
	}

	select {
	case <-buildNowCh:
		t.Fatal("expected only one BuildNow for burst")
	case <-time.After(75 * time.Millisecond):
		// ok
	}
}

func TestBuildDebouncer_MaxDelayForcesBuild(t *testing.T) {
	bus := events.NewBus()
	defer bus.Close()

	var running atomic.Bool
	debouncer, err := NewBuildDebouncer(bus, BuildDebouncerConfig{
		QuietWindow:       200 * time.Millisecond, // would postpone forever if requests keep coming
		MaxDelay:          60 * time.Millisecond,
		CheckBuildRunning: running.Load,
		PollInterval:      10 * time.Millisecond,
	})
	require.NoError(t, err)

	buildNowCh, unsub := events.Subscribe[events.BuildNow](bus, 10)
	defer unsub()

	ctx := t.Context()
	go func() { _ = debouncer.Run(ctx) }()

	select {
	case <-debouncer.Ready():
	case <-time.After(250 * time.Millisecond):
		t.Fatal("timed out waiting for debouncer ready")
	}

	deadline := time.Now().Add(150 * time.Millisecond)
	for time.Now().Before(deadline) {
		require.NoError(t, bus.Publish(context.Background(), events.BuildRequested{Reason: "test"}))
		time.Sleep(10 * time.Millisecond)
	}

	select {
	case got := <-buildNowCh:
		require.Equal(t, "max_delay", got.DebounceCause)
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for max-delay BuildNow")
	}
}

func TestBuildDebouncer_BuildRunningQueuesOneFollowUp(t *testing.T) {
	bus := events.NewBus()
	defer bus.Close()

	var running atomic.Bool
	running.Store(true)

	debouncer, err := NewBuildDebouncer(bus, BuildDebouncerConfig{
		QuietWindow:       20 * time.Millisecond,
		MaxDelay:          50 * time.Millisecond,
		CheckBuildRunning: running.Load,
		PollInterval:      10 * time.Millisecond,
	})
	require.NoError(t, err)

	buildNowCh, unsub := events.Subscribe[events.BuildNow](bus, 10)
	defer unsub()

	ctx := t.Context()
	go func() { _ = debouncer.Run(ctx) }()

	select {
	case <-debouncer.Ready():
	case <-time.After(250 * time.Millisecond):
		t.Fatal("timed out waiting for debouncer ready")
	}

	for range 10 {
		require.NoError(t, bus.Publish(context.Background(), events.BuildRequested{Reason: "test"}))
	}

	select {
	case <-buildNowCh:
		t.Fatal("expected no BuildNow while build is running")
	case <-time.After(100 * time.Millisecond):
		// ok
	}

	running.Store(false)

	select {
	case <-buildNowCh:
		// ok
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for follow-up BuildNow")
	}

	select {
	case <-buildNowCh:
		t.Fatal("expected exactly one follow-up BuildNow")
	case <-time.After(75 * time.Millisecond):
		// ok
	}
}

func TestBuildDebouncer_ImmediateEmitsBuildNow(t *testing.T) {
	bus := events.NewBus()
	defer bus.Close()

	var running atomic.Bool
	debouncer, err := NewBuildDebouncer(bus, BuildDebouncerConfig{
		QuietWindow:       200 * time.Millisecond,
		MaxDelay:          500 * time.Millisecond,
		CheckBuildRunning: running.Load,
		PollInterval:      10 * time.Millisecond,
	})
	require.NoError(t, err)

	buildNowCh, unsub := events.Subscribe[events.BuildNow](bus, 10)
	defer unsub()

	ctx := t.Context()
	go func() { _ = debouncer.Run(ctx) }()

	select {
	case <-debouncer.Ready():
	case <-time.After(250 * time.Millisecond):
		t.Fatal("timed out waiting for debouncer ready")
	}

	require.NoError(t, bus.Publish(context.Background(), events.BuildRequested{Reason: "webhook", Immediate: true}))

	select {
	case got := <-buildNowCh:
		require.Equal(t, "immediate", got.DebounceCause)
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timed out waiting for immediate BuildNow")
	}
}
