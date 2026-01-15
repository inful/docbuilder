package daemon

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// TestHandleVSCodeEdit_FeatureDisabled tests that the handler returns 404 when --vscode flag not set.
func TestHandleVSCodeEdit_FeatureDisabled(t *testing.T) {
	cfg := &config.Config{
		Build: config.BuildConfig{
			VSCodeEditLinks: false, // Feature disabled
		},
	}

	srv := &HTTPServer{config: cfg}
	req := httptest.NewRequest(http.MethodGet, "/_edit/test.md", nil)
	w := httptest.NewRecorder()

	srv.handleVSCodeEdit(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "not enabled") {
		t.Errorf("Expected 'not enabled' in response, got: %s", w.Body.String())
	}
}

// TestHandleVSCodeEdit_DaemonMode tests that the handler rejects daemon mode requests.
func TestHandleVSCodeEdit_DaemonMode(t *testing.T) {
	cfg := &config.Config{
		Build: config.BuildConfig{
			VSCodeEditLinks: true,
		},
		Daemon: &config.DaemonConfig{
			Storage: config.StorageConfig{
				RepoCacheDir: "/tmp/repos",
			},
		},
	}

	srv := &HTTPServer{config: cfg}
	req := httptest.NewRequest(http.MethodGet, "/_edit/test.md", nil)
	w := httptest.NewRecorder()

	srv.handleVSCodeEdit(w, req)

	if w.Code != http.StatusNotImplemented {
		t.Errorf("Expected status 501, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "preview mode") {
		t.Errorf("Expected 'preview mode' in response, got: %s", w.Body.String())
	}
}

// TestValidateAndResolveEditPath_InvalidPrefix tests URL prefix validation.
func TestValidateAndResolveEditPath_InvalidPrefix(t *testing.T) {
	cfg := &config.Config{
		Repositories: []config.Repository{
			{URL: "/tmp/docs"},
		},
	}

	srv := &HTTPServer{config: cfg}
	_, err := srv.validateAndResolveEditPath("/wrong/prefix/test.md")

	if err == nil {
		t.Fatal("Expected error for invalid prefix")
	}

	var editErr *editError
	if !errors.As(err, &editErr) {
		t.Fatalf("Expected editError, got: %T", err)
	}
	if editErr.statusCode != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", editErr.statusCode)
	}
}

// TestValidateAndResolveEditPath_EmptyPath tests empty path handling.
func TestValidateAndResolveEditPath_EmptyPath(t *testing.T) {
	cfg := &config.Config{
		Repositories: []config.Repository{
			{URL: "/tmp/docs"},
		},
	}

	srv := &HTTPServer{config: cfg}
	_, err := srv.validateAndResolveEditPath("/_edit/")

	if err == nil {
		t.Fatal("Expected error for empty path")
	}

	var editErr *editError
	if !errors.As(err, &editErr) {
		t.Fatalf("Expected editError, got: %T", err)
	}
	if !strings.Contains(editErr.message, "No file path") {
		t.Errorf("Expected 'No file path' message, got: %s", editErr.message)
	}
}

// TestValidateAndResolveEditPath_PathTraversal tests security against path traversal.
func TestValidateAndResolveEditPath_PathTraversal(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		Repositories: []config.Repository{
			{URL: tmpDir},
		},
	}

	srv := &HTTPServer{config: cfg}

	// Try to escape the docs directory
	_, err := srv.validateAndResolveEditPath("/_edit/../../../etc/passwd")

	if err == nil {
		t.Fatal("Expected error for path traversal attempt")
	}

	var editErr *editError
	if !errors.As(err, &editErr) {
		t.Fatalf("Expected editError, got: %T", err)
	}
	if editErr.statusCode != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", editErr.statusCode)
	}
}

