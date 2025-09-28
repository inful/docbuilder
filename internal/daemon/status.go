package daemon

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/hugo"
	"git.home.luguber.info/inful/docbuilder/internal/versioning"
)

// StatusPageData represents data for status page rendering
type StatusPageData struct {
	DaemonInfo     DaemonInfo         `json:"daemon_info"`
	Repositories   []RepositoryStatus `json:"repositories"`
	VersionSummary VersionSummary     `json:"version_summary"`
	BuildStatus    BuildStatusInfo    `json:"build_status"`
	SystemMetrics  SystemMetrics      `json:"system_metrics"`
	LastUpdated    time.Time          `json:"last_updated"`
	LastDiscovery  *time.Time         `json:"last_discovery,omitempty"`
	DiscoveryError *string            `json:"discovery_error,omitempty"`
	// DiscoveryErrors contains per-forge discovery errors (forge name -> error string)
	DiscoveryErrors map[string]string `json:"discovery_errors,omitempty"`
}

// DaemonInfo holds basic daemon information
type DaemonInfo struct {
	Status     DaemonStatus `json:"status"`
	Version    string       `json:"version"`
	StartTime  time.Time    `json:"start_time"`
	Uptime     string       `json:"uptime"`
	ConfigFile string       `json:"config_file"`
}

// RepositoryStatus tracks status of individual repositories
type RepositoryStatus struct {
	Name              string               `json:"name"`
	URL               string               `json:"url"`
	LastSync          *time.Time           `json:"last_sync"`
	LastBuild         *time.Time           `json:"last_build"`
	Status            string               `json:"status"` // "healthy", "error", "syncing", "building"
	VersionCount      int                  `json:"version_count"`
	DefaultVersion    string               `json:"default_version"`
	AvailableVersions []versioning.Version `json:"available_versions"`
	LastError         *string              `json:"last_error,omitempty"`
}

// VersionSummary provides overview of versioning across all repositories
type VersionSummary struct {
	TotalRepositories int                            `json:"total_repositories"`
	TotalVersions     int                            `json:"total_versions"`
	StrategyBreakdown map[string]int                 `json:"strategy_breakdown"`
	VersionTypes      map[versioning.VersionType]int `json:"version_types"`
}

// BuildStatusInfo tracks build queue and execution status
type BuildStatusInfo struct {
	QueueLength         int32                     `json:"queue_length"`
	ActiveJobs          int32                     `json:"active_jobs"`
	CompletedBuilds     int64                     `json:"completed_builds"`
	FailedBuilds        int64                     `json:"failed_builds"`
	LastBuildTime       *time.Time                `json:"last_build_time"`
	AverageBuildTime    string                    `json:"average_build_time"`
	LastBuildStages     map[string]string         `json:"last_build_stages,omitempty"` // stage -> duration
	LastBuildOutcome    string                    `json:"last_build_outcome,omitempty"`
	LastBuildSummary    string                    `json:"last_build_summary,omitempty"`
	LastBuildErrors     []string                  `json:"last_build_errors,omitempty"`
	LastBuildWarnings   []string                  `json:"last_build_warnings,omitempty"`
	RenderedPages       *int                      `json:"rendered_pages,omitempty"`
	ClonedRepositories  *int                      `json:"cloned_repositories,omitempty"`
	FailedRepositories  *int                      `json:"failed_repositories,omitempty"`
	SkippedRepositories *int                      `json:"skipped_repositories,omitempty"`
	StaticRendered      *bool                     `json:"static_rendered,omitempty"`
	StageCounts         map[string]map[string]int `json:"stage_counts,omitempty"` // stage -> {success,warning,fatal,canceled}
}

// SystemMetrics provides system resource information
type SystemMetrics struct {
	MemoryUsage    string `json:"memory_usage"`
	DiskUsage      string `json:"disk_usage"`
	GoroutineCount int    `json:"goroutine_count"`
	WorkspaceSize  string `json:"workspace_size"`
}

