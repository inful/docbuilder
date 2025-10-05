// Package responses defines API response types used by DocBuilder HTTP handlers.
package responses

import "time"

// ServerStatusResponse represents the daemon status API response.
type ServerStatusResponse struct {
	Status      string    `json:"status"`
	Title       string    `json:"title"`
	Description string    `json:"description,omitempty"`
	Theme       string    `json:"theme"`
	BaseURL     string    `json:"base_url,omitempty"`
	OutputDir   string    `json:"output_dir"`
	Timestamp   time.Time `json:"timestamp"`
}

// DaemonStatusResponse represents the daemon's operational status.
type DaemonStatusResponse struct {
	Status    string              `json:"status"`
	Uptime    float64             `json:"uptime"`
	StartTime time.Time           `json:"start_time"`
	Config    DaemonConfigSummary `json:"config"`
}

// DaemonConfigSummary represents a summary of daemon configuration.
type DaemonConfigSummary struct {
	ForgesCount      int    `json:"forges_count"`
	SyncSchedule     string `json:"sync_schedule"`
	ConcurrentBuilds int    `json:"concurrent_builds"`
	QueueSize        int    `json:"queue_size"`
}

// TriggerResponse represents the response for trigger operations.
type TriggerResponse struct {
	Status string `json:"status"`
	JobID  string `json:"job_id"`
}

// HealthResponse represents the health check API response.
type HealthResponse struct {
	Status       string    `json:"status"`
	Timestamp    time.Time `json:"timestamp"`
	Version      string    `json:"version"`
	Uptime       float64   `json:"uptime"`
	DaemonStatus string    `json:"daemon_status,omitempty"`
	ActiveJobs   int       `json:"active_jobs,omitempty"`
}

// MetricsResponse represents the metrics endpoint response.
type MetricsResponse struct {
	Status                string    `json:"status"`
	Timestamp             time.Time `json:"timestamp"`
	HTTPRequestsTotal     int       `json:"http_requests_total"`
	ActiveJobs            int       `json:"active_jobs"`
	LastDiscoveryDuration int       `json:"last_discovery_duration"`
	LastBuildDuration     int       `json:"last_build_duration"`
	RepositoriesTotal     int       `json:"repositories_total"`
}

// ConfigResponse represents the configuration API response.
type ConfigResponse struct {
	Status    string        `json:"status"`
	Config    ConfigSummary `json:"config"`
	Timestamp time.Time     `json:"timestamp"`
}

// ConfigSummary represents a sanitized view of the configuration.
type ConfigSummary struct {
	Hugo   HugoSummary    `json:"hugo"`
	Daemon DaemonSummary  `json:"daemon"`
	Forges []ForgeSummary `json:"forges"`
}

// HugoSummary represents Hugo configuration for API responses.
type HugoSummary struct {
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Theme       string `json:"theme"`
	BaseURL     string `json:"base_url,omitempty"`
}

// DaemonSummary represents daemon configuration for API responses.
type DaemonSummary struct {
	DocsPort    int `json:"docs_port"`
	WebhookPort int `json:"webhook_port"`
	AdminPort   int `json:"admin_port"`
}

// ForgeSummary represents forge configuration for API responses (sanitized).
type ForgeSummary struct {
	Name          string   `json:"name"`
	Type          string   `json:"type"`
	BaseURL       string   `json:"base_url,omitempty"`
	Organizations []string `json:"organizations,omitempty"`
	Groups        []string `json:"groups,omitempty"`
	AutoDiscover  bool     `json:"auto_discover"`
}

// BuildTriggerResponse represents the build trigger API response.
type BuildTriggerResponse struct {
	Status    string    `json:"status"`
	Message   string    `json:"message"`
	JobID     string    `json:"job_id,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// ErrorResponse represents an error API response.
type ErrorResponse struct {
	Status    string    `json:"status"`
	Error     string    `json:"error"`
	Details   string    `json:"details,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// BuildStatusResponse represents build status information.
type BuildStatusResponse struct {
	Status       string          `json:"status"`
	CurrentBuild *BuildInfo      `json:"current_build,omitempty"`
	LastBuild    *BuildInfo      `json:"last_build,omitempty"`
	QueueLength  int             `json:"queue_length"`
	Statistics   BuildStatistics `json:"statistics"`
	Timestamp    time.Time       `json:"timestamp"`
}

// BuildInfo represents information about a specific build.
type BuildInfo struct {
	ID         string            `json:"id"`
	Status     string            `json:"status"`
	StartTime  *time.Time        `json:"start_time,omitempty"`
	EndTime    *time.Time        `json:"end_time,omitempty"`
	Duration   *string           `json:"duration,omitempty"`
	Stages     map[string]string `json:"stages,omitempty"`
	Trigger    string            `json:"trigger,omitempty"`
	Repository string            `json:"repository,omitempty"`
}

// BuildStatistics represents build statistics.
type BuildStatistics struct {
	TotalBuilds      int                       `json:"total_builds"`
	SuccessfulBuilds int                       `json:"successful_builds"`
	FailedBuilds     int                       `json:"failed_builds"`
	StageCounts      map[string]map[string]int `json:"stage_counts,omitempty"`
}

// RepositoryStatusResponse represents repository status information.
type RepositoryStatusResponse struct {
	Status       string           `json:"status"`
	Repositories []RepositoryInfo `json:"repositories"`
	Timestamp    time.Time        `json:"timestamp"`
}

// RepositoryInfo represents information about a repository.
type RepositoryInfo struct {
	Name       string     `json:"name"`
	URL        string     `json:"url"`
	Branch     string     `json:"branch"`
	LastSync   *time.Time `json:"last_sync,omitempty"`
	LastCommit string     `json:"last_commit,omitempty"`
	DocCount   int        `json:"doc_count"`
	Status     string     `json:"status"`
}
