package daemon

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"os"
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

	// Verify the file exists and is a regular file
	fileInfo, err := os.Stat(cleanPath)
	if err != nil {
		if os.IsNotExist(err) {
			slog.Warn("VS Code edit handler: file not found",
				slog.String("path", cleanPath))
			http.Error(w, "File not found", http.StatusNotFound)
		} else {
			slog.Error("VS Code edit handler: failed to stat file",
				slog.String("path", cleanPath),
				slog.String("error", err.Error()))
			http.Error(w, "Failed to access file", http.StatusInternalServerError)
		}
		return
	}

	if !fileInfo.Mode().IsRegular() {
		slog.Warn("VS Code edit handler: not a regular file",
			slog.String("path", cleanPath))
		http.Error(w, "Not a regular file", http.StatusBadRequest)
		return
	}

	// Verify it's a documentation file (markdown)
	ext := strings.ToLower(filepath.Ext(cleanPath))
	if ext != ".md" && ext != ".markdown" {
		slog.Warn("VS Code edit handler: not a markdown file",
			slog.String("path", cleanPath),
			slog.String("extension", ext))
		http.Error(w, "Only markdown files can be edited", http.StatusBadRequest)
		return
	}

	// Execute 'code --goto <filepath>' to open in VS Code
	// The 'code' command works for both local and remote VS Code sessions
	// --goto flag ensures the file opens and gets focus
	// --reuse-window opens in the current VS Code window
	// Use a short timeout context to avoid hanging
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	// Find the code CLI - try bash login shell first (loads full PATH),
	// then fall back to common VS Code server locations.
	codeCmd := findCodeCLI(ctx)
	fullCommand := codeCmd + " --reuse-window --goto " + shellEscape(absPath)
	// #nosec G204 -- codeCmd comes from findCodeCLI (validated paths) and absPath is validated above
	cmd := exec.CommandContext(ctx, "bash", "-c", fullCommand)

	// Capture stdout and stderr to see what's happening
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Debug logging to see what we're executing
	slog.Debug("VS Code edit handler: executing command",
		slog.String("path", absPath),
		slog.String("command", fullCommand),
		slog.String("shell", "bash -l -c"))

	// Use Run() to wait for command completion and capture any errors
	if err := cmd.Run(); err != nil {
		slog.Error("VS Code edit handler: failed to execute code command",
			slog.String("path", absPath),
			slog.String("command", fullCommand),
			slog.String("error", err.Error()),
			slog.String("stdout", stdout.String()),
			slog.String("stderr", stderr.String()))
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

// findCodeCLI finds the VS Code CLI executable.
// Tries multiple strategies to locate the code command:
// 1. Check VS Code server locations with glob patterns (prioritize actual VS Code binaries).
// 2. Check fixed paths (/usr/local/bin/code, /usr/bin/code).
// 3. Use 'bash -l -c which code' to load full PATH.
// 4. Fall back to just 'code' and hope it's in PATH.
func findCodeCLI(parentCtx context.Context) string {
	// Common VS Code server locations in devcontainers
	// Glob patterns first (actual VS Code binaries), then fixed paths (may be wrappers)
	vscodePaths := []string{
		"/vscode/vscode-server/bin/linux-arm64/*/bin/remote-cli/code", // ARM64 architecture
		"/vscode/vscode-server/bin/linux-x64/*/bin/remote-cli/code",   // x64 architecture
		"/vscode/vscode-server/bin/*/bin/remote-cli/code",              // Any architecture
		"/usr/local/bin/code",
		"/usr/bin/code",
	}

	// Try to find code in common locations
	for _, pattern := range vscodePaths {
		if strings.Contains(pattern, "*") {
			// Glob pattern - try to expand it
			matches, err := filepath.Glob(pattern)
			if err == nil && len(matches) > 0 {
				// Use the first match and verify it's executable
				codePath := matches[0]
				if isExecutable(codePath) {
					slog.Debug("Found code CLI via glob",
						slog.String("pattern", pattern),
						slog.String("path", codePath))
					return codePath
				}
			}
		} else {
			// Direct path - check if it exists and is executable
			if isExecutable(pattern) {
				slog.Debug("Found code CLI at fixed location",
					slog.String("path", pattern))
				return pattern
			}
		}
	}

	// Try to find code via 'which' in login shell
	ctx, cancel := context.WithTimeout(parentCtx, 2*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "bash", "-l", "-c", "which code")
	output, err := cmd.Output()
	if err == nil && len(output) > 0 {
		codePath := strings.TrimSpace(string(output))
		if codePath != "" {
			slog.Debug("Found code CLI via which in login shell",
				slog.String("path", codePath))
			return codePath
		}
	}

	// Fall back to just 'code' and hope it's in PATH
	slog.Debug("Using fallback 'code' command (no explicit path found)")
	return "code"
}

// isExecutable checks if a file exists and is executable.
func isExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	// Check if it's a regular file and has execute permission
	return info.Mode().IsRegular() && (info.Mode().Perm()&0111 != 0)
}

// shellEscape escapes a file path for safe use in shell commands.
func shellEscape(path string) string {
	// Simple escaping: wrap in single quotes and escape any single quotes
	return "'" + strings.ReplaceAll(path, "'", "'\\''") + "'"
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