// TestValidateAndResolveEditPath_Success tests successful path validation.
func TestValidateAndResolveEditPath_Success(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a test markdown file
	testFile := filepath.Join(tmpDir, "test.md")
	if err := os.WriteFile(testFile, []byte("# Test"), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		Repositories: []config.Repository{
			{URL: tmpDir},
		},
	}

	srv := &HTTPServer{config: cfg}
	absPath, err := srv.validateAndResolveEditPath("/_edit/test.md")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if absPath != testFile {
		t.Errorf("Expected path %s, got %s", testFile, absPath)
	}
}

// TestValidateMarkdownFile_NotFound tests file not found handling.
func TestValidateMarkdownFile_NotFound(t *testing.T) {
	srv := &HTTPServer{}
	err := srv.validateMarkdownFile("/nonexistent/file.md")

	if err == nil {
		t.Fatal("Expected error for nonexistent file")
	}

	var editErr *editError
	if !errors.As(err, &editErr) {
		t.Fatalf("Expected editError, got: %T", err)
	}
	if editErr.statusCode != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", editErr.statusCode)
	}
}

// TestValidateMarkdownFile_Directory tests rejection of directories.
func TestValidateMarkdownFile_Directory(t *testing.T) {
	tmpDir := t.TempDir()
	srv := &HTTPServer{}
	err := srv.validateMarkdownFile(tmpDir)

	if err == nil {
		t.Fatal("Expected error for directory")
	}

	var editErr *editError
	if !errors.As(err, &editErr) {
		t.Fatalf("Expected editError, got: %T", err)
	}
	if editErr.statusCode != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", editErr.statusCode)
	}
	if !strings.Contains(editErr.message, "Not a regular file") {
		t.Errorf("Expected 'Not a regular file', got: %s", editErr.message)
	}
}

// TestValidateMarkdownFile_NonMarkdown tests rejection of non-markdown files.
func TestValidateMarkdownFile_NonMarkdown(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0o600); err != nil {
		t.Fatal(err)
	}

	srv := &HTTPServer{}
	err := srv.validateMarkdownFile(testFile)

	if err == nil {
		t.Fatal("Expected error for non-markdown file")
	}

	var editErr *editError
	if !errors.As(err, &editErr) {
		t.Fatalf("Expected editError, got: %T", err)
	}
	if editErr.statusCode != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", editErr.statusCode)
	}
	if !strings.Contains(editErr.message, "markdown") {
		t.Errorf("Expected 'markdown' in message, got: %s", editErr.message)
	}
}

// TestValidateMarkdownFile_Success tests successful markdown validation.
func TestValidateMarkdownFile_Success(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name     string
		filename string
	}{
		{"md extension", "test.md"},
		{"markdown extension", "test.markdown"},
		{"uppercase MD", "test.MD"},
		{"mixed case", "test.Markdown"},
	}

	srv := &HTTPServer{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testFile := filepath.Join(tmpDir, tt.filename)
			if err := os.WriteFile(testFile, []byte("# Test"), 0o600); err != nil {
				t.Fatal(err)
			}

			err := srv.validateMarkdownFile(testFile)
			if err != nil {
				t.Errorf("Expected no error, got: %v", err)
			}
		})
	}
}

// TestEditError_Error tests editError.Error() method.
func TestEditError_Error(t *testing.T) {
	err := &editError{
		message:    "test error",
		statusCode: 400,
		logLevel:   "warn",
	}

	if err.Error() != "test error" {
		t.Errorf("Expected 'test error', got: %s", err.Error())
	}
}

// TestHandleEditError_EditError tests error handling for editError type.
func TestHandleEditError_EditError(t *testing.T) {
	srv := &HTTPServer{}
	w := httptest.NewRecorder()

	err := &editError{
		message:    "validation failed",
		statusCode: http.StatusBadRequest,
		logLevel:   "warn",
	}

	srv.handleEditError(w, err)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "validation failed") {
		t.Errorf("Expected 'validation failed' in response, got: %s", w.Body.String())
	}
}

// TestHandleEditError_UnexpectedError tests error handling for unknown error types.
func TestHandleEditError_UnexpectedError(t *testing.T) {
	srv := &HTTPServer{}
	w := httptest.NewRecorder()

	err := errors.New("unexpected error")

	srv.handleEditError(w, err)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", w.Code)
	}
}

