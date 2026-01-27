package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/daemon/events"
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
	if d.orchestrationBus == nil {
		return ""
	}

	jobID := ""
	if d.buildDebouncer != nil {
		if planned, ok := d.buildDebouncer.PlannedJobID(); ok {
			jobID = planned
		}
	}
	if jobID == "" {
		jobID = fmt.Sprintf("manual-%d", time.Now().UnixNano())
	}
	_ = d.orchestrationBus.Publish(context.Background(), events.BuildRequested{
		JobID:       jobID,
		Immediate:   true,
		Reason:      "manual",
		RequestedAt: time.Now(),
	})

	slog.Info("Manual build requested", logfields.JobID(jobID))
	return jobID
}

// TriggerWebhookBuild processes a webhook event and requests an orchestrated build.
//
// The webhook payload is used to decide whether a build should be requested and which
// repository should be treated as "changed", but it does not narrow the site scope:
// the build remains a canonical full-site build.

func (d *Daemon) TriggerWebhookBuild(forgeName, repoFullName, branch string, changedFiles []string) string {
	if d.GetStatus() != StatusRunning {
		return ""
	}
	if d.orchestrationBus == nil {
		return ""
	}
	jobID := ""
	if d.buildDebouncer != nil {
		if planned, ok := d.buildDebouncer.PlannedJobID(); ok {
			jobID = planned
		}
	}
	if jobID == "" {
		jobID = fmt.Sprintf("webhook-%d", time.Now().UnixNano())
	}

	filesCopy := append([]string(nil), changedFiles...)
	_ = d.orchestrationBus.Publish(context.Background(), events.WebhookReceived{
		JobID:        jobID,
		ForgeName:    forgeName,
		RepoFullName: repoFullName,
		Branch:       branch,
		ChangedFiles: filesCopy,
		ReceivedAt:   time.Now(),
	})

	slog.Info("Webhook received",
		logfields.JobID(jobID),
		slog.String("forge", forgeName),
		slog.String("repo", repoFullName),
		slog.String("branch", branch),
		slog.Int("changed_files", len(filesCopy)))
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
	if d.orchestrationBus == nil {
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
	_ = d.orchestrationBus.Publish(ctx, events.BuildRequested{
		JobID:       jobID,
		Reason:      "scheduled build",
		RequestedAt: time.Now(),
	})
	slog.Info("Scheduled build requested",
		logfields.JobID(jobID),
		slog.Int("repositories", len(d.config.Repositories)))
}
