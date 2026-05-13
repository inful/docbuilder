package httpserver

import (
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
)

// handleVSCodeEdit handles requests to open files in VS Code.
// URL format: /_edit/<relative-path-to-file>
// This handler opens the file in VS Code and redirects back to the referer.
func (s *Server) handleVSCodeEdit(w http.ResponseWriter, r *http.Request) {
	// Check if VS Code edit links are enabled (requires --vscode flag)
	if s.cfg == nil || !s.cfg.Build.VSCodeEditLinks {
		slog.Warn("VS Code edit handler: feature not enabled - use --vscode flag",
			slog.String("path", r.URL.Path))
		http.Error(w, "VS Code edit links not enabled. Use --vscode flag with preview command.", http.StatusNotFound)
		return
	}

	// VS Code edit handler is only for preview mode (single local repository)
	if s.cfg.Daemon != nil && s.cfg.Daemon.Storage.RepoCacheDir != "" {
		slog.Warn("VS Code edit handler called in daemon mode - this endpoint is for preview mode only",
			slog.String("path", r.URL.Path))
		http.Error(w, "VS Code edit links are only available in preview mode", http.StatusNotImplemented)
		return
	}

	// Extract and validate the file path
	absPath, err := s.validateAndResolveEditPath(r.URL.Path)
	if err != nil {
		s.handleEditError(w, err)
		return
	}

	// Execute the VS Code open command
	if err := s.executeVSCodeOpen(r.Context(), absPath); err != nil {
		s.handleEditError(w, err)
		return
	}

	slog.Info("Opened file in VS Code", slog.String("path", absPath))

	redirectTarget := safeRedirectTargetFromReferer(r)
	//nolint:gosec // redirectTarget is constrained to same-origin/relative targets by safeRedirectTargetFromReferer
	http.Redirect(w, r, redirectTarget, http.StatusSeeOther)
}

func safeRedirectTargetFromReferer(r *http.Request) string {
	redirectTarget := "/"
	referer := r.Referer()
	if referer == "" {
		return redirectTarget
	}

	u, err := url.Parse(referer)
	if err != nil {
		return redirectTarget
	}

	// Allow relative paths.
	if u.Scheme == "" && u.Host == "" {
		if strings.HasPrefix(u.Path, "/") && !strings.HasPrefix(u.Path, "//") {
			return u.RequestURI()
		}
		return redirectTarget
	}

	// Allow same-origin absolute URLs.
	if (u.Scheme == "http" || u.Scheme == "https") && strings.EqualFold(u.Host, r.Host) {
		return u.RequestURI()
	}

	return redirectTarget
}

// editError represents an error from the VS Code edit handler with an HTTP status code.
type editError struct {
	message    string
	statusCode int
	logLevel   string // logLevelWarn or logLevelError
	logFields  []any
}

func (e *editError) Error() string {
	return e.message
}

// handleEditError logs and responds with the appropriate error.
func (s *Server) handleEditError(w http.ResponseWriter, err error) {
	var editErr *editError
	if ok := errors.As(err, &editErr); ok {
		if editErr.logLevel == logLevelError {
			slog.Error("VS Code edit handler: "+editErr.message, editErr.logFields...)
		} else {
			slog.Warn("VS Code edit handler: "+editErr.message, editErr.logFields...)
		}
		http.Error(w, editErr.message, editErr.statusCode)
	} else {
		slog.Error("VS Code edit handler: unexpected error", slog.String("error", err.Error()))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}
