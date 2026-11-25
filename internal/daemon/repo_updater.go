package daemon

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/daemon/events"
	"git.home.luguber.info/inful/docbuilder/internal/git"
	"git.home.luguber.info/inful/docbuilder/internal/logfields"
)

type RepoUpdater struct {
	// mu guards the bus: it is written when the daemon starts and read
	// by the goroutine of the updater.
	mu    sync.Mutex
	bus   *events.Bus
	ready chan struct{} // closed once Run has subscribed to events

	remoteChecker RemoteHeadChecker
	cache         *git.RemoteHeadCache

	reposForLookup func() []config.Repository
}

type RemoteHeadChecker interface {
	CheckRemoteChanged(ctx context.Context, cache *git.RemoteHeadCache, repo config.Repository, branch string) (bool, string, error)
}

// SetBus gives the updater the bus of the current run. The bus of the
// last run was closed when the daemon stopped, and a subscription to a
// closed bus returns at once.
func (u *RepoUpdater) SetBus(bus *events.Bus) {
	if u == nil || bus == nil {
		return
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	u.bus = bus
	// The new run needs a new readiness signal: the old one is closed
	// and says nothing about the subscription of this run.
	u.ready = make(chan struct{})
}

// busOf returns the bus of the current run.
func (u *RepoUpdater) busOf() *events.Bus {
	if u == nil {
		return nil
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.bus
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
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.ready
}

func (u *RepoUpdater) Run(ctx context.Context) {
	bus := u.busOf()
	if ctx == nil || u == nil || bus == nil || u.remoteChecker == nil {
		return
	}

	reqCh, unsubscribe := events.Subscribe[events.RepoUpdateRequested](bus, 32)
	defer unsubscribe()

	// The readiness signal is closed under the lock: the daemon starts
	// the updater again with a new one, and two goroutines must not
	// close the same channel.
	u.mu.Lock()
	if u.ready != nil {
		select {
		case <-u.ready:
			// already closed
		default:
			close(u.ready)
		}
	}
	u.mu.Unlock()

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

	changed, sha, err := u.remoteChecker.CheckRemoteChanged(ctx, u.cache, repo, branch)
	if err != nil {
		slog.Warn("Repo update check failed; triggering build",
			logfields.JobID(req.JobID),
			logfields.Name(repo.Name),
			logfields.URL(repo.URL),
			logfields.Error(err))

		if perr := publishOrchestrationEventOnBus(ctx, u.busOf(), events.RepoUpdateFailed{
			JobID:     req.JobID,
			RepoURL:   repo.URL,
			Branch:    branch,
			Error:     err.Error(),
			UpdatedAt: time.Now(),
			Immediate: req.Immediate,
		}); perr != nil {
			slog.Warn("Failed to publish RepoUpdateFailed",
				logfields.JobID(req.JobID),
				logfields.Name(repo.Name),
				logfields.URL(repo.URL),
				logfields.Error(perr))
		}

		if berr := publishOrchestrationEventOnBus(ctx, u.busOf(), events.BuildRequested{
			JobID:       req.JobID,
			Immediate:   req.Immediate,
			Reason:      "repo_update_failed",
			RepoURL:     repo.URL,
			Branch:      branch,
			Snapshot:    nil,
			RequestedAt: time.Now(),
		}); berr != nil {
			slog.Warn("Failed to publish BuildRequested after repo update failure",
				logfields.JobID(req.JobID),
				logfields.Name(repo.Name),
				logfields.URL(repo.URL),
				logfields.Error(berr))
		}
		return
	}

	if err := publishOrchestrationEventOnBus(ctx, u.busOf(), events.RepoUpdated{
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
	if err := publishOrchestrationEventOnBus(ctx, u.busOf(), events.BuildRequested{
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
