package daemon

import (
	"context"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/daemon/events"
	"git.home.luguber.info/inful/docbuilder/internal/forge"
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

	evtBranch := normalizeGitBranchRef(evt.Branch)

	repos := d.currentReposForOrchestratedBuild()

	forgeHost := ""
	if evt.ForgeName != "" && d.forgeManager != nil {
		if cfg := d.forgeManager.GetForgeConfigs()[evt.ForgeName]; cfg != nil {
			forgeHost = extractHost(cfg.BaseURL)
		}
	}

	matchedRepoURL, matchedBranch, matchedDocsPaths := d.matchWebhookRepo(evt, evtBranch, forgeHost, repos)
	if matchedRepoURL == "" {
		matchedRepoURL, matchedBranch, matchedDocsPaths = d.resolveWebhookRepoFromForge(ctx, evt)
	}

	if matchedRepoURL == "" {
		if d.handleWebhookWithNoRepos(ctx, evt, evtBranch, repos) {
			return
		}
		slog.Warn("Webhook did not match any known repository",
			logfields.JobID(evt.JobID),
			slog.String("forge", evt.ForgeName),
			slog.String("repo", evt.RepoFullName),
			slog.String("branch", evtBranch))
		return
	}

	// DocBuilder builds are always full-site builds on the repo's configured/default
	// branch. Ignore push events for other branches to avoid fetching refs we don't
	// care about (and to prevent errors when feature branches are deleted).
	if matchedBranch != "" && evtBranch != "" && evtBranch != matchedBranch {
		slog.Info("Webhook push ignored (non-default branch)",
			logfields.JobID(evt.JobID),
			slog.String("forge", evt.ForgeName),
			slog.String("repo", evt.RepoFullName),
			slog.String("branch", evtBranch),
			slog.String("default_branch", matchedBranch))
		return
	}

	if len(evt.ChangedFiles) > 0 {
		if !hasDocsRelevantChange(evt.ChangedFiles, matchedDocsPaths) {
			slog.Info("Webhook push ignored (no docs changes)",
				logfields.JobID(evt.JobID),
				slog.String("forge", evt.ForgeName),
				slog.String("repo", evt.RepoFullName),
				slog.String("branch", evtBranch),
				slog.Int("changed_files", len(evt.ChangedFiles)),
				slog.Any("docs_paths", matchedDocsPaths))
			return
		}
	}

	immediate := true
	if d.config != nil && d.config.Daemon != nil && d.config.Daemon.BuildDebounce != nil && d.config.Daemon.BuildDebounce.WebhookImmediate != nil {
		immediate = *d.config.Daemon.BuildDebounce.WebhookImmediate
	}

	if err := d.publishOrchestrationEvent(ctx, events.RepoUpdateRequested{
		JobID:       evt.JobID,
		Immediate:   immediate,
		RepoURL:     matchedRepoURL,
		Branch:      strings.TrimSpace(firstNonEmpty(matchedBranch, evtBranch)),
		RequestedAt: time.Now(),
	}); err != nil {
		slog.Warn("Failed to publish repo update request",
			logfields.JobID(evt.JobID),
			slog.String("repo_url", matchedRepoURL),
			logfields.Error(err))
	}
}

