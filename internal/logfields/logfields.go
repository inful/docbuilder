// Package logfields provides canonical log field names and helpers for structured logging in DocBuilder.
package logfields

import "log/slog"

// Canonical log field name constants to avoid drift across packages.
// These are used for structured logging with slog.
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

// JobID returns a slog.Attr for the job ID field.
//
// The following helpers return slog.Attr for common log fields, allowing composable structured logging.

func JobID(id string) slog.Attr       { return slog.String(KeyJobID, id) }       // JobID returns a slog.Attr for job ID.
func JobType(t string) slog.Attr      { return slog.String(KeyJobType, t) }      // JobType returns a slog.Attr for job type.
func JobPriority(p int) slog.Attr     { return slog.Int(KeyJobPriority, p) }     // JobPriority returns a slog.Attr for job priority.
func JobStatus(s string) slog.Attr    { return slog.String(KeyJobStatus, s) }    // JobStatus returns a slog.Attr for job status.
func Stage(name string) slog.Attr     { return slog.String(KeyStage, name) }     // Stage returns a slog.Attr for stage name.
func DurationMS(ms float64) slog.Attr { return slog.Float64(KeyDurationMS, ms) } // DurationMS returns a slog.Attr for duration in ms.
func ScheduleID(id string) slog.Attr  { return slog.String(KeyScheduleID, id) }  // ScheduleID returns a slog.Attr for schedule ID.
func ScheduleName(n string) slog.Attr { return slog.String(KeySchedule, n) }     // ScheduleName returns a slog.Attr for schedule name.
func Repository(r string) slog.Attr   { return slog.String(KeyRepo, r) }         // Repository returns a slog.Attr for repository name.
func Section(s string) slog.Attr      { return slog.String(KeySection, s) }      // Section returns a slog.Attr for section name.

// Path returns a slog.Attr for a file path.
func Path(p string) slog.Attr { return slog.String(KeyPath, p) }

// File returns a slog.Attr for a file name.
func File(f string) slog.Attr { return slog.String(KeyFile, f) }

// Worker returns a slog.Attr for a worker ID.
func Worker(id string) slog.Attr { return slog.String(KeyWorker, id) }

// Method returns a slog.Attr for an HTTP method.
func Method(m string) slog.Attr { return slog.String(KeyMethod, m) }

// UserAgent returns a slog.Attr for a user agent string.
func UserAgent(ua string) slog.Attr { return slog.String(KeyUserAgent, ua) }

// RemoteAddr returns a slog.Attr for a remote address.
func RemoteAddr(a string) slog.Attr { return slog.String(KeyRemoteAddr, a) }

// RequestID returns a slog.Attr for a request ID.
func RequestID(id string) slog.Attr { return slog.String(KeyRequestID, id) }

// Status returns a slog.Attr for an HTTP status code.
func Status(code int) slog.Attr { return slog.Int(KeyStatus, code) }

// ResponseSize returns a slog.Attr for a response size in bytes.
func ResponseSize(sz int) slog.Attr { return slog.Int(KeyResponseSz, sz) }

// ForgeType returns a slog.Attr for a forge type.
func ForgeType(t string) slog.Attr { return slog.String(KeyForgeType, t) }

// ContentLength returns a slog.Attr for content length in bytes.
func ContentLength(cl int64) slog.Attr { return slog.Int64(KeyContentLen, cl) }

// Name returns a slog.Attr for a generic name field.
func Name(n string) slog.Attr { return slog.String(KeyName, n) }

// URL returns a slog.Attr for a URL field.
func URL(u string) slog.Attr { return slog.String(KeyURL, u) }

// Error returns a slog.Attr for an error, or an empty string if nil.
func Error(err error) slog.Attr {
	if err == nil {
		return slog.String(KeyError, "")
	}
	return slog.String(KeyError, err.Error())
}