// GenerateStatusData collects and formats status information
func (d *Daemon) GenerateStatusData() (*StatusPageData, error) {
	slog.Debug("Status: acquiring read lock")
	d.mu.RLock()
	slog.Debug("Status: read lock acquired")
	defer func() {
		d.mu.RUnlock()
		slog.Debug("Status: read lock released")
	}()

	status := &StatusPageData{
		LastUpdated: time.Now(),
	}

	slog.Debug("Status: building daemon info")
	status.DaemonInfo = DaemonInfo{
		Status:     d.status.Load().(DaemonStatus),
		Version:    "2.0.0", // TODO: Get from build info
		StartTime:  d.startTime,
		Uptime:     time.Since(d.startTime).String(),
		ConfigFile: "config.yaml", // TODO: Get from actual config path
	}

	slog.Debug("Status: collecting build status")
	status.BuildStatus = BuildStatusInfo{
		QueueLength:   d.queueLength,
		ActiveJobs:    d.activeJobs,
		LastBuildTime: d.lastBuild,
		// TODO: Add more metrics from build queue
	}

	// Extract most recent build stage timings (best-effort)
	if d.buildQueue != nil {
		if history := d.buildQueue.GetHistory(); len(history) > 0 {
			last := history[len(history)-1]
			if last != nil && last.Metadata != nil {
				if brRaw, ok := last.Metadata["build_report"]; ok {
					if br, ok2 := brRaw.(*hugo.BuildReport); ok2 && br != nil {
						if len(br.StageDurations) > 0 {
							stages := make(map[string]string, len(br.StageDurations))
							for k, v := range br.StageDurations {
								stages[k] = v.Truncate(time.Millisecond).String()
							}
							status.BuildStatus.LastBuildStages = stages
						}
						status.BuildStatus.LastBuildOutcome = string(br.Outcome)
						status.BuildStatus.LastBuildSummary = br.Summary()
						if br.RenderedPages > 0 {
							rp := br.RenderedPages
							status.BuildStatus.RenderedPages = &rp
						}
						if br.ClonedRepositories > 0 {
							cr := br.ClonedRepositories
							status.BuildStatus.ClonedRepositories = &cr
						}
						if br.FailedRepositories > 0 {
							fr := br.FailedRepositories
							status.BuildStatus.FailedRepositories = &fr
						}
						if br.SkippedRepositories > 0 {
							srk := br.SkippedRepositories
							status.BuildStatus.SkippedRepositories = &srk
						}
						if br.StaticRendered {
							sr := true
							status.BuildStatus.StaticRendered = &sr
						}
						if len(br.StageCounts) > 0 {
							m := make(map[string]map[string]int, len(br.StageCounts))
							for stage, sc := range br.StageCounts {
								m[string(stage)] = map[string]int{"success": sc.Success, "warning": sc.Warning, "fatal": sc.Fatal, "canceled": sc.Canceled}
							}
							status.BuildStatus.StageCounts = m
						}
						if len(br.Errors) > 0 {
							for _, e := range br.Errors {
								msg := e.Error()
								if len(msg) > 300 {
									msg = msg[:300] + "…"
								}
								status.BuildStatus.LastBuildErrors = append(status.BuildStatus.LastBuildErrors, msg)
							}
						}
						if len(br.Warnings) > 0 {
							for _, w := range br.Warnings {
								msg := w.Error()
								if len(msg) > 300 {
									msg = msg[:300] + "…"
								}
								status.BuildStatus.LastBuildWarnings = append(status.BuildStatus.LastBuildWarnings, msg)
							}
						}
					}
				}
			}
		}
	}

	// Repository status with version information
	slog.Debug("Status: generating repository status")
	repositories, err := d.generateRepositoryStatus()
	if err != nil {
		return nil, fmt.Errorf("failed to generate repository status: %w", err)
	}
	status.Repositories = repositories

	slog.Debug("Status: summarizing versions")
	status.VersionSummary = d.generateVersionSummary(repositories)

	slog.Debug("Status: collecting system metrics")
	status.SystemMetrics = d.generateSystemMetrics()

	// Discovery metadata
	d.discoveryCacheMu.RLock()
	if d.lastDiscovery != nil {
		status.LastDiscovery = d.lastDiscovery
	}
	if d.lastDiscoveryError != nil {
		errStr := d.lastDiscoveryError.Error()
		status.DiscoveryError = &errStr
	}
	// Extract per-forge errors from last discovery result (if any)
	if d.lastDiscoveryResult != nil && len(d.lastDiscoveryResult.Errors) > 0 {
		status.DiscoveryErrors = make(map[string]string, len(d.lastDiscoveryResult.Errors))
		for forgeName, ferr := range d.lastDiscoveryResult.Errors {
			if ferr != nil {
				// Truncate very long error strings to avoid bloating response
				msg := ferr.Error()
				if len(msg) > 500 {
					msg = msg[:500] + "… (truncated)"
				}
				status.DiscoveryErrors[forgeName] = msg
			}
		}
		if len(status.DiscoveryErrors) == 0 {
			status.DiscoveryErrors = nil // ensure omitted if all nil
		}
	}
	d.discoveryCacheMu.RUnlock()

	slog.Debug("Status: status data fully generated", "repos", len(repositories))

	return status, nil
}