// TestShellEscape tests shell escaping for file paths.
func TestShellEscape(t *testing.T) {
	t.Skip("shellEscape removed in favor of direct exec (security improvement)")
}

// TestValidateIPCSocketPath tests IPC socket path validation.
func TestValidateIPCSocketPath(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		expectErr bool
	}{
		{"valid /tmp socket", "/tmp/vscode-ipc-abc123.sock", false},
		{"valid /run/user socket", "/run/user/1000/vscode-ipc-xyz.sock", false},
		{"newline injection", "/tmp/vscode-ipc-test.sock\nMALICIOUS=value", true},
		{"carriage return", "/tmp/vscode-ipc-test.sock\r", true},
		{"null byte", "/tmp/vscode-ipc-test\x00.sock", true},
		{"wrong location", "/home/user/evil.sock", true},
		{"missing .sock extension", "/tmp/vscode-ipc-test", true},
		{"relative path", "./vscode-ipc-test.sock", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateIPCSocketPath(tt.path)
			if tt.expectErr && err == nil {
				t.Errorf("Expected error for path: %s", tt.path)
			}
			if !tt.expectErr && err != nil {
				t.Errorf("Expected no error for path: %s, got: %v", tt.path, err)
			}
		})
	}
}

// TestValidateMarkdownFile_Symlink tests symlink rejection.
func TestValidateMarkdownFile_Symlink(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a real markdown file
	realFile := filepath.Join(tmpDir, "real.md")
	if err := os.WriteFile(realFile, []byte("# Real"), 0o600); err != nil {
		t.Fatal(err)
	}

	// Create a symlink to it
	symlinkFile := filepath.Join(tmpDir, "symlink.md")
	if err := os.Symlink(realFile, symlinkFile); err != nil {
		t.Skip("Cannot create symlinks on this system")
	}

	srv := &HTTPServer{}
	err := srv.validateMarkdownFile(symlinkFile)

	if err == nil {
		t.Fatal("Expected error for symlink file")
	}

	var editErr *editError
	if !errors.As(err, &editErr) {
		t.Fatalf("Expected editError, got: %T", err)
	}
	if editErr.statusCode != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", editErr.statusCode)
	}
	if !strings.Contains(editErr.message, "Symlink") {
		t.Errorf("Expected 'Symlink' in message, got: %s", editErr.message)
	}
}

// TestIsExecutable tests executable file detection.
func TestIsExecutable(t *testing.T) {
	tmpDir := t.TempDir()

	// Create executable file
	execFile := filepath.Join(tmpDir, "executable")
	// #nosec G306 -- test file needs to be executable
	if err := os.WriteFile(execFile, []byte("#!/bin/bash"), 0o700); err != nil {
		t.Fatal(err)
	}

	// Create non-executable file
	nonExecFile := filepath.Join(tmpDir, "not-executable")
	if err := os.WriteFile(nonExecFile, []byte("test"), 0o600); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name       string
		path       string
		executable bool
	}{
		{"executable file", execFile, true},
		{"non-executable file", nonExecFile, false},
		{"nonexistent file", "/nonexistent", false},
		{"directory", tmpDir, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isExecutable(tt.path)
			if result != tt.executable {
				t.Errorf("Expected %v, got %v", tt.executable, result)
			}
		})
	}
}

// TestFileExists tests file existence checking.
func TestFileExists(t *testing.T) {
	tmpDir := t.TempDir()
	existingFile := filepath.Join(tmpDir, "exists")
	if err := os.WriteFile(existingFile, []byte("test"), 0o600); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name   string
		path   string
		exists bool
	}{
		{"existing file", existingFile, true},
		{"existing directory", tmpDir, true},
		{"nonexistent path", "/nonexistent/path", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := fileExists(tt.path)
			if result != tt.exists {
				t.Errorf("Expected %v, got %v", tt.exists, result)
			}
		})
	}
}

