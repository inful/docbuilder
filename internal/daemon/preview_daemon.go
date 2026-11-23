package daemon

import (
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// NewPreviewDaemon constructs the minimal daemon instance needed for local preview.
// This keeps local preview wiring out of the main daemon lifecycle.
func NewPreviewDaemon(cfg *config.Config) *Daemon {
	d := &Daemon{
		config: cfg,
		lifecycle: lifecycleStack{
			startTime: time.Now(),
		},
		serve: &serveStack{
			metrics: NewMetricsCollector(),
		},
		// The stacks are there, empty: the monitoring endpoints read
		// them, and a nil stack is a panic in the preview.
		build:    &buildStack{},
		state:    &stateStack{},
		discover: &discoveryStack{},
		optional: &optionalStack{
			liveReload: NewLiveReloadHub(nil),
		},
	}
	d.lifecycle.status.Store(StatusRunning)
	return d
}

// LiveReloadHub exposes the preview-mode hub for broadcasting rebuild notifications.
func (d *Daemon) LiveReloadHub() *LiveReloadHub {
	if d == nil {
		return nil
	}
	return d.optional.liveReload
}