// generateRepositoryStatus collects status for discovered repositories
func (d *Daemon) generateRepositoryStatus() ([]RepositoryStatus, error) {
	var repositories []RepositoryStatus

	// Use cached discovery result for fast response
	d.discoveryCacheMu.RLock()
	result := d.lastDiscoveryResult
	discoveryErr := d.lastDiscoveryError
	d.discoveryCacheMu.RUnlock()

	if discoveryErr != nil {
		slog.Warn("Using last failed discovery state for status", "error", discoveryErr)
	}

	if result == nil {
		// No discovery has run yet; return empty set with a note
		slog.Info("No discovery results cached yet; returning empty repository list")
		return repositories, nil
	}

	for _, repo := range result.Repositories {
		repoStatus := RepositoryStatus{
			Name:   repo.Name,
			URL:    repo.CloneURL,
			Status: "healthy",
		}

		// Version info (future: integrate with versionService for cached metadata)

		// Placeholder LastSync from cached discovery timestamp
		if d.lastDiscovery != nil {
			repoStatus.LastSync = d.lastDiscovery
		}

		repositories = append(repositories, repoStatus)
	}

	slog.Debug("Generated repository status from cache", "count", len(repositories))
	return repositories, nil
}

// generateVersionSummary creates version overview across all repositories
func (d *Daemon) generateVersionSummary(repositories []RepositoryStatus) VersionSummary {
	summary := VersionSummary{
		TotalRepositories: len(repositories),
		StrategyBreakdown: make(map[string]int),
		VersionTypes:      make(map[versioning.VersionType]int),
	}

	for _, repo := range repositories {
		summary.TotalVersions += repo.VersionCount

		// Count version types
		for _, version := range repo.AvailableVersions {
			summary.VersionTypes[version.Type]++
		}
	}

	// For now, assume all repos use the same strategy from config
	if d.versionService != nil {
		// TODO: Get actual strategy from version service config
		summary.StrategyBreakdown["default_only"] = len(repositories)
	}

	return summary
}

// generateSystemMetrics collects system resource information
func (d *Daemon) generateSystemMetrics() SystemMetrics {
	// TODO: Implement actual system metrics collection
	return SystemMetrics{
		MemoryUsage:    "Unknown",
		DiskUsage:      "Unknown",
		GoroutineCount: 0, // runtime.NumGoroutine()
		WorkspaceSize:  "Unknown",
	}
}

