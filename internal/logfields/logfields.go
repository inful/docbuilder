// Package logfields provides canonical log field names and helpers for structured logging in DocBuilder.
package logfields

import "log/slog"

// Canonical log field name constants to avoid drift across packages.
// These are used for structured logging with slog.
const (
	KeyJobID      = "job_id"
	KeyJobType    = "job_type"
	KeyStage      = "stage"
	KeyDurationMS = "duration_ms"
	KeySchedule   = "schedule_name"
	KeyRepo       = "repository"
	KeySection    = "section"
	KeyError      = "error"
	KeyPath       = "path"
	KeyFile       = "file"
	KeyMethod     = "method"
	KeyUserAgent  = "user_agent"
	KeyRemoteAddr = "remote_addr"
	KeyRequestID  = "request_id"
	KeyStatus     = "status"
	KeyName       = "name"
	KeyURL        = "url"
)

// JobID returns a slog.Attr for the job ID field.
func JobID(id string) slog.Attr { return slog.String(KeyJobID, id) }

// Repository returns a slog.Attr for repository name.
func Repository(r string) slog.Attr { return slog.String(KeyRepo, r) }

// Section returns a slog.Attr for section name.
func Section(s string) slog.Attr { return slog.String(KeySection, s) }

// Path returns a slog.Attr for a file path.
func Path(p string) slog.Attr { return slog.String(KeyPath, p) }

// File returns a slog.Attr for a file name.
func File(f string) slog.Attr { return slog.String(KeyFile, f) }

// Method returns a slog.Attr for an HTTP method.
func Method(m string) slog.Attr { return slog.String(KeyMethod, m) }

// UserAgent returns a slog.Attr for a user agent string.
func UserAgent(ua string) slog.Attr { return slog.String(KeyUserAgent, ua) }

// RemoteAddr returns a slog.Attr for a remote address.
func RemoteAddr(a string) slog.Attr { return slog.String(KeyRemoteAddr, a) }

// Status returns a slog.Attr for an HTTP status code.
func Status(code int) slog.Attr { return slog.Int(KeyStatus, code) }

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
