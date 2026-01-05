package daemon

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
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

	// VS Code edit handler is only for preview mode (single local repository)
	// In daemon mode, edit URLs point to forge web interfaces
	if s.config.Daemon != nil && s.config.Daemon.Storage.RepoCacheDir != "" {
		slog.Warn("VS Code edit handler called in daemon mode - this endpoint is for preview mode only",
			slog.String("path", relPath))
		http.Error(w, "VS Code edit links are only available in preview mode", http.StatusNotImplemented)
		return
	}

	// Preview mode: direct path relative to docs dir
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

	// Find VS Code IPC socket - check environment first, then search filesystem
	ipcSocket := findVSCodeIPCSocket()
	if ipcSocket == "" {
		slog.Warn("VS Code edit handler: could not find VS Code IPC socket",
			slog.String("path", absPath),
			slog.String("hint", "Ensure VS Code is running and connected via remote SSH"))
		http.Error(w, "VS Code IPC socket not found - is VS Code running?", http.StatusServiceUnavailable)
		return
	}

	// Find the code CLI - try bash login shell first (loads full PATH),
	// then fall back to common VS Code server locations.
	codeCmd := findCodeCLI(ctx)
	fullCommand := codeCmd + " --reuse-window --goto " + shellEscape(absPath)
	// #nosec G204 -- codeCmd comes from findCodeCLI (validated paths) and absPath is validated above
	cmd := exec.CommandContext(ctx, "bash", "-c", fullCommand)

	// Set VS Code IPC socket environment variable
	cmd.Env = append(os.Environ(),
		"VSCODE_IPC_HOOK_CLI="+ipcSocket,
	)

	// Capture stdout and stderr to see what's happening
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Debug logging to see what we're executing
	slog.Debug("VS Code edit handler: executing command",
		slog.String("path", absPath),
		slog.String("code_cli", codeCmd),
		slog.String("command", fullCommand),
		slog.String("ipc_socket", ipcSocket))

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
		"/vscode/vscode-server/bin/*/bin/remote-cli/code",             // Any architecture
		"/usr/local/bin/code",
		"/usr/bin/code",
	}

	// Try to find code in common locations
	for _, pattern := range vscodePaths {
		if codePath := tryPattern(pattern); codePath != "" {
			return codePath
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

// tryPattern attempts to find an executable VS Code CLI at the given pattern.
// Returns the path if found and executable, empty string otherwise.
func tryPattern(pattern string) string {
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
	} else if isExecutable(pattern) {
		// Direct path - check if it exists and is executable
		slog.Debug("Found code CLI at fixed location",
			slog.String("path", pattern))
		return pattern
	}
	return ""
}

// fileExists checks if a file or socket exists at the given path.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// isExecutable checks if a file exists and is executable.
func isExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	// Check if it's a regular file and has execute permission
	return info.Mode().IsRegular() && (info.Mode().Perm()&0o111 != 0)
}

// shellEscape escapes a file path for safe use in shell commands.
func shellEscape(path string) string {
	// Simple escaping: wrap in single quotes and escape any single quotes
	return "'" + strings.ReplaceAll(path, "'", "'\\''") + "'"
}

// findVSCodeIPCSocket locates the VS Code IPC socket for remote CLI communication.
// It first checks the VSCODE_IPC_HOOK_CLI environment variable (most reliable),
// then searches for the most recently modified open socket in /tmp and /run/user/{uid}/.
//
// Based on the approach from code-connect: https://github.com/chvolkmann/code-connect
func findVSCodeIPCSocket() string {
	// Primary: Check if the environment variable is set
	// This is the most reliable method when VS Code has initialized the terminal
	if ipcSocket := os.Getenv("VSCODE_IPC_HOOK_CLI"); ipcSocket != "" {
		// Trust the environment variable - it's set by VS Code itself
		// Even if we can't connect now, it's the correct socket to use
		if fileExists(ipcSocket) {
			slog.Debug("Found VS Code IPC socket from environment",
				slog.String("socket", ipcSocket))
			return ipcSocket
		}
		slog.Warn("Environment IPC socket does not exist, searching filesystem",
			slog.String("socket", ipcSocket))
	}

	// Search for IPC sockets in multiple locations
	// VS Code may store sockets in /tmp or /run/user/{uid}/ depending on the environment
	uid := os.Getuid()
	searchPaths := []string{
		"/tmp/vscode-ipc-*.sock",
		filepath.Join(fmt.Sprintf("/run/user/%d", uid), "vscode-ipc-*.sock"),
	}

	var allMatches []string
	for _, pattern := range searchPaths {
		matches, err := filepath.Glob(pattern)
		if err == nil && len(matches) > 0 {
			allMatches = append(allMatches, matches...)
			slog.Debug("Found VS Code IPC socket candidates",
				slog.String("pattern", pattern),
				slog.Int("count", len(matches)))
		}
	}

	if len(allMatches) == 0 {
		slog.Debug("No VS Code IPC sockets found in any location",
			slog.Any("searched", searchPaths),
			slog.Int("uid", uid))
		return ""
	}

	// Fallback: Search filesystem for most recently modified socket
	// Sort by modification time (most recent first) - the active socket will be
	// the one that was most recently touched by VS Code
	type socketInfo struct {
		path    string
		modTime time.Time
	}

	sockets := make([]socketInfo, 0, len(allMatches))
	maxIdleTime := 4 * time.Hour // Same as code-connect default
	now := time.Now()

	for _, sockPath := range allMatches {
		info, err := os.Stat(sockPath)
		if err != nil {
			continue
		}

		// Only consider recently modified sockets (active VS Code sessions)
		modTime := info.ModTime()
		if now.Sub(modTime) > maxIdleTime {
			slog.Debug("Skipping stale IPC socket",
				slog.String("socket", sockPath),
				slog.Duration("idle", now.Sub(modTime)))
			continue
		}

		sockets = append(sockets, socketInfo{
			path:    sockPath,
			modTime: modTime,
		})
	}

	// Sort by modification time, most recent first
	sort.Slice(sockets, func(i, j int) bool {
		return sockets[i].modTime.After(sockets[j].modTime)
	})

	// Return the most recently modified socket
	// This is likely the active VS Code instance
	if len(sockets) > 0 {
		selected := sockets[0]
		slog.Debug("Selected most recent IPC socket",
			slog.String("socket", selected.path),
			slog.Time("modified", selected.modTime),
			slog.Int("total_candidates", len(sockets)))
		return selected.path
	}

	slog.Warn("No open VS Code IPC sockets found",
		slog.Int("total_candidates", len(allMatches)),
		slog.Int("recent_candidates", len(sockets)))
	return ""
}

// getDocsDirectory returns the docs directory for preview mode edit operations.
// VS Code edit links are only supported in preview mode, not daemon mode.
func (s *HTTPServer) getDocsDirectory() string {
	if s.config == nil || len(s.config.Repositories) == 0 {
		return ""
	}

	// In preview mode (single repository), the repository URL is the local docs directory
	docsDir := s.config.Repositories[0].URL

	// Ensure absolute path
	if !filepath.IsAbs(docsDir) {
		if abs, err := filepath.Abs(docsDir); err == nil {
			return abs
		}
	}

	slog.Debug("VS Code edit handler: using repository URL as docs dir",
		slog.String("docs_dir", docsDir))
	return docsDir
}
