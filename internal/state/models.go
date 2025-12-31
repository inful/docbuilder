package state

import (
	"slices"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/foundation"
)

// Repository represents the state of a tracked repository.
type Repository struct {
	URL           string                       `json:"url"`
	Name          string                       `json:"name"`
	Branch        string                       `json:"branch"`
	LastDiscovery foundation.Option[time.Time] `json:"last_discovery"`
	LastBuild     foundation.Option[time.Time] `json:"last_build"`
	LastCommit    foundation.Option[string]    `json:"last_commit"`
	DocumentCount int                          `json:"document_count"`
	BuildCount    int64                        `json:"build_count"`
	ErrorCount    int64                        `json:"error_count"`
	LastError     foundation.Option[string]    `json:"last_error"`
	DocFilesHash  foundation.Option[string]    `json:"doc_files_hash"`
	DocFilePaths  []string                     `json:"doc_file_paths"`
	CreatedAt     time.Time                    `json:"created_at"`
	UpdatedAt     time.Time                    `json:"updated_at"`
}

// BuildStatus represents the status of a build operation.
type BuildStatus string

const (
	BuildStatusPending   BuildStatus = "pending"
	BuildStatusRunning   BuildStatus = "running"
	BuildStatusCompleted BuildStatus = "completed"
	BuildStatusFailed    BuildStatus = "failed"
	BuildStatusCanceled  BuildStatus = "canceled"
)

// Build represents a build operation and its state.
type Build struct {
	ID          string                       `json:"id"`
	Status      BuildStatus                  `json:"status"`
	StartTime   time.Time                    `json:"start_time"`
	EndTime     foundation.Option[time.Time] `json:"end_time"`
	Duration    foundation.Option[float64]   `json:"duration_seconds"`
	TriggeredBy string                       `json:"triggered_by"`
	CommitHash  foundation.Option[string]    `json:"commit_hash"`
	ErrorMsg    foundation.Option[string]    `json:"error_message"`
	OutputPath  foundation.Option[string]    `json:"output_path"`
	LogLevel    string                       `json:"log_level"`
	CreatedAt   time.Time                    `json:"created_at"`
	UpdatedAt   time.Time                    `json:"updated_at"`
}

// ScheduleConfig represents strongly-typed config for a schedule.
type ScheduleConfig struct {
	// Add fields as needed for your domain, e.g.:
	MaxRetries  int    `json:"max_retries,omitempty"`
	NotifyEmail string `json:"notify_email,omitempty"`
	CustomParam string `json:"custom_param,omitempty"`
}

// Schedule represents a scheduled build operation.
type Schedule struct {
	ID           string                       `json:"id"`
	Name         string                       `json:"name"`
	CronExpr     string                       `json:"cron_expression"`
	IsActive     bool                         `json:"is_active"`
	LastRun      foundation.Option[time.Time] `json:"last_run"`
	NextRun      foundation.Option[time.Time] `json:"next_run"`
	RunCount     int64                        `json:"run_count"`
	FailureCount int64                        `json:"failure_count"`
	Config       ScheduleConfig               `json:"config"`
	CreatedAt    time.Time                    `json:"created_at"`
	UpdatedAt    time.Time                    `json:"updated_at"`
}

// Statistics represents aggregate daemon statistics.
type Statistics struct {
	TotalBuilds      int64     `json:"total_builds"`
	SuccessfulBuilds int64     `json:"successful_builds"`
	FailedBuilds     int64     `json:"failed_builds"`
	TotalDiscoveries int64     `json:"total_discoveries"`
	DocumentsFound   int64     `json:"documents_found"`
	AverageBuildTime float64   `json:"average_build_time_seconds"`
	LastStatReset    time.Time `json:"last_stat_reset"`
	UptimeSeconds    float64   `json:"uptime_seconds"`
	LastUpdated      time.Time `json:"last_updated"`
}

// DaemonInfo represents overall daemon state information.
type DaemonInfo struct {
	Version    string    `json:"version"`
	StartTime  time.Time `json:"start_time"`
	LastUpdate time.Time `json:"last_update"`
	Status     string    `json:"status"`
}

