package httpserver

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// validateIPCSocketPath validates that an IPC socket path is safe to use.
// This prevents environment variable injection and ensures the path is from expected locations.
func validateIPCSocketPath(socketPath string) error {
	// Reject paths with newlines or other control characters that could inject env vars
	if strings.ContainsAny(socketPath, "\n\r\x00") {
		return errors.New("socket path contains invalid characters")
	}

	// Reject relative paths
	if !filepath.IsAbs(socketPath) {
		return errors.New("socket path must be absolute")
	}

	// Ensure socket path is from expected VS Code locations
	if !strings.HasPrefix(socketPath, "/tmp/vscode-ipc-") &&
		!strings.Contains(socketPath, "/run/user/") &&
		!strings.Contains(socketPath, "/vscode-ipc-") {
		return errors.New("socket path not from expected VS Code location")
	}

	// Ensure it has .sock extension
	if !strings.HasSuffix(socketPath, ".sock") {
		return errors.New("socket path must end with .sock")
	}

	return nil
}

// fileExists checks if a file or socket exists at the given path.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
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
