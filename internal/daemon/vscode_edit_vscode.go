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
	"strings"
	"time"
)

// executeVSCodeOpen finds the VS Code CLI and IPC socket, then opens the file.
func (s *HTTPServer) executeVSCodeOpen(parentCtx context.Context, absPath string) error {
	// Allow some time for transient VS Code IPC reconnects.
	ctx, cancel := context.WithTimeout(parentCtx, 10*time.Second)
	defer cancel()

	// Find the code CLI once; retries focus on IPC socket discovery/connection.
	findCLI := s.vscodeFindCLI
	if findCLI == nil {
		findCLI = findCodeCLI
	}
	codeCmd := findCLI(ctx)

	findSocket := s.vscodeFindIPCSocket
	if findSocket == nil {
		findSocket = findVSCodeIPCSocket
	}

	runCLI := s.vscodeRunCLI
	if runCLI == nil {
		runCLI = runVSCodeCLI
	}

	// Retry a few times to handle transient VS Code server/IPC disconnects.
	// This commonly happens when the remote VS Code server restarts.
	backoffs := s.vscodeOpenBackoffs
	if backoffs == nil {
		backoffs = []time.Duration{200 * time.Millisecond, 600 * time.Millisecond, 1200 * time.Millisecond}
	}
	var lastStdout, lastStderr string
	var lastErr error

	for attempt := range len(backoffs) + 1 {
		// Find VS Code IPC socket
		ipcSocket := findSocket()
		if ipcSocket == "" {
			lastErr = errors.New("ipc socket not found")
			lastStderr = ""
			lastStdout = ""
			if attempt < len(backoffs) {
				slog.Warn("VS Code edit handler: IPC socket not found, retrying",
					slog.String("path", absPath),
					slog.Int("attempt", attempt+1),
					slog.Int("max_attempts", len(backoffs)+1))
				if err := sleepWithContext(ctx, backoffs[attempt]); err == nil {
					continue
				}
			}
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

		// Security: Validate IPC socket path to prevent environment variable injection
		if err := validateIPCSocketPath(ipcSocket); err != nil {
			return &editError{
				message:    "Invalid IPC socket path",
				statusCode: http.StatusInternalServerError,
				logLevel:   "error",
				logFields: []any{
					slog.String("socket", ipcSocket),
					slog.String("error", err.Error()),
				},
			}
		}

		stdoutStr, stderrStr, err := runCLI(ctx, codeCmd, []string{"--reuse-window", "--goto", absPath}, append(os.Environ(), "VSCODE_IPC_HOOK_CLI="+ipcSocket))

		slog.Debug("VS Code edit handler: executing command",
			slog.String("path", absPath),
			slog.String("code_cli", codeCmd),
			slog.String("ipc_socket", ipcSocket),
			slog.Int("attempt", attempt+1),
			slog.Int("max_attempts", len(backoffs)+1))

		if err != nil {
			lastErr = err
			lastStdout = stdoutStr
			lastStderr = stderrStr

			if attempt < len(backoffs) && isRetriableVSCodeOpenFailure(err, lastStderr) {
				slog.Warn("VS Code edit handler: failed to open file, retrying",
					slog.String("path", absPath),
					slog.String("code_cli", codeCmd),
					slog.String("error", err.Error()),
					slog.Int("attempt", attempt+1),
					slog.Int("max_attempts", len(backoffs)+1))
				if sleepErr := sleepWithContext(ctx, backoffs[attempt]); sleepErr == nil {
					continue
				}
			}

			return &editError{
				message:    "Failed to open file in VS Code",
				statusCode: http.StatusInternalServerError,
				logLevel:   "error",
				logFields: []any{
					slog.String("path", absPath),
					slog.String("code_cli", codeCmd),
					slog.String("error", err.Error()),
					slog.String("stdout", lastStdout),
					slog.String("stderr", lastStderr),
				},
			}
		}

		return nil
	}

	return &editError{
		message:    "Failed to open file in VS Code",
		statusCode: http.StatusInternalServerError,
		logLevel:   "error",
		logFields: []any{
			slog.String("path", absPath),
			slog.String("code_cli", codeCmd),
			slog.String("error", fmt.Sprintf("%v", lastErr)),
			slog.String("stdout", lastStdout),
			slog.String("stderr", lastStderr),
		},
	}
}

func sleepWithContext(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func runVSCodeCLI(ctx context.Context, codeCmd string, args []string, env []string) (stdout string, stderr string, err error) {
	// Security: Execute directly without shell to prevent injection attacks.
	// Pass arguments as separate parameters instead of using bash -c.
	// #nosec G204 -- codeCmd comes from findCodeCLI (validated trusted paths only)
	cmd := exec.CommandContext(ctx, codeCmd, args...)
	cmd.Env = env

	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err = cmd.Run()
	return outBuf.String(), errBuf.String(), err
}

func isRetriableVSCodeOpenFailure(err error, stderr string) bool {
	// If the process was killed due to timeout/cancel, don't retry.
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	// Heuristic: VS Code remote CLI commonly reports IPC failures in stderr.
	msg := strings.ToLower(stderr)
	if msg == "" {
		// Some failures don't emit stderr, but are still transient (e.g., stale socket).
		// Retry once or twice in that case.
		return true
	}
	keywords := []string{
		"vscode-ipc",
		"ipc",
		"socket",
		"econnrefused",
		"econnreset",
		"epipe",
		"enoent",
		"timed out",
		"timeout",
		"not running",
		"could not connect",
		"connection refused",
		"connection reset",
	}
	for _, k := range keywords {
		if strings.Contains(msg, k) {
			return true
		}
	}
	return false
}

// findCodeCLI finds the VS Code CLI executable.
// Tries multiple strategies to locate the code command:
// 1. Check VS Code server locations with glob patterns (prioritize actual VS Code binaries).
// 2. Check fixed paths (/usr/local/bin/code, /usr/bin/code).
// 3. Use 'bash -l -c which code' to load full PATH.
// 4. Fall back to just 'code' and hope it's in PATH.
func findCodeCLI(parentCtx context.Context) string {
	// Allow tests (and advanced users) to explicitly override which VS Code CLI is used.
	// This is especially useful to avoid side effects (like opening files) during test runs.
	if override := os.Getenv("DOCBUILDER_VSCODE_CLI"); override != "" {
		if filepath.IsAbs(override) && isExecutable(override) {
			slog.Debug("Using VS Code CLI override", slog.String("path", override))
			return override
		}
		slog.Warn("Ignoring invalid VS Code CLI override",
			slog.String("env", "DOCBUILDER_VSCODE_CLI"),
			slog.String("path", override))
	}

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

// isExecutable checks if a file exists and is executable.
func isExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	// Check if it's a regular file and has execute permission
	return info.Mode().IsRegular() && (info.Mode().Perm()&0o111 != 0)
}
