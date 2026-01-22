package daemon

import (
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

type previewBuildStatus interface {
	GetStatus() (bool, error, bool)
}

// NewPreviewDaemon constructs the minimal daemon instance needed for local preview.
// This keeps local preview wiring out of the main daemon lifecycle.
func NewPreviewDaemon(cfg *config.Config, buildStatus previewBuildStatus) *Daemon {
	d := &Daemon{
		config:      cfg,
		startTime:   time.Now(),
		metrics:     NewMetricsCollector(),
		liveReload:  NewLiveReloadHub(nil),
		buildStatus: buildStatus,
	}
	d.status.Store(StatusRunning)
	return d
}

// LiveReloadHub exposes the preview-mode hub for broadcasting rebuild notifications.
func (d *Daemon) LiveReloadHub() *LiveReloadHub {
	if d == nil {
		return nil
	}
	return d.liveReload
}
