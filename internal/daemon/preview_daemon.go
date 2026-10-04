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
