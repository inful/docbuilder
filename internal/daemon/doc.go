// Package daemon implements the long-running DocBuilder service.
//
// # Architecture
//
// The package owns the Daemon struct, which is a thin façade over
// six per-concern sub-structs:
//
//   - lifecycleStack  — status, run coordination, scheduled-job bookkeeping
//   - stateStack      — state manager, event store, projection, emitter
//   - buildStack      — BuildService, queue, debouncer, repo updater
//   - discoveryStack  — forge manager, discovery service, cache, runner
//   - serveStack      — HTTP server, scheduler, orchestration bus, metrics
//   - optionalStack   — opt-in features (livereload, link verifier,
//     outbound dispatcher), any of which may be nil
//
// The Daemon struct itself holds only the configuration reference,
// the two shared atomic counters, and pointers to the sub-structs.
// Adding a new dependency now touches one sub-struct, not the
// top-level surface. The full design rationale is in
// docs/adr/adr-023-daemon-decomposition.md.
//
// # Construction
//
// NewDaemonWithConfigFile is a 35-line recipe that calls nine
// per-concern init* methods in dependency order. Each method owns
// one subsystem and either returns an error or is safe to ignore.
// Reading the recipe top-to-bottom is the fastest way to see what
// a fully-initialized daemon contains.
//
// # Test seam
//
// Tests that build a partial Daemon go through the
// daemonForTest(t, opts) helper in daemon_test_helpers.go. The
// helper accepts a small opts struct that names each seam the
// test wants to wire up. Fields the test leaves unset get a
// non-nil empty sub-struct, mirroring the production
// constructor's behavior. Direct &Daemon{...} literals are not
// used in the test suite.
//
// # Public API stability
//
// The Daemon's exported method set (Start, Stop, GetStatus, Build,
// and the queue.Builder / queue.BuildEventEmitter interface
// assertions) is stable. The single external caller is
// cmd/docbuilder/commands/daemon.go.
package daemon
