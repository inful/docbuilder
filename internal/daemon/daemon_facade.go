package daemon

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/build"
	"git.home.luguber.info/inful/docbuilder/internal/build/queue"
	"git.home.luguber.info/inful/docbuilder/internal/daemon/events"
	"git.home.luguber.info/inful/docbuilder/internal/eventstore"
	"git.home.luguber.info/inful/docbuilder/internal/forge"
	"git.home.luguber.info/inful/docbuilder/internal/linkverify"
	"git.home.luguber.info/inful/docbuilder/internal/server/httpserver"
	"git.home.luguber.info/inful/docbuilder/internal/state"
)

// lifecycleStack owns the daemon's lifecycle state: status, run
// coordination, the background worker group, and the bookkeeping for
// scheduled jobs. The lifecycle mutex guards every non-atomic field
// here. Fields declared atomic may be read or written without the
// lock.
type lifecycleStack struct {
	mu          sync.RWMutex
	status      atomic.Value // holds Status
	startTime   time.Time
	stopChan    chan struct{}
	runCancel   context.CancelFunc
	workers     WorkerGroup
	lastBuild   *time.Time
	syncJobID   string
	statusJobID string
	promJobID   string
}

// stateStack owns the daemon's persistence layer: the typed state
// manager that survives restarts, the append-only event store, the
// read-side projection, and the emitter that translates daemon events
// into the store.
type stateStack struct {
	stateManager    state.DaemonStateManager
	eventStore      eventstore.Store
	buildProjection *eventstore.BuildHistoryProjection
	eventEmitter    *EventEmitter
}

// buildStack owns the daemon's build pipeline: the canonical
// BuildService, the job queue, the debouncer, the git-backed repo
// updater, and the mutex that serializes concurrent builds against
// shared staging paths.
type buildStack struct {
	buildSvc       build.BuildService
	buildMu        sync.Mutex
	buildQueue     *queue.BuildQueue
	buildDebouncer *BuildDebouncer
	repoUpdater    *RepoUpdater
}

// discoveryStack owns the discovery pipeline: the forge manager, the
// discovery service, the cache, and the runner that orchestrates
// discovery-driven builds.
type discoveryStack struct {
	forgeManager    *forge.Manager
	discovery       *forge.DiscoveryService
	discoveryCache  *DiscoveryCache
	discoveryRunner *DiscoveryRunner
}

// serveStack owns the daemon's runtime services: the HTTP server, the
// cron-like scheduler, the in-process orchestration event bus, and the
// metrics collector.
type serveStack struct {
	httpServer       *httpserver.Server
	scheduler        *Scheduler
	orchestrationBus *events.Bus
	metrics          *MetricsCollector
}

// optionalStack owns features that are opt-in via configuration and
// may be nil. Centralizing them in one sub-struct makes the nil-check
// pattern uniform across the daemon.
type optionalStack struct {
	liveReload         *LiveReloadHub
	linkVerifier       *linkverify.VerificationService
	outboundDispatcher *OutboundDispatcher
}
