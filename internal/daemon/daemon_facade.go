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
	"git.home.luguber.info/inful/docbuilder/internal/git"
	"git.home.luguber.info/inful/docbuilder/internal/linkverify"
	"git.home.luguber.info/inful/docbuilder/internal/state"
)

// lifecycleStack owns the daemon's lifecycle state: status, run
// coordination, the background worker group, and the bookkeeping for
// scheduled jobs. The lifecycle mutex guards every non-atomic field
// here. Fields declared atomic may be read or written without the
// lock.
type lifecycleStack struct {
	mu        sync.RWMutex
	status    atomic.Value // holds Status
	startTime time.Time
	stopChan  chan struct{}
	stopOnce  sync.Once
	runCancel context.CancelFunc
	workers   WorkerGroup
	// storesClosed says that the event store and the search index are
	// closed: a daemon that is stopped twice must not close them twice.
	storesClosed atomic.Bool
	lastBuild    *time.Time
	syncJobID    string
	statusJobID  string
	promJobID    string
	pruneJobID   string
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

	// remoteHeadCache holds what the local clones have. The updater
	// reads it to know whether a repository moved, and the build writes
	// it when it fetches a repository: two caches would overwrite each
	// other's file, and the updater would not see what the build
	// learned.
	remoteHeadCache *git.RemoteHeadCache

	// afterRemoval says that a repository was removed. The next build
	// has to run even when no repository is left: the pages of the
	// removed repository are still published, and only a build takes
	// them away.
	afterRemoval atomic.Bool

	// forceFullBuild says that the next build must not be skipped when
	// nothing moved. A reingestion asks for it: the corpus is what is
	// sent to ragabast, and a skipped build sends nothing.
	forceFullBuild atomic.Bool
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

// httpServer is the part of the HTTP server that the daemon uses.
//
// The interface keeps the daemon testable: a test can watch what the
// daemon does when the server starts.
type httpServer interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

// serveStack owns the daemon's runtime services: the HTTP server, the
// cron-like scheduler, the in-process orchestration event bus, and the
// metrics collector.
type serveStack struct {
	httpServer       httpServer
	scheduler        *Scheduler
	orchestrationBus *events.Bus
	metrics          *MetricsCollector
}

// orchestrationBus returns the bus of the current run.
//
// The bus is written when the daemon starts and read by the publishers
// and the triggers: the pointer is read under the lock, so a publisher
// never sees the bus of the last run.
func (d *Daemon) orchestrationBus() *events.Bus {
	if d == nil {
		return nil
	}
	d.lifecycle.mu.RLock()
	defer d.lifecycle.mu.RUnlock()
	return d.serve.orchestrationBus
}

// optionalStack owns features that are opt-in via configuration and
// may be nil. Centralizing them in one sub-struct makes the nil-check
// pattern uniform across the daemon.
type optionalStack struct {
	liveReload         *LiveReloadHub
	linkVerifier       *linkverify.VerificationService
	outboundDispatcher *OutboundDispatcher
	search             *SearchService
}
