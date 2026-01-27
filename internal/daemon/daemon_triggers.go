package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync/atomic"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/daemon/events"
	"git.home.luguber.info/inful/docbuilder/internal/forge"
	"git.home.luguber.info/inful/docbuilder/internal/logfields"
)

// TriggerDiscovery manually triggers repository discovery.
func (d *Daemon) TriggerDiscovery() string {
	return d.discoveryRunner.TriggerManual(func() bool { return d.GetStatus() == StatusRunning }, &d.activeJobs)
}

// TriggerBuild manually triggers a site build.
func (d *Daemon) TriggerBuild() string {
	if d.GetStatus() != StatusRunning {
		return ""
	}

	jobID := fmt.Sprintf("build-%d", time.Now().Unix())

	job := &BuildJob{
		ID:        jobID,
		Type:      BuildTypeManual,
		Priority:  PriorityHigh,
		CreatedAt: time.Now(),
		TypedMeta: &BuildJobMetadata{
			V2Config:      d.config,
			StateManager:  d.stateManager,
			LiveReloadHub: d.liveReload,
		},
	}

	if err := d.buildQueue.Enqueue(job); err != nil {
		slog.Error("Failed to enqueue build job", logfields.JobID(jobID), logfields.Error(err))
		return ""
	}

	slog.Info("Manual build triggered", logfields.JobID(jobID))
	return jobID
}

