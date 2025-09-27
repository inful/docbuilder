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
	KeyPath        = "path"
	KeyFile        = "file"
	KeyWorker      = "worker"
	KeyMethod      = "method"
	KeyUserAgent   = "user_agent"
	KeyRemoteAddr  = "remote_addr"
	KeyRequestID   = "request_id"
	KeyStatus      = "status"
	KeyResponseSz  = "response_size"
	KeyForgeType   = "forge_type"
	KeyContentLen  = "content_length"
	KeyName        = "name"
	KeyURL         = "url"
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
// Additional common context helpers
func Path(p string) slog.Attr         { return slog.String(KeyPath, p) }
func File(f string) slog.Attr         { return slog.String(KeyFile, f) }
func Worker(id string) slog.Attr      { return slog.String(KeyWorker, id) }
func Method(m string) slog.Attr       { return slog.String(KeyMethod, m) }
func UserAgent(ua string) slog.Attr   { return slog.String(KeyUserAgent, ua) }
func RemoteAddr(a string) slog.Attr   { return slog.String(KeyRemoteAddr, a) }
func RequestID(id string) slog.Attr   { return slog.String(KeyRequestID, id) }
func Status(code int) slog.Attr       { return slog.Int(KeyStatus, code) }
func ResponseSize(sz int) slog.Attr   { return slog.Int(KeyResponseSz, sz) }
func ForgeType(t string) slog.Attr    { return slog.String(KeyForgeType, t) }
func ContentLength(cl int64) slog.Attr { return slog.Int64(KeyContentLen, cl) }
func Name(n string) slog.Attr         { return slog.String(KeyName, n) }
func URL(u string) slog.Attr          { return slog.String(KeyURL, u) }
func Error(err error) slog.Attr {
	if err == nil { return slog.String(KeyError, "") }
	return slog.String(KeyError, err.Error())
}
