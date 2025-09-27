package logfields

import "log/slog"

// Canonical log field name constants to avoid drift across packages.
const (
	KeyJobID       = "job_id"
	KeyJobType     = "job_type"
	KeyJobPriority = "job_priority"
	KeyJobStatus   = "job_status"
	KeyStage       = "stage"
	KeyDurationMS  = "duration_ms"
	KeyScheduleID  = "schedule_id"
	KeySchedule    = "schedule_name"
	KeyRepo        = "repository"
	KeySection     = "section"
	KeyError       = "error"
)

// Simple helpers returning slog.Attr. Keeping each granular means callers can compose.
func JobID(id string) slog.Attr       { return slog.String(KeyJobID, id) }
func JobType(t string) slog.Attr      { return slog.String(KeyJobType, t) }
func JobPriority(p int) slog.Attr     { return slog.Int(KeyJobPriority, p) }
func JobStatus(s string) slog.Attr    { return slog.String(KeyJobStatus, s) }
func Stage(name string) slog.Attr     { return slog.String(KeyStage, name) }
func DurationMS(ms float64) slog.Attr { return slog.Float64(KeyDurationMS, ms) }
func ScheduleID(id string) slog.Attr  { return slog.String(KeyScheduleID, id) }
func ScheduleName(n string) slog.Attr { return slog.String(KeySchedule, n) }
func Repository(r string) slog.Attr   { return slog.String(KeyRepo, r) }
func Section(s string) slog.Attr      { return slog.String(KeySection, s) }
func Error(err error) slog.Attr {
	if err == nil { return slog.String(KeyError, "") }
	return slog.String(KeyError, err.Error())
}