// TriggerWebhookBuild triggers a build for specific repositories from a webhook event.
// This allows targeted rebuilds without refetching all repositories.
func (d *Daemon) TriggerWebhookBuild(repoFullName, branch string, changedFiles []string) string {
	if d.GetStatus() != StatusRunning {
		return ""
	}

	// A webhook build should rebuild the full site with the currently known repository
	// set. The webhook payload only determines whether we trigger, and which repository
	// we annotate as changed.
	//
	// In explicit-repo mode (config.repositories provided) use the configured list.
	// In discovery-only mode, use the most recently discovered repository list.
	var reposForBuild []config.Repository
	if len(d.config.Repositories) > 0 {
		reposForBuild = append([]config.Repository{}, d.config.Repositories...)
	} else {
		discovered, err := d.GetDiscoveryResult()
		if err == nil && discovered != nil && d.discovery != nil {
			reposForBuild = d.discovery.ConvertToConfigRepositories(discovered.Repositories, d.forgeManager)
		}
	}

	// Determine whether the webhook matches any currently known repository.
	matched := false
	matchedRepoURL := ""
	matchedDocsPaths := []string{"docs"}
	for i := range reposForBuild {
		repo := &reposForBuild[i]
		if repo.Name != repoFullName && !matchesRepoURL(repo.URL, repoFullName) {
			continue
		}

		// In explicit-repo mode, honor configured branch filters.
		if len(d.config.Repositories) > 0 {
			if branch != "" && repo.Branch != branch {
				continue
			}
		}

		matched = true
		matchedRepoURL = repo.URL
		if len(repo.Paths) > 0 {
			matchedDocsPaths = repo.Paths
		}
		if branch != "" {
			repo.Branch = branch
		}
		slog.Info("Webhook matched repository",
			"repo", repo.Name,
			"full_name", repoFullName,
			"branch", branch)
	}

	if !matched {
		slog.Warn("No matching repositories found for webhook",
			"repo_full_name", repoFullName,
			"branch", branch)
		return ""
	}

	// If the webhook payload included changed files (push-like event), only trigger
	// a rebuild when at least one change touches the configured docs paths.
	if len(changedFiles) > 0 {
		if !hasDocsRelevantChange(changedFiles, matchedDocsPaths) {
			slog.Info("Webhook push ignored (no docs changes)",
				"repo_full_name", repoFullName,
				"branch", branch,
				"changed_files", len(changedFiles),
				"docs_paths", matchedDocsPaths)
			return ""
		}
	}
	if len(reposForBuild) == 0 {
		slog.Warn("No repositories available for webhook build; falling back to target-only build",
			"repo_full_name", repoFullName,
			"branch", branch)
		// Best-effort: keep previous behavior as a fallback.
		reposForBuild = d.discoveredReposForWebhook(repoFullName, branch)
		if len(reposForBuild) == 0 {
			return ""
		}
		matchedRepoURL = reposForBuild[0].URL
	}

	jobID := ""
	if d.buildDebouncer != nil {
		if planned, ok := d.buildDebouncer.PlannedJobID(); ok {
			jobID = planned
		}
	}
	if jobID == "" {
		jobID = fmt.Sprintf("webhook-%d", time.Now().Unix())
	}
	if d.orchestrationBus != nil {
		_ = d.orchestrationBus.Publish(context.Background(), events.BuildRequested{
			JobID:       jobID,
			Immediate:   true,
			Reason:      "webhook",
			RepoURL:     matchedRepoURL,
			Branch:      branch,
			RequestedAt: time.Now(),
		})
		slog.Info("Webhook build requested",
			logfields.JobID(jobID),
			slog.String("repo", repoFullName),
			slog.String("branch", branch),
			slog.Int("repositories", len(reposForBuild)))
		return jobID
	}

	job := &BuildJob{
		ID:        jobID,
		Type:      BuildTypeWebhook,
		Priority:  PriorityHigh,
		CreatedAt: time.Now(),
		TypedMeta: &BuildJobMetadata{
			V2Config:      d.config,
			Repositories:  reposForBuild,
			StateManager:  d.stateManager,
			LiveReloadHub: d.liveReload,
			DeltaRepoReasons: map[string]string{
				matchedRepoURL: fmt.Sprintf("webhook push to %s", branch),
			},
		},
	}

	if err := d.buildQueue.Enqueue(job); err != nil {
		slog.Error("Failed to enqueue webhook build job", logfields.JobID(jobID), logfields.Error(err))
		return ""
	}

	slog.Info("Webhook build triggered",
		logfields.JobID(jobID),
		slog.String("repo", repoFullName),
		slog.String("branch", branch),
		slog.Int("target_count", 1),
		slog.Int("repositories", len(reposForBuild)))

	atomic.AddInt32(&d.queueLength, 1)
	return jobID
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

func (d *Daemon) discoveredReposForWebhook(repoFullName, branch string) []config.Repository {
	discovered, err := d.GetDiscoveryResult()
	if err != nil || discovered == nil {
		return nil
	}
	if d.discovery == nil {
		return nil
	}

	for _, repo := range discovered.Repositories {
		if repo == nil {
			continue
		}
		if repo.FullName != repoFullName && !matchesRepoURL(repo.CloneURL, repoFullName) && !matchesRepoURL(repo.SSHURL, repoFullName) {
			continue
		}

		converted := d.discovery.ConvertToConfigRepositories([]*forge.Repository{repo}, d.forgeManager)
		for i := range converted {
			if branch != "" {
				converted[i].Branch = branch
			}
		}

		slog.Info("Webhook matched discovered repository",
			"repo", repo.Name,
			"full_name", repoFullName,
			"branch", branch)
		return converted
	}

	return nil
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

// triggerScheduledBuildForExplicitRepos triggers a scheduled build for explicitly configured repositories.
func (d *Daemon) triggerScheduledBuildForExplicitRepos(ctx context.Context) {
	if d.GetStatus() != StatusRunning {
		return
	}
	if ctx == nil {
		return
	}

	jobID := ""
	if d.buildDebouncer != nil {
		if planned, ok := d.buildDebouncer.PlannedJobID(); ok {
			jobID = planned
		}
	}
	if jobID == "" {
		jobID = fmt.Sprintf("scheduled-build-%d", time.Now().Unix())
	}
	if d.orchestrationBus != nil {
		_ = d.orchestrationBus.Publish(ctx, events.BuildRequested{
			JobID:       jobID,
			Reason:      "scheduled build",
			RequestedAt: time.Now(),
		})
		slog.Info("Scheduled build requested",
			logfields.JobID(jobID),
			slog.Int("repositories", len(d.config.Repositories)))
		return
	}

	slog.Info("Triggering scheduled build for explicit repositories",
		logfields.JobID(jobID),
		slog.Int("repositories", len(d.config.Repositories)))

	job := &BuildJob{
		ID:        jobID,
		Type:      BuildTypeScheduled,
		Priority:  PriorityNormal,
		CreatedAt: time.Now(),
		TypedMeta: &BuildJobMetadata{
			V2Config:      d.config,
			Repositories:  d.config.Repositories,
			StateManager:  d.stateManager,
			LiveReloadHub: d.liveReload,
		},
	}

	if err := d.buildQueue.Enqueue(job); err != nil {
		slog.Error("Failed to enqueue scheduled build", logfields.JobID(jobID), logfields.Error(err))
		return
	}

	atomic.AddInt32(&d.queueLength, 1)
}
