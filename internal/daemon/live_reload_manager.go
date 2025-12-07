package daemon

import "git.home.luguber.info/inful/docbuilder/internal/hugo"

// LiveReloadManager handles live reload notifications
type LiveReloadManager interface {
	// BroadcastUpdate sends live reload notification if hub is available
	BroadcastUpdate(report *hugo.BuildReport, job *BuildJob)
}

// LiveReloadManagerImpl implements LiveReloadManager
type LiveReloadManagerImpl struct{}

// NewLiveReloadManager creates a new live reload manager
func NewLiveReloadManager() LiveReloadManager {
	return &LiveReloadManagerImpl{}
}

// BroadcastUpdate sends live reload notification if hub is available
func (lrm *LiveReloadManagerImpl) BroadcastUpdate(report *hugo.BuildReport, job *BuildJob) {
	if report.DocFilesHash == "" {
		return
	}

	EnsureTypedMeta(job)
	hub := job.TypedMeta.LiveReloadHub
	if hub == nil {
		return
	}

	hub.Broadcast(report.DocFilesHash)
}
