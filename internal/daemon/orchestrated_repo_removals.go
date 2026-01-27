package daemon

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"git.home.luguber.info/inful/docbuilder/internal/daemon/events"
	"git.home.luguber.info/inful/docbuilder/internal/logfields"
)

func (d *Daemon) runRepoRemovedConsumer(ctx context.Context) {
	if ctx == nil || d == nil || d.orchestrationBus == nil {
		return
	}

	repoRemovedCh, unsubscribe := events.Subscribe[events.RepoRemoved](d.orchestrationBus, 16)
	defer unsubscribe()

	for {
		select {
		case <-ctx.Done():
			return
		case evt, ok := <-repoRemovedCh:
			if !ok {
				return
			}
			d.handleRepoRemoved(evt)
		}
	}
}

func (d *Daemon) handleRepoRemoved(evt events.RepoRemoved) {
	if d == nil {
		return
	}
	if evt.RepoURL == "" {
		return
	}

	if remover, ok := any(d.stateManager).(interface{ RemoveRepositoryState(string) }); ok {
		remover.RemoveRepositoryState(evt.RepoURL)
		slog.Info("Repository removed from state", slog.String("repo_url", evt.RepoURL), logfields.Name(evt.RepoName))
	}

	// Best-effort: prune any cached remote-head entries for the removed repository.
	if d.repoUpdater != nil && d.repoUpdater.cache != nil {
		d.repoUpdater.cache.DeleteByURL(evt.RepoURL)
		if err := d.repoUpdater.cache.Save(); err != nil {
			slog.Warn("Failed to persist remote HEAD cache after repo removal",
				slog.String("repo_url", evt.RepoURL),
				logfields.Error(err))
		}
	}

	// Best-effort: remove cached clone directory for the removed repository.
	if d.config == nil || d.config.Daemon == nil {
		return
	}
	repoCacheDir := strings.TrimSpace(d.config.Daemon.Storage.RepoCacheDir)
	if repoCacheDir == "" || strings.TrimSpace(evt.RepoName) == "" {
		return
	}

	base := filepath.Clean(repoCacheDir)
	target := filepath.Clean(filepath.Join(base, evt.RepoName))
	if !strings.HasPrefix(target, base+string(os.PathSeparator)) {
		slog.Warn("Skipping repo cache deletion: path escapes repo cache dir",
			slog.String("repo_url", evt.RepoURL),
			logfields.Name(evt.RepoName),
			slog.String("repo_cache_dir", base),
			slog.String("target", target))
		return
	}
	if err := os.RemoveAll(target); err != nil {
		slog.Warn("Failed to remove repo cache directory",
			slog.String("repo_url", evt.RepoURL),
			logfields.Name(evt.RepoName),
			logfields.Path(target),
			logfields.Error(err))
		return
	}
	slog.Info("Repository cache directory removed",
		slog.String("repo_url", evt.RepoURL),
		logfields.Name(evt.RepoName),
		logfields.Path(target))
}
