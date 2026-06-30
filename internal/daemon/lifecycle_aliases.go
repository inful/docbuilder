package daemon

// Type aliases for the lifecycle sub-package. The daemon owns a single
// Scheduler and WorkerGroup instance whose types live in the lifecycle
// sub-package; these aliases preserve the daemon package's existing
// field types and constructor signatures so call sites in this package
// and the test suite don't need to be rewritten during the carve.
//
// The aliases resolve to the same underlying types, so the existing
// field declarations (e.g. `scheduler *Scheduler`) continue to compile
// unchanged.

import "git.home.luguber.info/inful/docbuilder/internal/daemon/lifecycle"

type (
	Scheduler   = lifecycle.Scheduler
	WorkerGroup = lifecycle.WorkerGroup
)

// NewScheduler delegates to the lifecycle package's NewScheduler. The
// wrapper keeps the daemon's existing constructor signature stable for
// the test suite (which calls daemon.NewScheduler directly) and for
// NewDaemonWithConfigFile's call site.
func NewScheduler() (*Scheduler, error) {
	return lifecycle.NewScheduler()
}
