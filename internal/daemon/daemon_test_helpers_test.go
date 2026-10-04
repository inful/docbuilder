package daemon

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/daemon/events"
	"git.home.luguber.info/inful/docbuilder/internal/forge"
	"git.home.luguber.info/inful/docbuilder/internal/state"
)

// TestDaemonForTest_AppliesOpts locks the helper's invariants. If a
// future change to daemonForTest silently drops an option (for
// example, a typo in a field assignment), this test catches it
// directly without depending on every consumer of the helper.
func TestDaemonForTest_AppliesOpts(t *testing.T) {
	sm := state.NewServiceAdapter(state.NewService(t.TempDir()).Unwrap())
	bus := events.NewBus()
	defer bus.Close()

	cfg := &config.Config{Daemon: &config.DaemonConfig{}}
	fm := forge.NewForgeManager()
	sch := &Scheduler{}

	d := daemonForTest(t, daemonForTestOpts{
		Config:           cfg,
		StateManager:     sm,
		OrchestrationBus: bus,
		ForgeManager:     fm,
		Scheduler:        sch,
	})

	require.NotNil(t, d, "helper must return a non-nil daemon")
	require.Same(t, cfg, d.config, "Config must be stored on the façade")
	require.Same(t, sm, d.state.stateManager, "StateManager must populate state.stateManager")
	require.Same(t, bus, d.serve.orchestrationBus, "OrchestrationBus must populate serve.orchestrationBus")
	require.Same(t, fm, d.discover.forgeManager, "ForgeManager must populate discover.forgeManager")
	require.Same(t, sch, d.serve.scheduler, "Scheduler must populate serve.scheduler")

	// Sub-struct defaults: every sub-struct must be non-nil so
	// production code that reads a field does not nil-deref.
	require.NotNil(t, d.state)
	require.NotNil(t, d.build)
	require.NotNil(t, d.discover)
	require.NotNil(t, d.serve)
	require.NotNil(t, d.optional)

	// Default status is StatusRunning. Tests that exercise the
	// stopped/stopping paths adjust after this call.
	require.Equal(t, StatusRunning, d.GetStatus())
}

// TestDaemonForTest_NilSubStructsByDefault locks the contract that
// seams a test does not pass remain at their zero value. The
// production constructor always populates every field; the helper
// is selective. Any test that relies on an unset seam being nil
// depends on this.
func TestDaemonForTest_NilSubStructsByDefault(t *testing.T) {
	d := daemonForTest(t, daemonForTestOpts{
		Config: &config.Config{Daemon: &config.DaemonConfig{}},
	})

	require.Nil(t, d.build.buildSvc, "BuildService must be nil when not provided")
	require.Nil(t, d.build.buildQueue, "BuildQueue must be nil when not provided")
	require.Nil(t, d.build.buildDebouncer, "BuildDebouncer must be nil when not provided")
	require.Nil(t, d.build.repoUpdater, "RepoUpdater must be nil when not provided")
	require.Nil(t, d.discover.discovery, "Discovery must be nil when not provided")
	require.Nil(t, d.discover.discoveryRunner, "DiscoveryRunner must be nil when not provided")
	require.Nil(t, d.optional.liveReload, "LiveReload must be nil when not provided")
	require.Nil(t, d.optional.linkVerifier, "LinkVerifier must be nil when not provided")
	require.Nil(t, d.optional.outboundDispatcher, "OutboundDispatcher must be nil when not provided")
}

// TestDaemonForTest_RequiresConfig locks the failure mode: the
// helper refuses to construct a daemon with no config. This
// mirrors the production constructor's nil check.
func TestDaemonForTest_RequiresConfig(t *testing.T) {
	rec := &fakeTRecorder{}
	_ = daemonForTest(rec, daemonForTestOpts{})
	require.True(t, rec.fatalCalled.Load(), "daemonForTest must call Fatal when Config is nil")
}

// TestDaemonForTest_StopChanBuffered locks the buffer-size contract.
// The production constructor uses an unbuffered channel; the helper
// uses a buffered one so tests can close the daemon without a
// paired receiver. A regression that drops the buffer would break
// the cleanup pattern in every test using t.Cleanup.
func TestDaemonForTest_StopChanBuffered(t *testing.T) {
	d := daemonForTest(t, daemonForTestOpts{
		Config: &config.Config{Daemon: &config.DaemonConfig{}},
	})

	require.NotNil(t, d.lifecycle.stopChan, "stopChan must be initialized")
	// The channel must accept at least one send without blocking.
	// Use a goroutine to send and confirm we do not deadlock.
	done := make(chan struct{})
	go func() {
		d.lifecycle.stopChan <- struct{}{}
		close(done)
	}()
	select {
	case <-done:
		// success
	case <-timeAfter(50):
		t.Fatal("stopChan appears to be unbuffered: send blocked")
	}
}

// timeAfter returns a one-shot channel that fires after n
// milliseconds. Used to bound a test that could otherwise hang.
func timeAfter(n int) <-chan struct{} {
	ch := make(chan struct{})
	go func() {
		t := time.NewTimer(time.Duration(n) * 1_000_000)
		defer t.Stop()
		<-t.C
		close(ch)
	}()
	return ch
}

// fakeTRecorder is a minimal testingT implementation that records
// whether Fatal was called. Used to verify the helper's required-
// field check without failing the enclosing test.
type fakeTRecorder struct {
	fatalCalled atomic.Bool
}

func (f *fakeTRecorder) Helper()           {}
func (f *fakeTRecorder) Fatal(args ...any) { f.fatalCalled.Store(true) }
