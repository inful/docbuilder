package testing

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// TestScenario provides a structured pattern for complex integration tests.
type TestScenario struct {
	Name        string
	Description string
	Setup       func(t *testing.T) *TestEnvironment
	Execute     func(t *testing.T, env *TestEnvironment) *TestResult
	Cleanup     func(t *testing.T, env *TestEnvironment)
	Timeout     time.Duration
}

// TestEnvironment encapsulates test setup and resources.
type TestEnvironment struct {
	t         *testing.T
	TempDir   string
	ConfigDir string
	OutputDir string
	Config    *config.Config
	Cancel    context.CancelFunc
	Resources map[string]any
}

// TestResult captures test execution results.
type TestResult struct {
	Success      bool
	Duration     time.Duration
	Output       string
	ErrorOutput  string
	ExitCode     int
	FilesCreated []string
	Metrics      map[string]any
}

// NewTestEnvironment creates a new test environment with temporary directories.
func NewTestEnvironment(t *testing.T) *TestEnvironment {
	tempDir := t.TempDir()

	configDir := filepath.Join(tempDir, "config")
	outputDir := filepath.Join(tempDir, "output")

	if err := os.MkdirAll(configDir, 0o750); err != nil {
		t.Fatalf("Failed to create config directory: %v", err)
	}

	if err := os.MkdirAll(outputDir, 0o750); err != nil {
		t.Fatalf("Failed to create output directory: %v", err)
	}

	_, cancel := context.WithTimeout(context.Background(), 30*time.Second)

	return &TestEnvironment{
		t:         t,
		TempDir:   tempDir,
		ConfigDir: configDir,
		OutputDir: outputDir,
		Cancel:    cancel,
		Resources: make(map[string]any),
	}
}

// Cleanup removes temporary directories and cancels context.
func (env *TestEnvironment) Cleanup() {
	if env.Cancel != nil {
		env.Cancel()
	}
	if env.TempDir != "" {
		_ = os.RemoveAll(env.TempDir) // Best effort cleanup in test teardown
	}
}

// WithConfig sets the configuration for the environment.
func (env *TestEnvironment) WithConfig(cfg *config.Config) *TestEnvironment {
	env.Config = cfg
	return env
}

// WithTimeout sets a custom timeout for the environment.
func (env *TestEnvironment) WithTimeout(timeout time.Duration) *TestEnvironment {
	if env.Cancel != nil {
		env.Cancel()
	}
	_, cancel := context.WithTimeout(context.Background(), timeout)
	env.Cancel = cancel
	return env
}

// ConfigPath returns the path to the configuration file.
func (env *TestEnvironment) ConfigPath() string {
	return filepath.Join(env.ConfigDir, "docbuilder.yaml")
}

// Run executes a test scenario.
func (scenario *TestScenario) Run(t *testing.T) {
	t.Run(scenario.Name, func(t *testing.T) {
		if scenario.Description != "" {
			t.Logf("=== %s ===", scenario.Description)
		}

		// Apply timeout if specified
		if scenario.Timeout > 0 {
			ctx, cancel := context.WithTimeout(context.Background(), scenario.Timeout)
			defer cancel()

			done := make(chan bool)
			go func() {
				scenario.runInternal(t)
				done <- true
			}()

			select {
			case <-done:
				// Test completed normally
			case <-ctx.Done():
				t.Fatalf("Test scenario %s timed out after %v", scenario.Name, scenario.Timeout)
			}
		} else {
			scenario.runInternal(t)
		}
	})
}

func (scenario *TestScenario) runInternal(t *testing.T) {
	// Setup
	var env *TestEnvironment
	if scenario.Setup != nil {
		env = scenario.Setup(t)
		if env == nil {
			t.Fatal("Setup function returned nil environment")
		}
		defer env.Cleanup()
	}

	// Execute
	var result *TestResult
	if scenario.Execute != nil {
		result = scenario.Execute(t, env)
		if result == nil {
			t.Fatal("Execute function returned nil result")
		}
	}

	// Cleanup
	if scenario.Cleanup != nil {
		scenario.Cleanup(t, env)
	}

	// Validate results
	if result != nil && !result.Success {
		t.Errorf("Test scenario failed: %s", result.ErrorOutput)
	}
}

// AssertExitCode validates the exit code in test results.
func (result *TestResult) AssertExitCode(t *testing.T, expected int) {
	t.Helper()
	if result.ExitCode != expected {
		t.Errorf("Expected exit code %d, got %d", expected, result.ExitCode)
	}
}

// AssertOutputContains validates that output contains expected text.
func (result *TestResult) AssertOutputContains(t *testing.T, expected string) {
	t.Helper()
	if result.Output == "" {
		t.Errorf("Expected output to contain %q, but output was empty", expected)
		return
	}
	// Simple string search for now - could be enhanced with regex
	found := false
	if result.Output != "" && expected != "" {
		for i := 0; i <= len(result.Output)-len(expected); i++ {
			if result.Output[i:i+len(expected)] == expected {
				found = true
				break
			}
		}
	}
	if !found {
		t.Errorf("Expected output to contain %q, but it was not found in: %s", expected, result.Output)
	}
}
