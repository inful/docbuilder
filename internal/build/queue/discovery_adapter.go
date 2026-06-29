package queue

import (
	"context"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// DiscoveryEnqueuerAdapter adapts BuildQueue to the discovery runner's
// BuildEnqueuer port. It owns the BuildJob literal construction that used
// to live in internal/forge/discoveryrunner, which keeps the runner out
// of the queue package's struct shape.
//
// The jobID is supplied by the runner (via its Config.NewJobID callback)
// so the same identifier can be used across log lines and metrics. now
// is optional and defaults to time.Now.
type DiscoveryEnqueuerAdapter struct {
	queue      *BuildQueue
	liveReload LiveReloadHub
	now        func() time.Time
}

// NewDiscoveryEnqueuerAdapter constructs an adapter. now is optional;
// when nil, time.Now is used.
func NewDiscoveryEnqueuerAdapter(q *BuildQueue, liveReload LiveReloadHub, now func() time.Time) *DiscoveryEnqueuerAdapter {
	if now == nil {
		now = time.Now
	}
	return &DiscoveryEnqueuerAdapter{
		queue:      q,
		liveReload: liveReload,
		now:        now,
	}
}

// EnqueueDiscoveryBuild schedules a build for the given repositories. It is
// the implementation of discoveryrunner.BuildEnqueuer.
func (a *DiscoveryEnqueuerAdapter) EnqueueDiscoveryBuild(ctx context.Context, jobID string, repos []config.Repository, cfg *config.Config) error {
	job := &BuildJob{
		ID:        jobID,
		Type:      BuildTypeDiscovery,
		Priority:  PriorityNormal,
		CreatedAt: a.now(),
		TypedMeta: &BuildJobMetadata{
			V2Config:      cfg,
			Repositories:  repos,
			LiveReloadHub: a.liveReload,
		},
	}
	return a.queue.Enqueue(job)
}
