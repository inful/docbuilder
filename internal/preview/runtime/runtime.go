// Package runtime exposes the narrow Runtime surface that the preview CLI
// needs. It exists so the preview package does not have to import the
// (much larger) daemon package just to obtain a LiveReloadHub.
//
// Runtime satisfies the httpserver.Runtime interface, but preview mode
// only uses a tiny subset of those methods (the build status tracker is
// passed alongside, and the httpserver is told to not start the admin /
// docs / webhook servers). The other methods are no-ops returning zero
// values; they exist purely so the type satisfies httpserver.Runtime.
package runtime

import (
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/build/queue"
)

// Runtime is the preview-mode substitute for the daemon. It exposes
// only what the preview HTTP server needs: a LiveReloadHub. All other
// httpserver.Runtime methods are stubs returning zero values.
type Runtime struct {
	liveReload queue.LiveReloadHub
}

// New constructs a preview Runtime. The hub is a fresh in-memory
// LiveReloadHub with no metrics wiring (preview doesn't expose metrics).
func New() *Runtime {
	return &Runtime{
		liveReload: newHub(),
	}
}

// LiveReloadHub returns the hub for broadcasting rebuild notifications
// to connected SSE clients.
func (r *Runtime) LiveReloadHub() queue.LiveReloadHub { return r.liveReload }

// --- httpserver.Runtime surface ---
//
// Preview's HTTP server doesn't run admin/docs/webhook routes (preview
// only serves the docs site + livereload SSE), so these are no-ops.

func (r *Runtime) GetStatus() string             { return "running" }
func (r *Runtime) GetActiveJobs() int            { return 0 }
func (r *Runtime) GetStartTime() time.Time       { return time.Time{} }
func (r *Runtime) HTTPRequestsTotal() int        { return 0 }
func (r *Runtime) RepositoriesTotal() int        { return 0 }
func (r *Runtime) LastDiscoveryDurationSec() int { return 0 }
func (r *Runtime) LastBuildDurationSec() int     { return 0 }
func (r *Runtime) GetQueueLength() int           { return 0 }
func (r *Runtime) TriggerDiscovery() string      { return "" }
func (r *Runtime) TriggerBuild() string          { return "" }

// TriggerWebhookBuild is a no-op for preview mode.
func (r *Runtime) TriggerWebhookBuild(_, _, _ string, _ []string) string {
	return ""
}
