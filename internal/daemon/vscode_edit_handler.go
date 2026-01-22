package daemon

import (
	"errors"
	"log/slog"
	"net/http"
)

// handleVSCodeEdit handles requests to open files in VS Code.
// URL format: /_edit/<relative-path-to-file>
// This handler opens the file in VS Code and redirects back to the referer.
func (s *HTTPServer) handleVSCodeEdit(w http.ResponseWriter, r *http.Request) {
	// Check if VS Code edit links are enabled (requires --vscode flag)
	if s.config == nil || !s.config.Build.VSCodeEditLinks {
		slog.Warn("VS Code edit handler: feature not enabled - use --vscode flag",
			slog.String("path", r.URL.Path))
		http.Error(w, "VS Code edit links not enabled. Use --vscode flag with preview command.", http.StatusNotFound)
		return
	}

	// VS Code edit handler is only for preview mode (single local repository)
	if s.config.Daemon != nil && s.config.Daemon.Storage.RepoCacheDir != "" {
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

	// Redirect back to the referer, or to home if no referer
	referer := r.Referer()
	if referer == "" {
		referer = "/"
	}
	http.Redirect(w, r, referer, http.StatusSeeOther)
}

// editError represents an error from the VS Code edit handler with an HTTP status code.
type editError struct {
	message    string
	statusCode int
	logLevel   string // "warn" or "error"
	logFields  []any
}

func (e *editError) Error() string {
	return e.message
}

// handleEditError logs and responds with the appropriate error.
func (s *HTTPServer) handleEditError(w http.ResponseWriter, err error) {
	var editErr *editError
	if ok := errors.As(err, &editErr); ok {
		if editErr.logLevel == "error" {
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
