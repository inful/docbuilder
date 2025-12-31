package errors

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// HTTPErrorAdapter handles error presentation and status code determination for HTTP applications.
type HTTPErrorAdapter struct {
	logger *slog.Logger
}

// NewHTTPErrorAdapter creates a new HTTP error adapter with an optional slog logger.
// If logger is nil, the default package logger will be used.
func NewHTTPErrorAdapter(logger *slog.Logger) *HTTPErrorAdapter {
	if logger == nil {
		logger = slog.Default()
	}
	return &HTTPErrorAdapter{logger: logger}
}

// errorResponse represents a standard JSON error payload.
type HTTPErrorResponse struct {
	Error     string         `json:"error"`
	Code      string         `json:"code,omitempty"`
	Details   map[string]any `json:"details,omitempty"`
	Retryable bool           `json:"retryable,omitempty"`
}

// StatusCodeFor determines the HTTP status code for a given error based on
// its classification. Unknown errors map to 500.
func (a *HTTPErrorAdapter) StatusCodeFor(err error) int {
	if err == nil {
		return http.StatusOK
	}

	// Classified errors from the foundation package
	if c, ok := AsClassified(err); ok {
		switch c.Category() {
		case CategoryValidation, CategoryConfig:
			return http.StatusBadRequest
		case CategoryAuth:
			return http.StatusUnauthorized
		case CategoryNotFound:
			return http.StatusNotFound
		case CategoryAlreadyExists:
			return http.StatusConflict
		case CategoryNetwork, CategoryGit, CategoryForge:
			return http.StatusBadGateway
		case CategoryBuild, CategoryHugo:
			return http.StatusUnprocessableEntity
		case CategoryFileSystem:
			return http.StatusInternalServerError
		case CategoryRuntime, CategoryDaemon:
			return http.StatusServiceUnavailable
		case CategoryInternal:
			return http.StatusInternalServerError
		default:
			return http.StatusInternalServerError
		}
	}

	// Fallback
	return http.StatusInternalServerError
}

// WriteErrorResponse writes a JSON error response and logs with appropriate level.
func (a *HTTPErrorAdapter) WriteErrorResponse(w http.ResponseWriter, r *http.Request, err error) {
	if err == nil {
		w.WriteHeader(http.StatusOK)
		return
	}

	ctx := r.Context()
	_ = ctx // Use context for future enhancements (tracing, etc.)

	status := a.StatusCodeFor(err)
	payload := a.FormatErrorResponse(err)

	b, jerr := json.Marshal(payload)
	if jerr != nil {
		// Fall back to a minimal message
		w.WriteHeader(status)
		_, _ = w.Write([]byte("{\"error\":\"internal error\"}"))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(b)

	// Structured logging by severity
	if c, ok := AsClassified(err); ok {
		lvl := a.slogLevelFromSeverity(c.Severity())
		a.logger.Log(r.Context(), lvl, c.Error())
		return
	}
	// Unknown error
	a.logger.Error(err.Error())
}

// FormatErrorResponse converts known errors into a canonical error payload.
func (a *HTTPErrorAdapter) FormatErrorResponse(err error) HTTPErrorResponse {
	if err == nil {
		return HTTPErrorResponse{Error: ""}
	}
	if c, ok := AsClassified(err); ok {
		resp := HTTPErrorResponse{Error: c.Message(), Code: string(c.Category())}
		if len(c.Context()) > 0 {
			resp.Details = map[string]any(c.Context())
		}
		if c.RetryStrategy() != RetryNever {
			resp.Retryable = true
			if resp.Details == nil {
				resp.Details = make(map[string]any)
			}
			resp.Details["retryable"] = true
		}
		return resp
	}
	return HTTPErrorResponse{Error: err.Error()}
}

// Helper: map severities.
func (a *HTTPErrorAdapter) slogLevelFromSeverity(s ErrorSeverity) slog.Level {
	switch s {
	case SeverityInfo:
		return slog.LevelInfo
	case SeverityWarning:
		return slog.LevelWarn
	case SeverityError:
		return slog.LevelError
	case SeverityFatal:
		return slog.LevelError
	default:
		return slog.LevelError
	}
}
