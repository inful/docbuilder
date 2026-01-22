package daemon

import (
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
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
func (d *Daemon) TriggerWebhookBuild(repoFullName, branch string) string {
	if d.GetStatus() != StatusRunning {
		return ""
	}

	// Find matching repository in config
	var targetRepos []config.Repository
	for i := range d.config.Repositories {
		repo := &d.config.Repositories[i]
		// Match by name or full name extracted from URL
		// GitHub URL format: https://github.com/owner/repo.git or git@github.com:owner/repo.git
		// GitLab URL format: https://gitlab.com/owner/repo.git or git@gitlab.com:owner/repo.git
		// Forgejo URL format: https://git.home.luguber.info/owner/repo.git or git@git.home.luguber.info:owner/repo.git
		if repo.Name == repoFullName || matchesRepoURL(repo.URL, repoFullName) {
			// If branch is specified, only rebuild if it matches the configured branch
			if branch == "" || repo.Branch == branch {
				targetRepos = append(targetRepos, *repo)
				slog.Info("Webhook matched repository",
					"repo", repo.Name,
					"full_name", repoFullName,
					"branch", branch)
			}
		}
	}

	if len(targetRepos) == 0 {
		slog.Warn("No matching repositories found for webhook",
			"repo_full_name", repoFullName,
			"branch", branch)
		return ""
	}

	jobID := fmt.Sprintf("webhook-%d", time.Now().Unix())

	job := &BuildJob{
		ID:        jobID,
		Type:      BuildTypeWebhook,
		Priority:  PriorityHigh,
		CreatedAt: time.Now(),
		TypedMeta: &BuildJobMetadata{
			V2Config:      d.config,
			Repositories:  targetRepos,
			StateManager:  d.stateManager,
			LiveReloadHub: d.liveReload,
			DeltaRepoReasons: map[string]string{
				repoFullName: fmt.Sprintf("webhook push to %s", branch),
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
		slog.Int("target_count", len(targetRepos)))

	atomic.AddInt32(&d.queueLength, 1)
	return jobID
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
func (d *Daemon) triggerScheduledBuildForExplicitRepos() {
	if d.GetStatus() != StatusRunning {
		return
	}

	jobID := fmt.Sprintf("scheduled-build-%d", time.Now().Unix())

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
