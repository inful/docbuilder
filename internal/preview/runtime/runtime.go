// Package runtime exposes the narrow Runtime surface that the preview CLI
// needs. The httpserver.New(...) entry point only requires Status for
// preview-mode wiring; trigger and metrics surfaces are nil since
// preview does not run admin/webhook/metrics routes. This file drops the
// 9 zero-value stubs that the previous httpserver.Runtime interface
// required.
package runtime

import (
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/build/queue"
)

// Runtime is the preview-mode substitute for the daemon. It exposes
// only what the preview HTTP server needs: a Status triple plus a
// LiveReloadHub.
type Runtime struct {
	liveReload queue.LiveReloadHub
	startTime  time.Time
}

// New constructs a preview Runtime. The hub is a fresh in-memory
// LiveReloadHub with no metrics wiring (preview doesn't expose metrics).
func New() *Runtime {
	return &Runtime{
		liveReload: newHub(),
		startTime:  time.Now(),
	}
}

// Status triple (required by httpserver.New).

// GetStatus returns the preview runtime status. Preview only runs the docs
// site + livereload SSE; admin/webhook routes are not registered.
func (r *Runtime) GetStatus() string { return "preview" }

// GetActiveJobs is zero in preview (builds are not run as jobs).
func (r *Runtime) GetActiveJobs() int { return 0 }

// GetStartTime records when the preview process started; surfaced for /status.
func (r *Runtime) GetStartTime() time.Time { return r.startTime }

// LiveReloadHub returns the hub for broadcasting rebuild notifications
// to connected SSE clients.
func (r *Runtime) LiveReloadHub() queue.LiveReloadHub { return r.liveReload }
