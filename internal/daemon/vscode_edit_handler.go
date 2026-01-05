package daemon

import (
	"bytes"
	"context"
	"errors"
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

// validateAndResolveEditPath extracts the file path from the URL and validates it.
func (s *HTTPServer) validateAndResolveEditPath(urlPath string) (string, error) {
	// Extract file path from URL
	const editPrefix = "/_edit/"
	if !strings.HasPrefix(urlPath, editPrefix) {
		return "", &editError{
			message:    "Invalid edit URL",
			statusCode: http.StatusBadRequest,
			logLevel:   "warn",
		}
	}

	relPath := strings.TrimPrefix(urlPath, editPrefix)
	if relPath == "" {
		return "", &editError{
			message:    "No file path specified",
			statusCode: http.StatusBadRequest,
			logLevel:   "warn",
		}
	}

	// Get docs directory
	docsDir := s.getDocsDirectory()
	if docsDir == "" {
		return "", &editError{
			message:    "Server configuration error",
			statusCode: http.StatusInternalServerError,
			logLevel:   "error",
			logFields:  []any{slog.String("reason", "unable to determine docs directory")},
		}
	}

	// Resolve to absolute path
	absPath := filepath.Join(docsDir, relPath)

	// Security: ensure the resolved path is within the docs directory
	cleanDocs := filepath.Clean(docsDir)
	cleanPath := filepath.Clean(absPath)
	if !strings.HasPrefix(cleanPath, cleanDocs) {
		return "", &editError{
			message:    "Invalid file path",
			statusCode: http.StatusBadRequest,
			logLevel:   "warn",
			logFields: []any{
				slog.String("requested", relPath),
				slog.String("resolved", cleanPath),
				slog.String("docs_dir", cleanDocs),
			},
		}
	}

	// Validate file exists and is a markdown file
	if err := s.validateMarkdownFile(cleanPath); err != nil {
		return "", err
	}

	return cleanPath, nil
}

// validateMarkdownFile checks that the file exists, is regular, and is markdown.
func (s *HTTPServer) validateMarkdownFile(path string) error {
	fileInfo, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &editError{
				message:    "File not found",
				statusCode: http.StatusNotFound,
				logLevel:   "warn",
				logFields:  []any{slog.String("path", path)},
			}
		}
		return &editError{
			message:    "Failed to access file",
			statusCode: http.StatusInternalServerError,
			logLevel:   "error",
			logFields: []any{
				slog.String("path", path),
				slog.String("error", err.Error()),
			},
		}
	}

	if !fileInfo.Mode().IsRegular() {
		return &editError{
			message:    "Not a regular file",
			statusCode: http.StatusBadRequest,
			logLevel:   "warn",
			logFields:  []any{slog.String("path", path)},
		}
	}

	// Verify it's a markdown file
	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".md" && ext != ".markdown" {
		return &editError{
			message:    "Only markdown files can be edited",
			statusCode: http.StatusBadRequest,
			logLevel:   "warn",
			logFields: []any{
				slog.String("path", path),
				slog.String("extension", ext),
			},
		}
	}

	return nil
}

