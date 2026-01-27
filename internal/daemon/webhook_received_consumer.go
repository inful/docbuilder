package daemon

import (
	"context"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/daemon/events"
	"git.home.luguber.info/inful/docbuilder/internal/logfields"
)

func (d *Daemon) runWebhookReceivedConsumer(ctx context.Context) {
	if ctx == nil || d == nil || d.orchestrationBus == nil {
		return
	}

	ch, unsubscribe := events.Subscribe[events.WebhookReceived](d.orchestrationBus, 32)
	defer unsubscribe()

	for {
		select {
		case <-ctx.Done():
			return
		case evt, ok := <-ch:
			if !ok {
				return
			}
			d.handleWebhookReceived(ctx, evt)
		}
	}
}

func (d *Daemon) handleWebhookReceived(ctx context.Context, evt events.WebhookReceived) {
	if ctx == nil || d == nil || d.GetStatus() != StatusRunning || d.orchestrationBus == nil {
		return
	}

	repos := d.currentReposForOrchestratedBuild()
	if len(repos) == 0 {
		slog.Warn("Webhook received but no repositories available",
			logfields.JobID(evt.JobID),
			slog.String("forge", evt.ForgeName),
			slog.String("repo", evt.RepoFullName),
			slog.String("branch", evt.Branch))
		return
	}

	forgeHost := ""
	if evt.ForgeName != "" && d.forgeManager != nil {
		if cfg := d.forgeManager.GetForgeConfigs()[evt.ForgeName]; cfg != nil {
			forgeHost = extractHost(cfg.BaseURL)
		}
	}

	matchedRepoURL := ""
	matchedDocsPaths := []string{"docs"}
	for i := range repos {
		repo := &repos[i]

		if forgeHost != "" {
			repoHost := extractRepoHost(repo.URL)
			if repoHost == "" || repoHost != forgeHost {
				continue
			}
		}

		if !repoMatchesFullName(*repo, evt.RepoFullName) {
			continue
		}

		// In explicit-repo mode, honor configured branch filters.
		if d.config != nil && len(d.config.Repositories) > 0 {
			if evt.Branch != "" && repo.Branch != evt.Branch {
				continue
			}
		}

		matchedRepoURL = repo.URL
		if len(repo.Paths) > 0 {
			matchedDocsPaths = repo.Paths
		}
		break
	}

	if matchedRepoURL == "" {
		slog.Warn("Webhook did not match any known repository",
			logfields.JobID(evt.JobID),
			slog.String("forge", evt.ForgeName),
			slog.String("repo", evt.RepoFullName),
			slog.String("branch", evt.Branch))
		return
	}

	if len(evt.ChangedFiles) > 0 {
		if !hasDocsRelevantChange(evt.ChangedFiles, matchedDocsPaths) {
			slog.Info("Webhook push ignored (no docs changes)",
				logfields.JobID(evt.JobID),
				slog.String("forge", evt.ForgeName),
				slog.String("repo", evt.RepoFullName),
				slog.String("branch", evt.Branch),
				slog.Int("changed_files", len(evt.ChangedFiles)),
				slog.Any("docs_paths", matchedDocsPaths))
			return
		}
	}

	immediate := true
	if d.config != nil && d.config.Daemon != nil && d.config.Daemon.BuildDebounce != nil && d.config.Daemon.BuildDebounce.WebhookImmediate != nil {
		immediate = *d.config.Daemon.BuildDebounce.WebhookImmediate
	}

	_ = d.orchestrationBus.Publish(ctx, events.RepoUpdateRequested{
		JobID:       evt.JobID,
		Immediate:   immediate,
		RepoURL:     matchedRepoURL,
		Branch:      evt.Branch,
		RequestedAt: time.Now(),
	})
}

func repoMatchesFullName(repo config.Repository, fullName string) bool {
	if strings.TrimSpace(fullName) == "" {
		return false
	}

	if repo.Name == fullName {
		return true
	}
	if repo.Tags != nil {
		if repo.Tags["full_name"] == fullName {
			return true
		}
	}
	return matchesRepoURL(repo.URL, fullName)
}

func extractHost(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err == nil {
		if h := strings.ToLower(parsed.Hostname()); h != "" {
			return h
		}
	}

	// Best-effort fallback for host-only inputs.
	raw = strings.TrimPrefix(raw, "https://")
	raw = strings.TrimPrefix(raw, "http://")
	raw = strings.TrimSuffix(raw, "/")
	if raw == "" {
		return ""
	}
	if strings.Contains(raw, "/") {
		raw = strings.SplitN(raw, "/", 2)[0]
	}
	if strings.Contains(raw, ":") {
		raw = strings.SplitN(raw, ":", 2)[0]
	}
	return strings.ToLower(raw)
}

func extractRepoHost(repoURL string) string {
	repoURL = strings.TrimSpace(repoURL)
	if repoURL == "" {
		return ""
	}

	if strings.Contains(repoURL, "://") {
		u, err := url.Parse(repoURL)
		if err == nil {
			if h := strings.ToLower(u.Hostname()); h != "" {
				return h
			}
		}
	}

	// ssh scp-like: git@host:owner/repo.git
	if at := strings.Index(repoURL, "@"); at >= 0 {
		afterAt := repoURL[at+1:]
		hostPart := afterAt
		if strings.Contains(hostPart, ":") {
			hostPart = strings.SplitN(hostPart, ":", 2)[0]
		}
		if strings.Contains(hostPart, "/") {
			hostPart = strings.SplitN(hostPart, "/", 2)[0]
		}
		return strings.ToLower(strings.TrimSpace(hostPart))
	}

	return ""
}

func hasDocsRelevantChange(changedFiles []string, docsPaths []string) bool {
	if len(changedFiles) == 0 {
		return true
	}
	if len(docsPaths) == 0 {
		docsPaths = []string{"docs"}
	}

	normalize := func(p string) string {
		p = strings.TrimSpace(p)
		p = strings.TrimPrefix(p, "./")
		p = strings.TrimPrefix(p, "/")
		p = strings.TrimSuffix(p, "/")
		return p
	}

	nDocs := make([]string, 0, len(docsPaths))
	for _, dp := range docsPaths {
		dp = normalize(dp)
		if dp == "" {
			continue
		}
		nDocs = append(nDocs, dp)
	}
	if len(nDocs) == 0 {
		nDocs = []string{"docs"}
	}

	for _, f := range changedFiles {
		f = normalize(f)
		if f == "" {
			continue
		}
		for _, dp := range nDocs {
			if f == dp || strings.HasPrefix(f, dp+"/") {
				return true
			}
		}
	}

	return false
}

// matchesRepoURL checks if a repository URL matches the given full name (owner/repo).
func matchesRepoURL(repoURL, fullName string) bool {
	// Extract owner/repo from various URL formats:
	// - https://github.com/owner/repo.git
	// - git@github.com:owner/repo.git
	// - https://github.com/owner/repo
	// - git@github.com:owner/repo

	// Remove trailing .git if present
	url := repoURL
	if len(url) > 4 && url[len(url)-4:] == ".git" {
		url = url[:len(url)-4]
	}

	// Check if URL ends with the full name
	if len(url) > len(fullName) {
		// Check for /owner/repo or :owner/repo
		if url[len(url)-len(fullName)-1] == '/' || url[len(url)-len(fullName)-1] == ':' {
			if url[len(url)-len(fullName):] == fullName {
				return true
			}
		}
	}

	return false
}
