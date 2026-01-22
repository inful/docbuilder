package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"runtime"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/eventstore"
	"git.home.luguber.info/inful/docbuilder/internal/forge"
	"git.home.luguber.info/inful/docbuilder/internal/foundation/errors"
	"git.home.luguber.info/inful/docbuilder/internal/logfields"
	"git.home.luguber.info/inful/docbuilder/internal/version"
	"git.home.luguber.info/inful/docbuilder/internal/versioning"
)

// StatusProvider defines the minimal surface needed to render the admin status page.
// It is implemented by the daemon (and can be implemented by preview runtime adapters).
//
// Keep this interface stable: admin status page should not require deep daemon coupling.
//
//nolint:interfacebloat // Intentionally explicit to avoid leaking daemon internals.
type StatusProvider interface {
	GetStatus() string
	GetStartTime() time.Time
	GetActiveJobs() int
	GetQueueLength() int

	GetConfigFilePath() string
	GetConfig() *config.Config

	GetLastBuildTime() *time.Time
	GetBuildProjection() *eventstore.BuildHistoryProjection

	GetLastDiscovery() *time.Time
	GetDiscoveryResult() (*forge.DiscoveryResult, error)
}

// DaemonStatus is rendered on the status page.
// This is an alias so callers can pass plain strings.
type DaemonStatus = string

// StatusPageData represents data for status page rendering.
type StatusPageData struct {
	DaemonInfo      Info               `json:"daemon_info"`
	Repositories    []RepositoryStatus `json:"repositories"`
	VersionSummary  VersionSummary     `json:"version_summary"`
	BuildStatus     BuildStatusInfo    `json:"build_status"`
	SystemMetrics   SystemMetrics      `json:"system_metrics"`
	LastUpdated     time.Time          `json:"last_updated"`
	LastDiscovery   *time.Time         `json:"last_discovery,omitempty"`
	DiscoveryError  *string            `json:"discovery_error,omitempty"`
	DiscoveryErrors map[string]string  `json:"discovery_errors,omitempty"`
}

// Info holds basic daemon information.
type Info struct {
	Status     DaemonStatus `json:"status"`
	Version    string       `json:"version"`
	StartTime  time.Time    `json:"start_time"`
	Uptime     string       `json:"uptime"`
	ConfigFile string       `json:"config_file"`
}

// RepositoryStatus tracks status of individual repositories.
type RepositoryStatus struct {
	Name              string               `json:"name"`
	URL               string               `json:"url"`
	LastSync          *time.Time           `json:"last_sync"`
	LastBuild         *time.Time           `json:"last_build"`
	Status            string               `json:"status"`
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
	LastBuildStages     map[string]string         `json:"last_build_stages,omitempty"`
	LastBuildOutcome    string                    `json:"last_build_outcome,omitempty"`
	LastBuildSummary    string                    `json:"last_build_summary,omitempty"`
	LastBuildErrors     []string                  `json:"last_build_errors,omitempty"`
	LastBuildWarnings   []string                  `json:"last_build_warnings,omitempty"`
	RenderedPages       *int                      `json:"rendered_pages,omitempty"`
	ClonedRepositories  *int                      `json:"cloned_repositories,omitempty"`
	FailedRepositories  *int                      `json:"failed_repositories,omitempty"`
	SkippedRepositories *int                      `json:"skipped_repositories,omitempty"`
	StaticRendered      *bool                     `json:"static_rendered,omitempty"`
	StageCounts         map[string]map[string]int `json:"stage_counts,omitempty"`
}

// SystemMetrics provides system resource information.
type SystemMetrics struct {
	MemoryUsage    string `json:"memory_usage"`
	DiskUsage      string `json:"disk_usage"`
	GoroutineCount int    `json:"goroutine_count"`
	WorkspaceSize  string `json:"workspace_size"`
}

// GenerateStatusData collects and formats status information.
func GenerateStatusData(_ context.Context, p StatusProvider) (*StatusPageData, error) {
	if p == nil {
		return nil, errors.ValidationError("status provider is nil").Build()
	}

	cfgFile := p.GetConfigFilePath()
	if cfgFile == "" {
		cfgFile = "config.yaml"
	}

	st := p.GetStatus()
	if st == "" {
		st = "stopped"
	}

	data := &StatusPageData{LastUpdated: time.Now()}
	data.DaemonInfo = Info{
		Status:     st,
		Version:    version.Version,
		StartTime:  p.GetStartTime(),
		Uptime:     time.Since(p.GetStartTime()).String(),
		ConfigFile: cfgFile,
	}

	data.BuildStatus.QueueLength = int32(p.GetQueueLength()) // #nosec G115 -- bounded by runtime int
	data.BuildStatus.ActiveJobs = int32(p.GetActiveJobs())   // #nosec G115 -- bounded by runtime int
	data.BuildStatus.LastBuildTime = p.GetLastBuildTime()

	if proj := p.GetBuildProjection(); proj != nil {
		if last := proj.GetLastCompletedBuild(); last != nil && last.ReportData != nil {
			rd := last.ReportData
			if len(rd.StageDurations) > 0 {
				data.BuildStatus.LastBuildStages = convertStageDurations(rd.StageDurations)
			}
			data.BuildStatus.LastBuildOutcome = rd.Outcome
			data.BuildStatus.LastBuildSummary = rd.Summary
			populateBuildMetricsFromReport(rd, &data.BuildStatus)
			data.BuildStatus.LastBuildErrors = rd.Errors
			data.BuildStatus.LastBuildWarnings = rd.Warnings
		}
	}

	data.Repositories = generateRepositoryStatus(p)
	data.VersionSummary = generateVersionSummary(p.GetConfig(), data.Repositories)
	data.SystemMetrics = generateSystemMetrics()

	if last := p.GetLastDiscovery(); last != nil {
		data.LastDiscovery = last
	}
	res, derr := p.GetDiscoveryResult()
	if derr != nil {
		es := derr.Error()
		data.DiscoveryError = &es
	}
	if res != nil && len(res.Errors) > 0 {
		data.DiscoveryErrors = make(map[string]string, len(res.Errors))
		for forgeName, ferr := range res.Errors {
			if ferr == nil {
				continue
			}
			msg := ferr.Error()
			if len(msg) > 500 {
				msg = msg[:500] + "… (truncated)"
			}
			data.DiscoveryErrors[forgeName] = msg
		}
		if len(data.DiscoveryErrors) == 0 {
			data.DiscoveryErrors = nil
		}
	}

	return data, nil
}

