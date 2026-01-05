package daemon

import (
	"context"
	"log/slog"
	"net/http"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// handleVSCodeEdit handles requests to open files in VS Code.
// URL format: /_edit/<relative-path-to-file>
// This handler opens the file in VS Code and redirects back to the referer.
func (s *HTTPServer) handleVSCodeEdit(w http.ResponseWriter, r *http.Request) {
	// Extract file path from URL (remove /_edit/ prefix)
	const editPrefix = "/_edit/"
	if !strings.HasPrefix(r.URL.Path, editPrefix) {
		http.Error(w, "Invalid edit URL", http.StatusBadRequest)
		return
	}

	relPath := strings.TrimPrefix(r.URL.Path, editPrefix)
	if relPath == "" {
		http.Error(w, "No file path specified", http.StatusBadRequest)
		return
	}

	// Resolve to absolute path relative to docs directory
	docsDir := s.getDocsDirectory()
	if docsDir == "" {
		slog.Error("VS Code edit handler: unable to determine docs directory")
		http.Error(w, "Server configuration error", http.StatusInternalServerError)
		return
	}

	absPath := filepath.Join(docsDir, relPath)

	// Security: ensure the resolved path is within the docs directory
	cleanDocs := filepath.Clean(docsDir)
	cleanPath := filepath.Clean(absPath)
	if !strings.HasPrefix(cleanPath, cleanDocs) {
		slog.Warn("VS Code edit handler: attempted path traversal",
			slog.String("requested", relPath),
			slog.String("resolved", cleanPath),
			slog.String("docs_dir", cleanDocs))
		http.Error(w, "Invalid file path", http.StatusBadRequest)
		return
	}

	// Execute 'code --goto <filepath>' to open in VS Code
	// The 'code' command works for both local and remote VS Code sessions
	// --goto flag ensures the file opens and gets focus
	// --reuse-window opens in the current VS Code window
	// Use a short timeout context to avoid hanging
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	// #nosec G204 -- absPath is validated and sanitized above (path traversal check)
	cmd := exec.CommandContext(ctx, "code", "--reuse-window", "--goto", absPath)
	
	// Use Run() to wait for command completion and capture any errors
	if err := cmd.Run(); err != nil {
		slog.Error("VS Code edit handler: failed to execute code command",
			slog.String("path", absPath),
			slog.String("error", err.Error()))
		http.Error(w, "Failed to open file in VS Code", http.StatusInternalServerError)
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

// getDocsDirectory returns the docs directory for local preview mode.
// For preview mode, this is the repository URL (which is a local path).
func (s *HTTPServer) getDocsDirectory() string {
	if s.config == nil || len(s.config.Repositories) == 0 {
		return ""
	}

	// In preview mode, the repository URL is the local docs directory
	docsDir := s.config.Repositories[0].URL

	// Ensure absolute path
	if !filepath.IsAbs(docsDir) {
		if abs, err := filepath.Abs(docsDir); err == nil {
			return abs
		}
	}

	return docsDir
}