// executeVSCodeOpen finds the VS Code CLI and IPC socket, then opens the file.
func (s *HTTPServer) executeVSCodeOpen(parentCtx context.Context, absPath string) error {
	ctx, cancel := context.WithTimeout(parentCtx, 5*time.Second)
	defer cancel()

	// Find VS Code IPC socket
	ipcSocket := findVSCodeIPCSocket()
	if ipcSocket == "" {
		return &editError{
			message:    "VS Code IPC socket not found - is VS Code running?",
			statusCode: http.StatusServiceUnavailable,
			logLevel:   "warn",
			logFields: []any{
				slog.String("path", absPath),
				slog.String("hint", "Ensure VS Code is running and connected via remote SSH"),
			},
		}
	}

	// Find the code CLI
	codeCmd := findCodeCLI(ctx)
	fullCommand := codeCmd + " --reuse-window --goto " + shellEscape(absPath)

	// Execute command
	// #nosec G204 -- codeCmd comes from findCodeCLI (validated paths) and absPath is validated above
	cmd := exec.CommandContext(ctx, "bash", "-c", fullCommand)
	cmd.Env = append(os.Environ(), "VSCODE_IPC_HOOK_CLI="+ipcSocket)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	slog.Debug("VS Code edit handler: executing command",
		slog.String("path", absPath),
		slog.String("code_cli", codeCmd),
		slog.String("command", fullCommand),
		slog.String("ipc_socket", ipcSocket))

	if err := cmd.Run(); err != nil {
		return &editError{
			message:    "Failed to open file in VS Code",
			statusCode: http.StatusInternalServerError,
			logLevel:   "error",
			logFields: []any{
				slog.String("path", absPath),
				slog.String("command", fullCommand),
				slog.String("error", err.Error()),
				slog.String("stdout", stdout.String()),
				slog.String("stderr", stderr.String()),
			},
		}
	}

	return nil
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
// It uses multiple strategies to find the correct socket when VSCODE_IPC_HOOK_CLI is not set:
// 1. Check environment variable (most reliable when set)
// 2. Look for companion VS Code sockets (git, containers) to identify the active session
// 3. Use the most recently modified socket as fallback
//
// Based on the approach from code-connect: https://github.com/chvolkmann/code-connect
func findVSCodeIPCSocket() string {
	// Primary: Check if the environment variable is set
	// This is the most reliable method when VS Code has initialized the terminal
	if ipcSocket := os.Getenv("VSCODE_IPC_HOOK_CLI"); ipcSocket != "" {
		// Trust the environment variable - it's set by VS Code itself
		if fileExists(ipcSocket) {
			slog.Debug("Found VS Code IPC socket from environment",
				slog.String("socket", ipcSocket))
			return ipcSocket
		}
		slog.Warn("Environment IPC socket does not exist, searching filesystem",
			slog.String("socket", ipcSocket))
	}

	// Secondary: Look for companion VS Code sockets to identify the active session
	// When VS Code starts, it creates multiple related sockets (git, containers, ssh-auth)
	// These can help us identify which IPC socket belongs to the current session
	companionSocket := findCompanionSocket()
	if companionSocket != "" {
		// Try to match IPC sockets by proximity in time to companion socket
		if ipcSocket := findIPCSocketByCompanion(companionSocket); ipcSocket != "" {
			slog.Debug("Found VS Code IPC socket via companion match",
				slog.String("socket", ipcSocket),
				slog.String("companion", companionSocket))
			return ipcSocket
		}
	}

	// Fallback: Search for IPC sockets and select most recently modified
	return findMostRecentIPCSocket()
}

// findCompanionSocket looks for other VS Code sockets that can help identify the active session.
func findCompanionSocket() string {
	// Check for other VS Code environment variables that point to sockets
	companionEnvVars := []string{
		"VSCODE_GIT_IPC_HANDLE",
		"REMOTE_CONTAINERS_IPC",
		"SSH_AUTH_SOCK", // May be VS Code managed
	}

	for _, envVar := range companionEnvVars {
		if sockPath := os.Getenv(envVar); sockPath != "" {
			if fileExists(sockPath) && strings.Contains(sockPath, "vscode") {
				slog.Debug("Found companion VS Code socket",
					slog.String("env_var", envVar),
					slog.String("socket", sockPath))
				return sockPath
			}
		}
	}
	return ""
}

// findIPCSocketByCompanion finds an IPC socket that was created around the same time as a companion socket.
func findIPCSocketByCompanion(companionPath string) string {
	companionInfo, err := os.Stat(companionPath)
	if err != nil {
		return ""
	}
	companionTime := companionInfo.ModTime()

	// Search for IPC sockets
	uid := os.Getuid()
	searchPaths := []string{
		"/tmp/vscode-ipc-*.sock",
		filepath.Join(fmt.Sprintf("/run/user/%d", uid), "vscode-ipc-*.sock"),
	}

	var candidates []struct {
		path     string
		modTime  time.Time
		timeDiff time.Duration
	}

	for _, pattern := range searchPaths {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			continue
		}

		for _, sockPath := range matches {
			info, err := os.Stat(sockPath)
			if err != nil {
				continue
			}

			modTime := info.ModTime()
			timeDiff := companionTime.Sub(modTime)
			if timeDiff < 0 {
				timeDiff = -timeDiff
			}

			// Consider sockets created within 10 seconds of the companion
			if timeDiff <= 10*time.Second {
				candidates = append(candidates, struct {
					path     string
					modTime  time.Time
					timeDiff time.Duration
				}{sockPath, modTime, timeDiff})
			}
		}
	}

	// Return the socket with the smallest time difference
	if len(candidates) > 0 {
		sort.Slice(candidates, func(i, j int) bool {
			return candidates[i].timeDiff < candidates[j].timeDiff
		})
		selected := candidates[0]
		slog.Debug("Matched IPC socket to companion by time",
			slog.String("socket", selected.path),
			slog.Time("modified", selected.modTime),
			slog.Duration("time_diff", selected.timeDiff))
		return selected.path
	}

	return ""
}

// findMostRecentIPCSocket searches for IPC sockets and returns the most recently modified one.
func findMostRecentIPCSocket() string {
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