func (d *Daemon) resolveWebhookRepoFromForge(ctx context.Context, evt events.WebhookReceived) (string, string, []string) {
	if ctx == nil || d == nil || d.forgeManager == nil || d.discovery == nil {
		return "", "", nil
	}
	if strings.TrimSpace(evt.ForgeName) == "" || strings.TrimSpace(evt.RepoFullName) == "" {
		return "", "", nil
	}
	if !hasDocsRelevantChange(evt.ChangedFiles, nil) {
		return "", "", nil
	}

	client := d.forgeManager.GetForge(evt.ForgeName)
	if client == nil {
		return "", "", nil
	}

	owner, repoName, ok := splitWebhookRepoFullName(evt.RepoFullName)
	if !ok {
		return "", "", nil
	}

	repo, err := client.GetRepository(ctx, owner, repoName)
	if err != nil {
		slog.Warn("Failed to fetch repository after unmatched webhook",
			logfields.JobID(evt.JobID),
			slog.String("forge", evt.ForgeName),
			slog.String("repo", evt.RepoFullName),
			logfields.Error(err))
		return "", "", nil
	}
	if repo == nil {
		return "", "", nil
	}
	if repo.Metadata == nil {
		repo.Metadata = make(map[string]string)
	}
	if repo.Metadata["forge_name"] == "" {
		repo.Metadata["forge_name"] = evt.ForgeName
	}

	if err := client.CheckDocumentation(ctx, repo); err != nil {
		slog.Warn("Failed to check repository documentation after unmatched webhook",
			logfields.JobID(evt.JobID),
			slog.String("forge", evt.ForgeName),
			slog.String("repo", evt.RepoFullName),
			logfields.Error(err))
		return "", "", nil
	}

	include, reason := d.discovery.ShouldIncludeRepository(repo)
	if !include {
		slog.Info("Webhook repo remains excluded after direct probe",
			logfields.JobID(evt.JobID),
			slog.String("forge", evt.ForgeName),
			slog.String("repo", evt.RepoFullName),
			slog.String("reason", reason))
		return "", "", nil
	}

	d.upsertWebhookDiscoveredRepo(repo)

	converted := d.discovery.ConvertToConfigRepositories([]*forge.Repository{repo}, d.forgeManager)
	if len(converted) == 0 {
		return "", normalizeGitBranchRef(repo.DefaultBranch), []string{"docs"}
	}

	cfgRepo := converted[0]
	docsPaths := cfgRepo.Paths
	if len(docsPaths) == 0 {
		docsPaths = []string{"docs"}
	}

	slog.Info("Webhook matched repository after direct probe",
		logfields.JobID(evt.JobID),
		slog.String("forge", evt.ForgeName),
		slog.String("repo", evt.RepoFullName),
		slog.String("repo_url", cfgRepo.URL))

	return cfgRepo.URL, normalizeGitBranchRef(cfgRepo.Branch), docsPaths
}

func (d *Daemon) upsertWebhookDiscoveredRepo(repo *forge.Repository) {
	if d == nil || d.discoveryCache == nil || repo == nil {
		return
	}

	current, _ := d.discoveryCache.Get()
	next := &forge.DiscoveryResult{
		Timestamp: time.Now(),
	}
	if current != nil {
		*next = *current
		next.Repositories = append([]*forge.Repository(nil), current.Repositories...)
		next.Filtered = append([]*forge.Repository(nil), current.Filtered...)
	}

	repoCopy := *repo
	next.Repositories = upsertWebhookRepoEntry(next.Repositories, &repoCopy)
	next.Filtered = removeWebhookRepoEntry(next.Filtered, repo)
	d.discoveryCache.Update(next)
}

func upsertWebhookRepoEntry(repos []*forge.Repository, target *forge.Repository) []*forge.Repository {
	for i := range repos {
		if sameWebhookRepo(repos[i], target) {
			repos[i] = target
			return repos
		}
	}
	return append(repos, target)
}

func removeWebhookRepoEntry(repos []*forge.Repository, target *forge.Repository) []*forge.Repository {
	filtered := repos[:0]
	for _, repo := range repos {
		if sameWebhookRepo(repo, target) {
			continue
		}
		filtered = append(filtered, repo)
	}
	return filtered
}

func sameWebhookRepo(left, right *forge.Repository) bool {
	if left == nil || right == nil {
		return false
	}
	if left.CloneURL != "" && right.CloneURL != "" && left.CloneURL == right.CloneURL {
		return true
	}
	return left.FullName != "" && left.FullName == right.FullName
}

func splitWebhookRepoFullName(fullName string) (string, string, bool) {
	fullName = strings.TrimSpace(fullName)
	if fullName == "" {
		return "", "", false
	}
	lastSlash := strings.LastIndex(fullName, "/")
	if lastSlash <= 0 || lastSlash == len(fullName)-1 {
		return "", "", false
	}
	return fullName[:lastSlash], fullName[lastSlash+1:], true
}

