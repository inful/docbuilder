package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

func TestCommandExecutorService(t *testing.T) {
	// Create a command executor
	executor := NewCommandExecutor("test-executor")

	// Test service interface
	if name := executor.Name(); name != "test-executor" {
		t.Errorf("Expected name 'test-executor', got '%s'", name)
	}

	// Test service lifecycle
	ctx := context.Background()
	if err := executor.Start(ctx); err != nil {
		t.Fatalf("Failed to start executor: %v", err)
	}

	// Test health check
	health := executor.HealthCheck(ctx)
	if health.Status != "healthy" {
		t.Errorf("Expected healthy status, got '%s'", health.Status)
	}

	// Stop service
	if err := executor.Stop(ctx); err != nil {
		t.Errorf("Failed to stop executor: %v", err)
	}
}

func TestCommandExecutorInit(t *testing.T) {
	// Create temporary directory for test
	tempDir, err := os.MkdirTemp("", "cli-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	configPath := filepath.Join(tempDir, "test-config.yaml")

	executor := NewCommandExecutor("test-executor")
	ctx := context.Background()

	// Start the executor
	if err := executor.Start(ctx); err != nil {
		t.Fatalf("Failed to start executor: %v", err)
	}
	defer func() {
		if err := executor.Stop(ctx); err != nil {
			t.Errorf("Failed to stop executor: %v", err)
		}
	}()

	// Test init command
	req := InitRequest{
		ConfigPath: configPath,
		Force:      false,
	}

	result := executor.ExecuteInit(ctx, req)
	if result.IsErr() {
		t.Fatalf("Init command failed: %v", result.UnwrapErr())
	}

	response := result.Unwrap()
	if response.ConfigPath != configPath {
		t.Errorf("Expected config path '%s', got '%s'", configPath, response.ConfigPath)
	}

	if !response.Created {
		t.Error("Expected config to be created")
	}

	// Verify file was created
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("Config file was not created")
	}
}

func TestCommandExecutorBuildValidation(t *testing.T) {
	// Create temporary directory for test
	tempDir, err := os.MkdirTemp("", "cli-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Create a minimal valid config file
	configPath := filepath.Join(tempDir, "config.yaml")
	configContent := `
version: "2.0"
repositories: []
forges:
  - name: test-forge
    type: github
    auth:
      type: token
      token: "test-token"
    organizations: ["test-org"]
build:
  render_mode: "auto"
output:
  directory: "` + filepath.Join(tempDir, "output") + `"
hugo:
  base_url: "https://test.example.com"
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o600); err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	executor := NewCommandExecutor("test-executor")
	ctx := context.Background()

	// Start the executor
	if err := executor.Start(ctx); err != nil {
		t.Fatalf("Failed to start executor: %v", err)
	}
	defer func() {
		if err := executor.Stop(ctx); err != nil {
			t.Errorf("Failed to stop executor: %v", err)
		}
	}()

	// Test build command with empty repositories (should succeed with 0 files)
	req := BuildRequest{
		ConfigPath:  configPath,
		OutputDir:   filepath.Join(tempDir, "site"),
		Incremental: false,
		RenderMode:  "",
		Verbose:     false,
	}

	result := executor.ExecuteBuild(ctx, req)
	if result.IsErr() {
		t.Fatalf("Build command failed: %v", result.UnwrapErr())
	}

	response := result.Unwrap()
	if response.FilesBuilt != 0 {
		t.Errorf("Expected 0 files built, got %d", response.FilesBuilt)
	}

	if response.Repositories != 0 {
		t.Errorf("Expected 0 repositories, got %d", response.Repositories)
	}

	if response.BuildDuration <= 0 {
		t.Error("Expected positive build duration")
	}
}

func TestCommandExecutorValidation(t *testing.T) {
	executor := NewCommandExecutor("test-executor")
	ctx := context.Background()

	// Start the executor
	if err := executor.Start(ctx); err != nil {
		t.Fatalf("Failed to start executor: %v", err)
	}
	defer func() {
		if err := executor.Stop(ctx); err != nil {
			t.Errorf("Failed to stop executor: %v", err)
		}
	}()

	// Test with invalid config path
	req := BuildRequest{
		ConfigPath:  "/nonexistent/config.yaml",
		OutputDir:   "/tmp/output",
		Incremental: false,
		RenderMode:  "",
		Verbose:     false,
	}

	result := executor.ExecuteBuild(ctx, req)
	if result.IsOk() {
		t.Error("Expected build to fail with invalid config path")
	}

	// Test discover with invalid config
	discoverReq := DiscoverRequest{
		ConfigPath:   "/nonexistent/config.yaml",
		SpecificRepo: "",
	}

	discoverResult := executor.ExecuteDiscover(ctx, discoverReq)
	if discoverResult.IsOk() {
		t.Error("Expected discover to fail with invalid config path")
	}
}

func TestCommandExecutorIntegration(t *testing.T) {
	// This test validates the complete command executor lifecycle
	tempDir, err := os.MkdirTemp("", "cli-integration-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	executor := NewCommandExecutor("integration-executor")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Test service lifecycle
	if err := executor.Start(ctx); err != nil {
		t.Fatalf("Failed to start executor: %v", err)
	}

	// Verify health
	health := executor.HealthCheck(ctx)
	if health.Status != "healthy" {
		t.Errorf("Executor not healthy: %s - %s", health.Status, health.Message)
	}

	// Test command execution pipeline
	configPath := filepath.Join(tempDir, "integration.yaml")

	// 1. Init command
	initReq := InitRequest{
		ConfigPath: configPath,
		Force:      false,
	}

	initResult := executor.ExecuteInit(ctx, initReq)
	if initResult.IsErr() {
		t.Fatalf("Init failed: %v", initResult.UnwrapErr())
	}

	// 2. Verify config file structure and modify for testing
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("Config file not created: %v", err)
	}

	// Read and modify config to make it valid for build
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("Failed to load created config: %v", err)
	}

	// Ensure we have required fields for build
	if cfg.Hugo.BaseURL == "" {
		cfg.Hugo.BaseURL = "https://test.example.com"
	}
	if cfg.Output.Directory == "" {
		cfg.Output.Directory = filepath.Join(tempDir, "output")
	}

	// 3. Build command (should fail due to missing forge configuration)
	buildReq := BuildRequest{
		ConfigPath:  configPath,
		OutputDir:   filepath.Join(tempDir, "site"),
		Incremental: false,
		RenderMode:  "auto",
		Verbose:     true,
	}

	buildResult := executor.ExecuteBuild(ctx, buildReq)
	if buildResult.IsOk() {
		t.Error("Expected build to fail due to missing forge configuration")
	}

	// The failure should be related to Git authentication (expected for external repo)
	buildErr := buildResult.UnwrapErr()
	if buildErr == nil || (!strings.Contains(buildErr.Error(), "authentication required") && !strings.Contains(buildErr.Error(), "[git:error] failed to clone repository")) {
		t.Errorf("Expected authentication or git clone error, got: %v", buildErr)
	}

	// Test graceful shutdown
	if err := executor.Stop(ctx); err != nil {
		t.Errorf("Failed to stop executor: %v", err)
	}
}
