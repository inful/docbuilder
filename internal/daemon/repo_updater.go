package daemon

import (
	"context"
	"log/slog"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/daemon/events"
	"git.home.luguber.info/inful/docbuilder/internal/git"
	"git.home.luguber.info/inful/docbuilder/internal/logfields"
)

type RepoUpdater struct {
	bus   *events.Bus
	ready chan struct{} // closed once Run has subscribed to events

	remoteChecker RemoteHeadChecker
	cache         *git.RemoteHeadCache

	reposForLookup func() []config.Repository
}

type RemoteHeadChecker interface {
	CheckRemoteChanged(cache *git.RemoteHeadCache, repo config.Repository, branch string) (bool, string, error)
}

func NewRepoUpdater(bus *events.Bus, checker RemoteHeadChecker, cache *git.RemoteHeadCache, reposForLookup func() []config.Repository) *RepoUpdater {
	return &RepoUpdater{bus: bus, ready: make(chan struct{}), remoteChecker: checker, cache: cache, reposForLookup: reposForLookup}
}

// Ready is closed once Run has subscribed to RepoUpdateRequested events.
//
// This is primarily intended for tests and deterministic startup sequencing.
// Note: Ready() does not indicate that any particular update has been processed.
func (u *RepoUpdater) Ready() <-chan struct{} {
	if u == nil {
		return nil
	}
	return u.ready
}

func (u *RepoUpdater) Run(ctx context.Context) {
	if ctx == nil || u == nil || u.bus == nil || u.remoteChecker == nil {
		return
	}

	reqCh, unsubscribe := events.Subscribe[events.RepoUpdateRequested](u.bus, 32)
	defer unsubscribe()
	if u.ready != nil {
		select {
		case <-u.ready:
			// already closed
		default:
			close(u.ready)
		}
	}

	for {
		select {
		case <-ctx.Done():
			return
		case req, ok := <-reqCh:
			if !ok {
				return
			}
			u.handleRequest(ctx, req)
		}
	}
}

func (u *RepoUpdater) handleRequest(ctx context.Context, req events.RepoUpdateRequested) {
	repo, ok := u.lookupRepo(req.RepoURL)
	if !ok {
		slog.Warn("Repo update requested for unknown repo",
			logfields.JobID(req.JobID),
			logfields.URL(req.RepoURL))
		return
	}

	branch := req.Branch
	if branch == "" {
		branch = repo.Branch
	}

	changed, sha, err := u.remoteChecker.CheckRemoteChanged(u.cache, repo, branch)
	if err != nil {
		slog.Warn("Repo update check failed; assuming changed",
			logfields.JobID(req.JobID),
			logfields.Name(repo.Name),
			logfields.URL(repo.URL),
			logfields.Error(err))
		changed = true
	}

	if err := publishOrchestrationEventOnBus(ctx, u.bus, events.RepoUpdated{
		JobID:     req.JobID,
		RepoURL:   repo.URL,
		Branch:    branch,
		CommitSHA: sha,
		Changed:   changed,
		UpdatedAt: time.Now(),
		Immediate: req.Immediate,
	}); err != nil {
		slog.Warn("Failed to publish RepoUpdated",
			logfields.JobID(req.JobID),
			logfields.Name(repo.Name),
			logfields.URL(repo.URL),
			logfields.Error(err))
	}

	if !changed {
		slog.Info("Repo unchanged; skipping build request",
			logfields.JobID(req.JobID),
			logfields.Name(repo.Name),
			slog.String("branch", branch))
		return
	}

	snapshot := map[string]string{}
	if sha != "" {
		snapshot[repo.URL] = sha
	}
	if err := publishOrchestrationEventOnBus(ctx, u.bus, events.BuildRequested{
		JobID:       req.JobID,
		Immediate:   req.Immediate,
		Reason:      "webhook",
		RepoURL:     repo.URL,
		Branch:      branch,
		Snapshot:    snapshot,
		RequestedAt: time.Now(),
	}); err != nil {
		slog.Warn("Failed to publish BuildRequested",
			logfields.JobID(req.JobID),
			logfields.Name(repo.Name),
			logfields.URL(repo.URL),
			logfields.Error(err))
	}
}

func (u *RepoUpdater) lookupRepo(repoURL string) (config.Repository, bool) {
	if u.reposForLookup == nil {
		return config.Repository{}, false
	}
	repos := u.reposForLookup()
	for i := range repos {
		if repos[i].URL == repoURL {
			return repos[i], true
		}
	}
	return config.Repository{}, false
}