func (d *Daemon) handleWebhookWithNoRepos(ctx context.Context, evt events.WebhookReceived, evtBranch string, repos []config.Repository) bool {
	if len(repos) != 0 {
		return false
	}

	isForgeMode := d.config != nil && len(d.config.Repositories) == 0
	if !isForgeMode {
		slog.Warn("Webhook received but no repositories available",
			logfields.JobID(evt.JobID),
			slog.String("forge", evt.ForgeName),
			slog.String("repo", evt.RepoFullName),
			slog.String("branch", evtBranch))
		return true
	}

	// In forge mode, repository lists come from discovery. If discovery hasn't completed
	// yet, retry once it has populated the cache.
	if d.discoveryRunner != nil {
		go d.discoveryRunner.SafeRun(ctx, func() bool { return d.GetStatus() == StatusRunning })
	}

	slog.Warn("Webhook received but no repositories available; will retry after discovery",
		logfields.JobID(evt.JobID),
		slog.String("forge", evt.ForgeName),
		slog.String("repo", evt.RepoFullName),
		slog.String("branch", evtBranch))
	go d.retryWebhookAfterDiscovery(ctx, events.WebhookReceived{
		JobID:        evt.JobID,
		ForgeName:    evt.ForgeName,
		RepoFullName: evt.RepoFullName,
		Branch:       evtBranch,
		ChangedFiles: append([]string(nil), evt.ChangedFiles...),
		ReceivedAt:   evt.ReceivedAt,
	})
	return true
}

func (d *Daemon) matchWebhookRepo(evt events.WebhookReceived, evtBranch string, forgeHost string, repos []config.Repository) (string, string, []string) {
	matchedRepoURL := ""
	matchedDocsPaths := []string{"docs"}
	matchedBranch := ""

	for i := range repos {
		repo := &repos[i]
		repoBranch := normalizeGitBranchRef(repo.Branch)

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
			if evtBranch != "" && repoBranch != evtBranch {
				continue
			}
		}

		matchedRepoURL = repo.URL
		matchedBranch = repoBranch
		if len(repo.Paths) > 0 {
			matchedDocsPaths = repo.Paths
		}
		break
	}

	return matchedRepoURL, matchedBranch, matchedDocsPaths
}

func (d *Daemon) retryWebhookAfterDiscovery(ctx context.Context, evt events.WebhookReceived) {
	if d == nil || d.orchestrationBus == nil {
		return
	}
	// Wait for discovery to populate the repo list, then re-publish the webhook event.
	// This avoids dropping webhooks during startup when discovery hasn't completed.
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	timeout := time.NewTimer(2 * time.Minute)
	defer timeout.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-timeout.C:
			slog.Warn("Webhook retry timed out waiting for repositories",
				logfields.JobID(evt.JobID),
				slog.String("forge", evt.ForgeName),
				slog.String("repo", evt.RepoFullName))
			return
		case <-ticker.C:
			repos := d.currentReposForOrchestratedBuild()
			if len(repos) == 0 {
				continue
			}
			slog.Info("Retrying webhook after discovery",
				logfields.JobID(evt.JobID),
				slog.String("forge", evt.ForgeName),
				slog.String("repo", evt.RepoFullName),
				slog.String("branch", evt.Branch))
			_ = d.publishOrchestrationEvent(ctx, evt)
			return
		}
	}
}

func normalizeGitBranchRef(branch string) string {
	branch = strings.TrimSpace(branch)
	branch = strings.TrimPrefix(branch, "refs/heads/")
	branch = strings.TrimPrefix(branch, "refs/tags/")
	return strings.TrimSpace(branch)
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
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
	if _, after, ok := strings.Cut(repoURL, "@"); ok {
		afterAt := after
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
	// If the webhook payload does not include a file list, we conservatively assume
	// the change may affect docs so webhook processing is not accidentally skipped.
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
		// .docignore in the repository root controls whether the repository is
		// included at all during discovery, so any change to it must trigger a
		// rebuild even if no docs path changed.
		if f == ".docignore" {
			return true
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