func generateRepositoryStatus(p StatusProvider) []RepositoryStatus {
	repositories := make([]RepositoryStatus, 0)
	res, _ := p.GetDiscoveryResult()
	if res == nil {
		return repositories
	}
	for _, repo := range res.Repositories {
		repoStatus := RepositoryStatus{
			Name:   repo.Name,
			URL:    repo.CloneURL,
			Status: "healthy",
		}
		if lastDiscovery := p.GetLastDiscovery(); lastDiscovery != nil {
			repoStatus.LastSync = lastDiscovery
		}
		repositories = append(repositories, repoStatus)
	}
	return repositories
}

func generateVersionSummary(cfg *config.Config, repositories []RepositoryStatus) VersionSummary {
	summary := VersionSummary{
		TotalRepositories: len(repositories),
		StrategyBreakdown: make(map[string]int),
		VersionTypes:      make(map[versioning.VersionType]int),
	}

	for i := range repositories {
		repo := &repositories[i]
		summary.TotalVersions += repo.VersionCount
		for j := range repo.AvailableVersions {
			summary.VersionTypes[repo.AvailableVersions[j].Type]++
		}
	}

	strategy := "default_only"
	if cfg != nil && cfg.Versioning != nil && cfg.Versioning.Strategy != "" {
		strategy = string(cfg.Versioning.Strategy)
	}
	summary.StrategyBreakdown[strategy] = len(repositories)
	return summary
}

func generateSystemMetrics() SystemMetrics {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	memUsageMB := float64(m.Alloc) / 1024 / 1024
	memUsage := fmt.Sprintf("%.2f MB", memUsageMB)

	return SystemMetrics{
		MemoryUsage:    memUsage,
		DiskUsage:      "N/A",
		GoroutineCount: runtime.NumGoroutine(),
		WorkspaceSize:  "Unknown",
	}
}

// StatusPageHandlers serves the status page as JSON or HTML.
type StatusPageHandlers struct {
	provider     StatusProvider
	errorAdapter *errors.HTTPErrorAdapter
}

func NewStatusPageHandlers(provider StatusProvider) *StatusPageHandlers {
	return &StatusPageHandlers{
		provider:     provider,
		errorAdapter: errors.NewHTTPErrorAdapter(slog.Default()),
	}
}

func (h *StatusPageHandlers) HandleStatusPage(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	data, err := GenerateStatusData(r.Context(), h.provider)
	if err != nil {
		h.errorAdapter.WriteErrorResponse(w, r, err)
		return
	}

	// Record simple latency metric (best-effort). This is intentionally logged here to avoid
	// requiring a metrics dependency in the provider interface.
	slog.Debug("Status endpoint served", slog.Duration("duration", time.Since(start)), slog.Int("repos", len(data.Repositories)))

	if r.Header.Get("Accept") == "application/json" || r.URL.Query().Get("format") == "json" {
		w.Header().Set("Content-Type", "application/json")
		if encodeErr := json.NewEncoder(w).Encode(data); encodeErr != nil {
			slog.Error("failed to encode status json", logfields.Error(encodeErr))
			internalErr := errors.WrapError(encodeErr, errors.CategoryInternal, "failed to encode status json").Build()
			h.errorAdapter.WriteErrorResponse(w, r, internalErr)
		}
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	t, err := template.New("status").Parse(statusHTMLTemplate)
	if err != nil {
		internalErr := errors.WrapError(err, errors.CategoryInternal, "failed to parse status template").Build()
		h.errorAdapter.WriteErrorResponse(w, r, internalErr)
		return
	}
	if err := t.Execute(w, data); err != nil {
		internalErr := errors.WrapError(err, errors.CategoryInternal, "failed to render status template").Build()
		h.errorAdapter.WriteErrorResponse(w, r, internalErr)
		return
	}
}

const statusHTMLTemplate = `<!DOCTYPE html>
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

func convertStageDurations(stageDurations map[string]int64) map[string]string {
	stages := make(map[string]string, len(stageDurations))
	for k, ms := range stageDurations {
		stages[k] = (time.Duration(ms) * time.Millisecond).Truncate(time.Millisecond).String()
	}
	return stages
}

func populateBuildMetricsFromReport(rd *eventstore.BuildReportData, buildStatus *BuildStatusInfo) {
	if rd.RenderedPages > 0 {
		rp := rd.RenderedPages
		buildStatus.RenderedPages = &rp
	}
	if rd.ClonedRepositories > 0 {
		cr := rd.ClonedRepositories
		buildStatus.ClonedRepositories = &cr
	}
	if rd.FailedRepositories > 0 {
		fr := rd.FailedRepositories
		buildStatus.FailedRepositories = &fr
	}
	if rd.SkippedRepositories > 0 {
		srk := rd.SkippedRepositories
		buildStatus.SkippedRepositories = &srk
	}
	if rd.StaticRendered {
		sr := true
		buildStatus.StaticRendered = &sr
	}
}