// TestTryPattern tests VS Code CLI path pattern matching.
func TestTryPattern(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a mock VS Code binary
	vscodeDir := filepath.Join(tmpDir, "vscode-server", "bin", "abc123", "bin", "remote-cli")
	if err := os.MkdirAll(vscodeDir, 0o750); err != nil {
		t.Fatal(err)
	}
	codeExec := filepath.Join(vscodeDir, "code")
	// #nosec G306 -- test file needs to be executable to test findCodeCLI
	if err := os.WriteFile(codeExec, []byte("#!/bin/bash"), 0o700); err != nil {
		t.Fatal(err)
	}

	// Create a non-executable file
	nonExecDir := filepath.Join(tmpDir, "bin")
	if err := os.MkdirAll(nonExecDir, 0o750); err != nil {
		t.Fatal(err)
	}
	nonExecFile := filepath.Join(nonExecDir, "code")
	if err := os.WriteFile(nonExecFile, []byte("test"), 0o600); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		pattern  string
		expected string
	}{
		{
			name:     "glob pattern finds executable",
			pattern:  filepath.Join(tmpDir, "vscode-server", "bin", "*", "bin", "remote-cli", "code"),
			expected: codeExec,
		},
		{
			name:     "direct path to executable",
			pattern:  codeExec,
			expected: codeExec,
		},
		{
			name:     "non-executable file",
			pattern:  nonExecFile,
			expected: "",
		},
		{
			name:     "nonexistent path",
			pattern:  "/nonexistent/code",
			expected: "",
		},
		{
			name:     "glob pattern no matches",
			pattern:  filepath.Join(tmpDir, "nonexistent", "*", "code"),
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tryPattern(tt.pattern)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

// TestFindCodeCLI tests VS Code CLI discovery.
func TestFindCodeCLI(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	// This test just verifies the function returns a non-empty string
	// (it always returns at least "code" as a fallback)
	result := findCodeCLI(ctx)
	if result == "" {
		t.Error("Expected non-empty code CLI path")
	}
}

// TestGetDocsDirectory tests docs directory resolution.
func TestGetDocsDirectory(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *config.Config
		expected string
	}{
		{
			name:     "nil config",
			cfg:      nil,
			expected: "",
		},
		{
			name: "no repositories",
			cfg: &config.Config{
				Repositories: []config.Repository{},
			},
			expected: "",
		},
		{
			name: "absolute path",
			cfg: &config.Config{
				Repositories: []config.Repository{
					{URL: "/absolute/path/docs"},
				},
			},
			expected: "/absolute/path/docs",
		},
		{
			name: "relative path",
			cfg: &config.Config{
				Repositories: []config.Repository{
					{URL: "docs"},
				},
			},
			expected: "", // Will be converted to absolute, can't predict exact value
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := &HTTPServer{config: tt.cfg}
			result := srv.getDocsDirectory()

			switch {
			case tt.expected == "" && result != "" && tt.name != "relative path":
				t.Errorf("Expected empty string, got %s", result)
			case tt.expected != "" && result != tt.expected:
				t.Errorf("Expected %s, got %s", tt.expected, result)
			case tt.name == "relative path" && !filepath.IsAbs(result) && result != "":
				t.Errorf("Expected absolute path for relative input, got %s", result)
			}
		})
	}
}

// TestFindVSCodeIPCSocket_Environment tests socket detection from environment variable.
func TestFindVSCodeIPCSocket_Environment(t *testing.T) {
	// Create a temporary socket file
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "vscode-ipc-test.sock")
	if err := os.WriteFile(socketPath, []byte{}, 0o600); err != nil {
		t.Fatal(err)
	}

	// Set environment variable
	t.Setenv("VSCODE_IPC_HOOK_CLI", socketPath)

	result := findVSCodeIPCSocket()
	if result != socketPath {
		t.Errorf("Expected %s, got %s", socketPath, result)
	}
}

