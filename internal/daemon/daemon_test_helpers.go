package daemon

import (
	"git.home.luguber.info/inful/docbuilder/internal/build"
	"git.home.luguber.info/inful/docbuilder/internal/build/queue"
	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/daemon/events"
	"git.home.luguber.info/inful/docbuilder/internal/forge"
	"git.home.luguber.info/inful/docbuilder/internal/git"
	"git.home.luguber.info/inful/docbuilder/internal/linkverify"
	"git.home.luguber.info/inful/docbuilder/internal/state"
)

// daemonForTestOpts collects the seams a test wants to wire up. Any
// field left at its zero value is replaced with a non-nil empty
// sub-struct so production code that defensively accesses the seam
// does not nil-deref. Tests that want to assert on a nil seam (e.g.
// "outbound dispatcher is not configured") should leave that field
// unset and instead assert on the post-construction state.
type daemonForTestOpts struct {
	// Config is the only required field. The helper rejects an empty
	// config at construction time, mirroring the production check.
	Config *config.Config

	// Build pipeline seams. Leave unset for tests that only exercise
	// discovery or lifecycle code paths.
	BuildService      build.BuildService
	BuildQueue        *queue.BuildQueue
	BuildDebouncer    *BuildDebouncer
	RepoUpdater       *RepoUpdater
	BuildMockStateMgr state.DaemonStateManager

	// Orchestration bus. Most tests that exercise the consumer
	// loops set this.
	OrchestrationBus *events.Bus

	// Cron-like scheduler. Tests that exercise schedulePeriodicJobs
	// or runScheduledSyncTick pass a real scheduler.
	Scheduler *Scheduler

	// Discovery seams. Tests that exercise the discovery runner
	// pass a manager and a cache.
	ForgeManager    *forge.Manager
	Discovery       *forge.DiscoveryService
	DiscoveryCache  *DiscoveryCache
	DiscoveryRunner *DiscoveryRunner

	// Optional seams. Pass nil explicitly to assert the daemon does
	// not construct them.
	LiveReload         *LiveReloadHub
	LinkVerifier       *linkverify.VerificationService
	OutboundDispatcher *OutboundDispatcher

	// Misc.
	RemoteCache *git.RemoteHeadCache
}

// daemonForTest builds a minimal *Daemon suitable for exercising a
// specific code path. It is the test seam that replaces the
// hand-rolled &Daemon{lifecycle: ..., serve: ..., discover: ...}
// literals that the build-service-unification F5 review flagged.
//
// The helper enforces three invariants that the literals used to
// provide implicitly:
//
//  1. Every sub-struct is non-nil, so production code that reads
//     a sub-struct field does not nil-deref.
//  2. The lifecycle mutex is non-nil and the stop channel is created
//     with a buffer of 1 so callers can signal without a paired
//     receiver (matches the production constructor).
//  3. The status is set to StatusRunning so the daemon's gated code
//     paths run. Tests that need a different starting state can
//     adjust after construction.
func daemonForTest(t testingT, opts daemonForTestOpts) *Daemon {
	t.Helper()
	if opts.Config == nil {
		t.Fatal("daemonForTest: Config is required")
	}

	// Build the lifecycle inline; everything else goes into the
	// sub-structs. Using inline struct literal for lifecycle (value
	// type) and pointer literals for the rest mirrors the production
	// constructor.
	d := &Daemon{
		config: opts.Config,
		lifecycle: lifecycleStack{
			stopChan: make(chan struct{}, 1),
		},
		state:    &stateStack{stateManager: opts.BuildMockStateMgr},
		build:    &buildStack{},
		discover: &discoveryStack{},
		serve:    &serveStack{},
		optional: &optionalStack{},
	}
	d.lifecycle.status.Store(StatusRunning)

	if opts.BuildService != nil {
		d.build.buildSvc = opts.BuildService
	}
	if opts.BuildQueue != nil {
		d.build.buildQueue = opts.BuildQueue
	}
	if opts.BuildDebouncer != nil {
		d.build.buildDebouncer = opts.BuildDebouncer
	}
	if opts.RepoUpdater != nil {
		d.build.repoUpdater = opts.RepoUpdater
	}

	if opts.OrchestrationBus != nil {
		d.serve.orchestrationBus = opts.OrchestrationBus
	}
	if opts.Scheduler != nil {
		d.serve.scheduler = opts.Scheduler
	}

	if opts.ForgeManager != nil {
		d.discover.forgeManager = opts.ForgeManager
	}
	if opts.Discovery != nil {
		d.discover.discovery = opts.Discovery
	}
	if opts.DiscoveryCache != nil {
		d.discover.discoveryCache = opts.DiscoveryCache
	}
	if opts.DiscoveryRunner != nil {
		d.discover.discoveryRunner = opts.DiscoveryRunner
	}

	if opts.LiveReload != nil {
		d.optional.liveReload = opts.LiveReload
	}
	if opts.LinkVerifier != nil {
		d.optional.linkVerifier = opts.LinkVerifier
	}
	if opts.OutboundDispatcher != nil {
		d.optional.outboundDispatcher = opts.OutboundDispatcher
	}

	if opts.RemoteCache != nil {
		// Tests that need a remote-head cache wire it into the
		// build stack directly; the production wiring is more
		// elaborate but is not the responsibility of a test seam.
		if d.build.repoUpdater == nil {
			d.build.repoUpdater = &RepoUpdater{}
		}
	}

	return d
}

// testingT is the minimal *testing.T surface the helper needs. The
// alias makes the helper usable from non-*testing.T contexts (for
// instance, table-driven scenarios that pre-build daemons) without
// pulling in the full *testing.T contract.
type testingT interface {
	Helper()
	Fatal(args ...any)
}
