package httpserver

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
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

	// The docs port is reachable from the network: the preview server keeps
	// it open so a developer can look at the site from another machine.
	// Opening a file in the editor runs a command here, so the request has
	// to come from the machine itself.
	if !isLoopbackRequest(r) {
		slog.Warn("VS Code edit handler: request from another host refused",
			slog.String("path", r.URL.Path),
			slog.String("remote_addr", r.RemoteAddr))
		// The address of the request is in the answer: the page in the
		// browser is what the developer sees, and the address says
		// where the request came from.
		http.Error(w, "VS Code edit links are only available from the machine that runs the preview server\n\n"+
			formatEditErrorDetails([]any{
				slog.String("remote_addr", r.RemoteAddr),
				slog.String("path", r.URL.Path),
				slog.String("hint", "the request has to come from the machine that runs the preview"),
			}), http.StatusForbidden)
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

// isLoopbackRequest reports whether the request came from this machine.
//
// The peer address is read from the connection, not from a header: a
// header can be set by whoever sends the request.
func isLoopbackRequest(r *http.Request) bool {
	if r == nil {
		return false
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	return ip != nil && ip.IsLoopback()
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
		// The details are part of the answer: the page in the browser
		// is what the developer sees, and the message alone ("the IPC
		// socket was not found") does not say which one.
		http.Error(w, editErr.message+"\n\n"+formatEditErrorDetails(editErr.logFields), editErr.statusCode)
	} else {
		slog.Error("VS Code edit handler: unexpected error", slog.String("error", err.Error()))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// formatEditErrorDetails renders the details of a failed edit for the
// page in the browser.
func formatEditErrorDetails(fields []any) string {
	if len(fields) == 0 {
		return ""
	}

	var details []string
	for _, field := range fields {
		attr, ok := field.(slog.Attr)
		if !ok {
			details = append(details, fmt.Sprintf("%v", field))
			continue
		}
		details = append(details, fmt.Sprintf("%s: %s", attr.Key, attr.Value.String()))
	}

	return strings.Join(details, "\n")
}