// TestFindVSCodeIPCSocket_NoSocketsFound tests behavior when no sockets exist.
func TestFindVSCodeIPCSocket_NoSocketsFound(t *testing.T) {
	// Unset environment variable (set to empty)
	t.Setenv("VSCODE_IPC_HOOK_CLI", "")

	// This test assumes /tmp and /run/user/{uid} don't have recent vscode sockets
	// or returns the most recent one if they exist
	result := findVSCodeIPCSocket()

	// Result could be empty string or a real socket path if VS Code is running
	// We just verify it doesn't panic
	t.Logf("Found socket: %s", result)
}

// TestFindCompanionSocket tests companion socket detection.
func TestFindCompanionSocket(t *testing.T) {
	// Create a temporary socket file
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "vscode-git.sock")
	if err := os.WriteFile(socketPath, []byte{}, 0o600); err != nil {
		t.Fatal(err)
	}

	// Set environment variable
	t.Setenv("VSCODE_GIT_IPC_HANDLE", socketPath)

	result := findCompanionSocket()
	if result != socketPath {
		t.Errorf("Expected %s, got %s", socketPath, result)
	}
}

// TestFindIPCSocketByCompanion tests socket matching by companion timestamp.
func TestFindIPCSocketByCompanion(t *testing.T) {
	tmpDir := t.TempDir()

	// Create companion socket
	companionPath := filepath.Join(tmpDir, "vscode-companion.sock")
	if err := os.WriteFile(companionPath, []byte{}, 0o600); err != nil {
		t.Fatal(err)
	}

	// Create IPC socket with similar timestamp (within 1 second)
	time.Sleep(100 * time.Millisecond)
	ipcPath := filepath.Join(tmpDir, "vscode-ipc-test.sock")
	if err := os.WriteFile(ipcPath, []byte{}, 0o600); err != nil {
		t.Fatal(err)
	}

	// Note: This test may not find sockets since it only searches /tmp and /run/user/{uid}
	// But it verifies the function doesn't panic
	result := findIPCSocketByCompanion(companionPath)
	t.Logf("Found IPC socket: %s", result)
}

// TestFindMostRecentIPCSocket tests fallback socket selection.
func TestFindMostRecentIPCSocket(t *testing.T) {
	// This test just verifies the function doesn't panic
	// Actual socket detection depends on the system state
	result := findMostRecentIPCSocket()
	t.Logf("Found most recent socket: %s", result)
}

// TestHandleVSCodeEdit_Integration tests the full handler flow with a valid markdown file.
func TestHandleVSCodeEdit_Integration(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a test markdown file
	testFile := filepath.Join(tmpDir, "test.md")
	if err := os.WriteFile(testFile, []byte("# Test"), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		Build: config.BuildConfig{
			VSCodeEditLinks: true,
		},
		Repositories: []config.Repository{
			{URL: tmpDir},
		},
	}

	srv := &HTTPServer{config: cfg}
	req := httptest.NewRequest(http.MethodGet, "/_edit/test.md", nil)
	req.Header.Set("Referer", "http://localhost:1314/docs/")
	w := httptest.NewRecorder()

	srv.handleVSCodeEdit(w, req)

	// If VS Code is running, we get 303 redirect (success)
	// If VS Code is not running, we get 503 (service unavailable)
	if w.Code != http.StatusSeeOther && w.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected 303 (success) or 503 (no VS Code), got %d: %s", w.Code, w.Body.String())
	}
}

// TestExecuteVSCodeOpen_NoSocket tests execution behavior (may find socket if VS Code running).
func TestExecuteVSCodeOpen_NoSocket(t *testing.T) {
	srv := &HTTPServer{}
	err := srv.executeVSCodeOpen(t.Context(), "/tmp/nonexistent.md")

	// If VS Code is running, we might get a different error (file execution)
	// If VS Code is not running, we get socket not found error
	if err == nil {
		t.Log("VS Code command succeeded (VS Code is running)")
		return
	}

	t.Logf("Got expected error (VS Code not running or file issues): %v", err)
}
