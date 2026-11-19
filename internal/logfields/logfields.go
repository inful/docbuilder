// Package logfields provides canonical log field names and helpers for structured logging in DocBuilder.
package logfields

import "log/slog"

// Canonical log field name constants to avoid drift across packages.
// These are used for structured logging with slog.
const (
	KeyJobID      = "job_id"
	KeyRepo       = "repository"
	KeySection    = "section"
	KeyError      = "error"
	KeyPath       = "path"
	KeyFile       = "file"
	KeyWorker     = "worker"
	KeyMethod     = "method"
	KeyUserAgent  = "user_agent"
	KeyRemoteAddr = "remote_addr"
	KeyRequestID  = "request_id"
	KeyStatus     = "status"
	KeyResponseSz = "response_size"
	KeyForgeType  = "forge_type"
	KeyContentLen = "content_length"
	KeyName       = "name"
	KeyURL        = "url"
)

// The following helpers return slog.Attr for common log fields, allowing composable structured logging.

func JobID(id string) slog.Attr     { return slog.String(KeyJobID, id) }  // JobID returns a slog.Attr for the job ID field.
func Repository(r string) slog.Attr { return slog.String(KeyRepo, r) }    // Repository returns a slog.Attr for the repository name field.
func Section(s string) slog.Attr    { return slog.String(KeySection, s) } // Section returns a slog.Attr for the section name field.

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
