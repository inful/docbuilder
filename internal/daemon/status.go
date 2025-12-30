package daemon

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"runtime"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/foundation/errors"
	"git.home.luguber.info/inful/docbuilder/internal/logfields"
	"git.home.luguber.info/inful/docbuilder/internal/version"
	"git.home.luguber.info/inful/docbuilder/internal/versioning"
)

// StatusPageData represents data for status page rendering.
type StatusPageData struct {
	DaemonInfo     Info               `json:"daemon_info"`
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

// Info holds basic daemon information.
type Info struct {
	Status     Status    `json:"status"`
	Version    string    `json:"version"`
	StartTime  time.Time `json:"start_time"`
	Uptime     string    `json:"uptime"`
	ConfigFile string    `json:"config_file"`
}

// RepositoryStatus tracks status of individual repositories.
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

// VersionSummary provides overview of versioning across all repositories.
type VersionSummary struct {
	TotalRepositories int                            `json:"total_repositories"`
	TotalVersions     int                            `json:"total_versions"`
	StrategyBreakdown map[string]int                 `json:"strategy_breakdown"`
	VersionTypes      map[versioning.VersionType]int `json:"version_types"`
}

// BuildStatusInfo tracks build queue and execution status.
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

// SystemMetrics provides system resource information.
type SystemMetrics struct {
	MemoryUsage    string `json:"memory_usage"`
	DiskUsage      string `json:"disk_usage"`
	GoroutineCount int    `json:"goroutine_count"`
	WorkspaceSize  string `json:"workspace_size"`
}

// GenerateStatusData collects and formats status information.
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
	// Safely get daemon status with fallback
	var daemonStatus Status
	if statusVal := d.status.Load(); statusVal != nil {
		daemonStatus = statusVal.(Status)
	} else {
		daemonStatus = StatusStopped // Default to stopped if not initialized
	}

	configFile := d.configFilePath
	if configFile == "" {
		configFile = "config.yaml" // fallback for when path not provided
	}
	status.DaemonInfo = Info{
		Status:     daemonStatus,
		Version:    version.Version,
		StartTime:  d.startTime,
		Uptime:     time.Since(d.startTime).String(),
		ConfigFile: configFile,
	}

	slog.Debug("Status: collecting build status")
	// Update queue length from build queue if available
	if d.buildQueue != nil {
		qLen := d.buildQueue.Length()
		if qLen > 2147483647 { // max int32
			qLen = 2147483647
		}
		status.BuildStatus.QueueLength = int32(qLen) // #nosec G115 - bounds checked
	} else {
		status.BuildStatus.QueueLength = d.queueLength
	}
	status.BuildStatus.ActiveJobs = d.activeJobs
	status.BuildStatus.LastBuildTime = d.lastBuild

	// Extract most recent build stage timings from event-sourced projection (Phase B)
	if d.buildProjection != nil {
		if last := d.buildProjection.GetLastCompletedBuild(); last != nil && last.ReportData != nil {
			rd := last.ReportData

			// Convert stage durations from milliseconds to human-readable strings
			if len(rd.StageDurations) > 0 {
				stages := make(map[string]string, len(rd.StageDurations))
				for k, ms := range rd.StageDurations {
					stages[k] = (time.Duration(ms) * time.Millisecond).Truncate(time.Millisecond).String()
				}
				status.BuildStatus.LastBuildStages = stages
			}

			status.BuildStatus.LastBuildOutcome = rd.Outcome
			status.BuildStatus.LastBuildSummary = rd.Summary

			if rd.RenderedPages > 0 {
				rp := rd.RenderedPages
				status.BuildStatus.RenderedPages = &rp
			}
			if rd.ClonedRepositories > 0 {
				cr := rd.ClonedRepositories
				status.BuildStatus.ClonedRepositories = &cr
			}
			if rd.FailedRepositories > 0 {
				fr := rd.FailedRepositories
				status.BuildStatus.FailedRepositories = &fr
			}
			if rd.SkippedRepositories > 0 {
				srk := rd.SkippedRepositories
				status.BuildStatus.SkippedRepositories = &srk
			}
			if rd.StaticRendered {
				sr := true
				status.BuildStatus.StaticRendered = &sr
			}

			// Copy errors and warnings from report data
			status.BuildStatus.LastBuildErrors = rd.Errors
			status.BuildStatus.LastBuildWarnings = rd.Warnings
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
	if lastDiscovery := d.discoveryRunner.GetLastDiscovery(); lastDiscovery != nil {
		status.LastDiscovery = lastDiscovery
	}
	result, discoveryErr := d.discoveryCache.Get()
	if discoveryErr != nil {
		errStr := discoveryErr.Error()
		status.DiscoveryError = &errStr
	}
	// Extract per-forge errors from last discovery result (if any)
	if result != nil && len(result.Errors) > 0 {
		status.DiscoveryErrors = make(map[string]string, len(result.Errors))
		for forgeName, ferr := range result.Errors {
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

	slog.Debug("Status: status data fully generated", "repos", len(repositories))

	return status, nil
}

// generateRepositoryStatus collects status for discovered repositories
//
//nolint:unparam // generateRepositoryStatus currently never returns an error.
func (d *Daemon) generateRepositoryStatus() ([]RepositoryStatus, error) {
	var repositories []RepositoryStatus

	// Use cached discovery result for fast response
	result, discoveryErr := d.discoveryCache.Get()

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
		if lastDiscovery := d.discoveryRunner.GetLastDiscovery(); lastDiscovery != nil {
			repoStatus.LastSync = lastDiscovery
		}

		repositories = append(repositories, repoStatus)
	}

	slog.Debug("Generated repository status from cache", "count", len(repositories))
	return repositories, nil
}

// generateVersionSummary creates version overview across all repositories.
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

	// Get actual strategy from config
	if d.config.Versioning != nil && d.config.Versioning.Strategy != "" {
		strategy := string(d.config.Versioning.Strategy)
		summary.StrategyBreakdown[strategy] = len(repositories)
	} else {
		summary.StrategyBreakdown["default_only"] = len(repositories)
	}

	return summary
}

// generateSystemMetrics collects system resource information.
func (d *Daemon) generateSystemMetrics() SystemMetrics {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// Format memory usage in MB
	memUsageMB := float64(m.Alloc) / 1024 / 1024
	memUsage := fmt.Sprintf("%.2f MB", memUsageMB)

	return SystemMetrics{
		MemoryUsage:    memUsage,
		DiskUsage:      "N/A", // Disk usage requires platform-specific syscalls
		GoroutineCount: runtime.NumGoroutine(),
		WorkspaceSize:  "Unknown",
	}
}

// StatusHandler serves the status page as JSON or HTML.
func (d *Daemon) StatusHandler(w http.ResponseWriter, r *http.Request) {
	errorAdapter := errors.NewHTTPErrorAdapter(slog.Default())

	start := time.Now()
	slog.Debug("Status handler invoked")
	statusData, err := d.GenerateStatusData()
	if err != nil {
		internalErr := errors.WrapError(err, errors.CategoryInternal, "failed to generate status data").
			Build()
		errorAdapter.WriteErrorResponse(w, internalErr)
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
		if encodeErr := json.NewEncoder(w).Encode(statusData); encodeErr != nil {
			slog.Error("failed to encode status json", logfields.Error(encodeErr))
			internalErr := errors.WrapError(encodeErr, errors.CategoryInternal, "failed to encode status json").Build()
			errorAdapter.WriteErrorResponse(w, internalErr)
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
		internalErr := errors.WrapError(err, errors.CategoryInternal, "failed to parse status template").
			Build()
		errorAdapter.WriteErrorResponse(w, internalErr)
		return
	}

	if err := t.Execute(w, statusData); err != nil {
		internalErr := errors.WrapError(err, errors.CategoryInternal, "failed to render status template").
			Build()
		errorAdapter.WriteErrorResponse(w, internalErr)
		return
	}
}
