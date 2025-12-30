package testing

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// CLITestRunner provides utilities for testing CLI commands.
type CLITestRunner struct {
	t          *testing.T
	binaryPath string
	workingDir string
	env        []string
	timeout    time.Duration
}

// NewCLITestRunner creates a new CLI test runner.
func NewCLITestRunner(t *testing.T, binaryPath string) *CLITestRunner {
	return &CLITestRunner{
		t:          t,
		binaryPath: binaryPath,
		timeout:    30 * time.Second,
	}
}

// WithWorkingDir sets the working directory for CLI commands.
func (r *CLITestRunner) WithWorkingDir(dir string) *CLITestRunner {
	r.workingDir = dir
	return r
}

// WithEnv sets environment variables for CLI commands.
func (r *CLITestRunner) WithEnv(env []string) *CLITestRunner {
	r.env = env
	return r
}

// WithTimeout sets the timeout for CLI commands.
func (r *CLITestRunner) WithTimeout(timeout time.Duration) *CLITestRunner {
	r.timeout = timeout
	return r
}

// CLIResult represents the result of a CLI command execution.
type CLIResult struct {
	ExitCode int
	Stdout   string
	Stderr   string
	Duration time.Duration
	Error    error
}

// Run executes a CLI command and returns the result.
func (r *CLITestRunner) Run(args ...string) *CLIResult {
	r.t.Helper()

	start := time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()

	// r.binaryPath and args are test-controlled inputs. This is acceptable in test harness.
	cmd := exec.CommandContext(ctx, r.binaryPath, args...) //nolint:gosec // test runner intentionally executes built binary
	if r.workingDir != "" {
		cmd.Dir = r.workingDir
	}
	if r.env != nil {
		cmd.Env = r.env
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	duration := time.Since(start)

	result := &CLIResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		Duration: duration,
		Error:    err,
	}

	if err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			result.ExitCode = exitError.ExitCode()
		} else {
			result.ExitCode = -1
		}
	} else {
		result.ExitCode = 0
	}

	return result
}

// AssertExitCode validates the exit code.
func (result *CLIResult) AssertExitCode(t *testing.T, expected int) *CLIResult {
	t.Helper()
	if result.ExitCode != expected {
		t.Errorf("Expected exit code %d, got %d\nStdout: %s\nStderr: %s",
			expected, result.ExitCode, result.Stdout, result.Stderr)
	}
	return result
}

// AssertOutputContains validates that stdout contains expected text.
func (result *CLIResult) AssertOutputContains(t *testing.T, expected string) *CLIResult {
	t.Helper()
	if !strings.Contains(result.Stdout, expected) {
		t.Errorf("Expected output to contain %q\nActual output: %s", expected, result.Stdout)
	}
	return result
}

// AssertErrorContains validates that stderr contains expected text.
func (result *CLIResult) AssertErrorContains(t *testing.T, expected string) *CLIResult {
	t.Helper()
	if !strings.Contains(result.Stderr, expected) {
		t.Errorf("Expected error output to contain %q\nActual error: %s", expected, result.Stderr)
	}
	return result
}

// AssertSuccess validates that the command succeeded (exit code 0, no error).
func (result *CLIResult) AssertSuccess(t *testing.T) *CLIResult {
	t.Helper()
	if result.ExitCode != 0 {
		t.Errorf("Command failed with exit code %d\nStdout: %s\nStderr: %s",
			result.ExitCode, result.Stdout, result.Stderr)
	}
	return result
}

// AssertFailure validates that the command failed (non-zero exit code).
func (result *CLIResult) AssertFailure(t *testing.T) *CLIResult {
	t.Helper()
	if result.ExitCode == 0 {
		t.Errorf("Expected command to fail, but it succeeded\nStdout: %s", result.Stdout)
	}
	return result
}

// MockCLIEnvironment provides a complete testing environment for CLI integration tests.
type MockCLIEnvironment struct {
	*TestEnvironment
	runner *CLITestRunner
}

// NewMockCLIEnvironment creates a new mock CLI environment.
func NewMockCLIEnvironment(t *testing.T) *MockCLIEnvironment {
	env := NewTestEnvironment(t)
	runner := NewCLITestRunner(t, "docbuilder") // Assumes binary is in PATH

	return &MockCLIEnvironment{
		TestEnvironment: env,
		runner:          runner.WithWorkingDir(env.TempDir),
	}
}

// WithBinaryPath sets the path to the DocBuilder binary.
func (env *MockCLIEnvironment) WithBinaryPath(path string) *MockCLIEnvironment {
	// Resolve to absolute path so the runner can execute it from any working dir.
	absPath, err := filepath.Abs(path)
	if err != nil {
		env.t.Logf("Failed to resolve binary path %s: %v", path, err)
		env.runner.binaryPath = path
		return env
	}

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(absPath), 0o750); err != nil {
		env.t.Logf("Failed to create parent dir for binary %s: %v", absPath, err)
		env.runner.binaryPath = absPath
		return env
	}

	// Check if binary already exists and is executable
	if stat, err := os.Stat(absPath); err == nil && stat.Mode()&0o111 != 0 {
		// Binary exists and is executable, use it
		env.t.Logf("Using existing binary at %s", absPath)
		env.runner.binaryPath = absPath
		return env
	}

	// Binary doesn't exist or isn't executable, try to build it
	buildCmd := exec.CommandContext(context.Background(), "go", "build", "-o", absPath, "git.home.luguber.info/inful/docbuilder/cmd/docbuilder") //nolint:gosec // building test binary
	buildCmd.Env = os.Environ()
	if out, err := buildCmd.CombinedOutput(); err != nil {
		env.t.Logf("Failed to build docbuilder binary at %s: %v\n%s", absPath, err, string(out))
		// If there is an existing binary, prefer it; otherwise leave path as-is.
		env.runner.binaryPath = absPath
		return env
	}

	env.t.Logf("Built docbuilder binary at %s", absPath)
	env.runner.binaryPath = absPath
	return env
}

// RunCommand executes a DocBuilder CLI command.
func (env *MockCLIEnvironment) RunCommand(args ...string) *CLIResult {
	return env.runner.Run(args...)
}

// WriteConfigFile writes the environment's config to a file.
func (env *MockCLIEnvironment) WriteConfigFile() error {
	if env.Config == nil {
		env.t.Fatal("No config set in environment")
	}

	configPath := env.ConfigPath()
	builder := &ConfigBuilder{config: env.Config, t: env.t}
	builder.BuildAndSave(configPath)
	return nil
}