// StatusHandler serves the status page as JSON or HTML
func (d *Daemon) StatusHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	slog.Debug("Status handler invoked")
	statusData, err := d.GenerateStatusData()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to generate status: %v", err), http.StatusInternalServerError)
		return
	}

	// Quick flush test (should not block). If client requested JSON we'll overwrite.
	w.Header().Add("X-Status-Debug", "pre-serialization")

	// Record simple latency metric (best-effort)
	if d.metrics != nil {
		d.metrics.RecordHistogram("status_handler_duration_seconds", time.Since(start).Seconds())
	}

	slog.Debug("Status endpoint served", "duration", time.Since(start), "repos", len(statusData.Repositories))

	// Check if client wants JSON
	if r.Header.Get("Accept") == "application/json" || r.URL.Query().Get("format") == "json" {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(statusData); err != nil {
			slog.Error("failed to encode status json", "error", err)
		}
		return
	}

	// Serve HTML page
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	tmpl := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>DocBuilder Daemon Status</title>
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; margin: 20px; background: #f5f5f5; }
        .container { max-width: 1200px; margin: 0 auto; background: white; padding: 20px; border-radius: 8px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
        .header { border-bottom: 2px solid #eee; padding-bottom: 20px; margin-bottom: 30px; }
        .status { display: inline-block; padding: 4px 12px; border-radius: 20px; font-weight: bold; text-transform: uppercase; font-size: 12px; }
        .status.running { background: #d4edda; color: #155724; }
        .status.stopped { background: #f8d7da; color: #721c24; }
        .metrics { display: grid; grid-template-columns: repeat(auto-fit, minmax(250px, 1fr)); gap: 20px; margin: 30px 0; }
        .metric-card { background: #f8f9fa; padding: 15px; border-radius: 6px; border-left: 4px solid #007bff; }
        .metric-value { font-size: 24px; font-weight: bold; color: #007bff; }
        .metric-label { color: #666; font-size: 14px; margin-top: 4px; }
        .repo-grid { display: grid; gap: 15px; }
        .repo-card { background: #f8f9fa; padding: 15px; border-radius: 6px; border: 1px solid #dee2e6; }
        .repo-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 10px; }
        .repo-status { padding: 2px 8px; border-radius: 12px; font-size: 11px; font-weight: bold; }
        .healthy { background: #d4edda; color: #155724; }
        .error { background: #f8d7da; color: #721c24; }
        .version-list { margin-top: 10px; }
        .version-tag { display: inline-block; background: #e9ecef; padding: 2px 6px; margin: 2px; border-radius: 3px; font-size: 11px; }
        .default { background: #007bff; color: white; }
        .updated { color: #666; font-size: 12px; text-align: center; margin-top: 20px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>DocBuilder Daemon Status</h1>
            <p>
                <span class="status {{if eq .DaemonInfo.Status "running"}}running{{else}}stopped{{end}}">{{.DaemonInfo.Status}}</span>
                Version {{.DaemonInfo.Version}} • Uptime: {{.DaemonInfo.Uptime}}
            </p>
        </div>

        <div class="metrics">
            <div class="metric-card">
                <div class="metric-value">{{.VersionSummary.TotalRepositories}}</div>
                <div class="metric-label">Repositories</div>
            </div>
            <div class="metric-card">
                <div class="metric-value">{{.VersionSummary.TotalVersions}}</div>
                <div class="metric-label">Total Versions</div>
            </div>
            <div class="metric-card">
                <div class="metric-value">{{.BuildStatus.QueueLength}}</div>
                <div class="metric-label">Queued Builds</div>
            </div>
            <div class="metric-card">
                <div class="metric-value">{{.BuildStatus.ActiveJobs}}</div>
                <div class="metric-label">Active Jobs</div>
            </div>
        </div>

        <h2>Repository Status</h2>
        <div class="repo-grid">
            {{range .Repositories}}
            <div class="repo-card">
                <div class="repo-header">
                    <strong>{{.Name}}</strong>
                    <span class="repo-status {{.Status}}">{{.Status}}</span>
                </div>
                <div style="color: #666; font-size: 14px;">{{.URL}}</div>
                {{if .LastError}}
                <div style="color: #dc3545; font-size: 12px; margin-top: 5px;">Error: {{.LastError}}</div>
                {{end}}
                <div style="margin-top: 8px;">
                    <strong>{{.VersionCount}}</strong> versions available
                    {{if .DefaultVersion}}<span style="color: #666;"> • Default: {{.DefaultVersion}}</span>{{end}}
                </div>
                {{if .AvailableVersions}}
                <div class="version-list">
                    {{range .AvailableVersions}}
                    <span class="version-tag {{if .IsDefault}}default{{end}}">{{.DisplayName}}</span>
                    {{end}}
                </div>
                {{end}}
            </div>
            {{end}}
        </div>

		{{if .DiscoveryErrors}}
		<h2>Discovery Errors</h2>
		<ul>
		{{range $forge, $err := .DiscoveryErrors}}
			<li><strong>{{$forge}}:</strong> {{$err}}</li>
		{{end}}
		</ul>
		{{end}}
		<div class="updated">Last updated: {{.LastUpdated.Format "2006-01-02 15:04:05 UTC"}}</div>
    </div>
</body>
</html>`

	t, err := template.New("status").Parse(tmpl)
	if err != nil {
		http.Error(w, fmt.Sprintf("Template error: %v", err), http.StatusInternalServerError)
		return
	}

	if err := t.Execute(w, statusData); err != nil {
		http.Error(w, fmt.Sprintf("Failed to render template: %v", err), http.StatusInternalServerError)
		return
	}
}