// Validate validates a Repository using foundation utilities.
func (r *Repository) Validate() foundation.ValidationResult {
	var errors []foundation.FieldError

	// Validate required fields
	if r.URL == "" {
		errors = append(errors, foundation.NewValidationError("url", "required", "URL is required"))
	}
	if r.Name == "" {
		errors = append(errors, foundation.NewValidationError("name", "required", "name is required"))
	}
	if r.Branch == "" {
		errors = append(errors, foundation.NewValidationError("branch", "required", "branch is required"))
	}

	// Validate non-negative counts
	if r.DocumentCount < 0 {
		errors = append(errors, foundation.NewValidationError("document_count", "non_negative", "document count must be non-negative"))
	}
	if r.BuildCount < 0 {
		errors = append(errors, foundation.NewValidationError("build_count", "non_negative", "build count must be non-negative"))
	}
	if r.ErrorCount < 0 {
		errors = append(errors, foundation.NewValidationError("error_count", "non_negative", "error count must be non-negative"))
	}

	if len(errors) > 0 {
		return foundation.Invalid(errors...)
	}
	return foundation.Valid()
}

// Validate validates a Build using foundation utilities.
func (b *Build) Validate() foundation.ValidationResult {
	var errors []foundation.FieldError

	// Validate required fields
	if b.ID == "" {
		errors = append(errors, foundation.NewValidationError("id", "required", "ID is required"))
	}
	if b.TriggeredBy == "" {
		errors = append(errors, foundation.NewValidationError("triggered_by", "required", "triggered_by is required"))
	}

	// Validate status
	validStatuses := []BuildStatus{BuildStatusPending, BuildStatusRunning, BuildStatusCompleted, BuildStatusFailed, BuildStatusCanceled}
	statusValid := slices.Contains(validStatuses, b.Status)
	if !statusValid {
		errors = append(errors, foundation.NewValidationError("status", "invalid", "invalid build status"))
	}

	// Validate time relationships
	if b.EndTime.IsSome() && b.EndTime.Unwrap().Before(b.StartTime) {
		errors = append(errors, foundation.NewValidationError("end_time", "invalid", "end time must be after start time"))
	}

	if len(errors) > 0 {
		return foundation.Invalid(errors...)
	}
	return foundation.Valid()
}

// Validate validates a Schedule using foundation utilities.
func (s *Schedule) Validate() foundation.ValidationResult {
	var errors []foundation.FieldError

	// Validate required fields
	if s.ID == "" {
		errors = append(errors, foundation.NewValidationError("id", "required", "ID is required"))
	}
	if s.Name == "" {
		errors = append(errors, foundation.NewValidationError("name", "required", "name is required"))
	}
	if s.CronExpr == "" {
		errors = append(errors, foundation.NewValidationError("cron_expression", "required", "cron expression is required"))
	}

	// Validate non-negative counts
	if s.RunCount < 0 {
		errors = append(errors, foundation.NewValidationError("run_count", "non_negative", "run count must be non-negative"))
	}
	if s.FailureCount < 0 {
		errors = append(errors, foundation.NewValidationError("failure_count", "non_negative", "failure count must be non-negative"))
	}

	if len(errors) > 0 {
		return foundation.Invalid(errors...)
	}
	return foundation.Valid()
}

// UpdateBuildStats updates statistics when a build completes.
func (s *Statistics) UpdateBuildStats(build *Build) {
	if build == nil {
		return
	}

	s.TotalBuilds++

	switch build.Status {
	case BuildStatusCompleted:
		s.SuccessfulBuilds++
	case BuildStatusFailed:
		s.FailedBuilds++
	case BuildStatusPending, BuildStatusRunning, BuildStatusCanceled:
		// These statuses don't update success/failure counters
	}

	// Update average build time if we have duration
	if build.Duration.IsSome() {
		duration := build.Duration.Unwrap()
		if s.TotalBuilds == 1 {
			s.AverageBuildTime = duration
		} else {
			// Calculate running average
			totalTime := s.AverageBuildTime * float64(s.TotalBuilds-1)
			s.AverageBuildTime = (totalTime + duration) / float64(s.TotalBuilds)
		}
	}

	s.LastUpdated = time.Now()
}

// UpdateDiscoveryStats updates statistics when a discovery completes.
func (s *Statistics) UpdateDiscoveryStats(documentCount int) {
	s.TotalDiscoveries++
	s.DocumentsFound += int64(documentCount)
	s.LastUpdated = time.Now()
}

// Reset resets statistics counters.
func (s *Statistics) Reset() {
	s.TotalBuilds = 0
	s.SuccessfulBuilds = 0
	s.FailedBuilds = 0
	s.TotalDiscoveries = 0
	s.DocumentsFound = 0
	s.AverageBuildTime = 0
	s.LastStatReset = time.Now()
	s.LastUpdated = time.Now()
}
