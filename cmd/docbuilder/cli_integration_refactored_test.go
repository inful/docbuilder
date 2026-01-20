package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	testutils "git.home.luguber.info/inful/docbuilder/internal/testutil/testutils"
)

// TestRefactoredCLIFramework demonstrates how to use the new testing framework
// to reduce complexity in CLI integration tests.
func TestRefactoredCLIFramework(t *testing.T) {
	// Test Scenario 1: CLI Init Command
	initScenario := &testutils.TestScenario{
		Name:        "CLIInitCommand",
		Description: "Test CLI initialization workflow with various options",
		Timeout:     10 * time.Second,
		Setup:       testutils.NewTestEnvironment,
		Execute: func(t *testing.T, env *testutils.TestEnvironment) *testutils.TestResult {
			t.Helper()
			cliEnv := testutils.NewMockCLIEnvironment(t).
				WithBinaryPath("../../bin/docbuilder")

			// Test basic init
			result := cliEnv.RunCommand("init", "--output", env.TempDir)
			result.AssertSuccess(t).
				AssertOutputContains(t, "Initializing DocBuilder project").
				AssertOutputContains(t, "initialized successfully")

			// Verify config file was created
			fileAssertions := testutils.NewFileAssertions(t, env.TempDir)
			fileAssertions.AssertFileExists("docbuilder.yaml").
				AssertFileContains("docbuilder.yaml", "version: \"2.0\"")

			return &testutils.TestResult{
				Success:  result.ExitCode == 0,
				Duration: result.Duration,
				Output:   result.Stdout,
			}
		},
	}

	// Test Scenario 2: CLI Build Command
	buildScenario := &testutils.TestScenario{
		Name:        "CLIBuildCommand",
		Description: "Test CLI build workflow with realistic configuration",
		Timeout:     30 * time.Second,
		Setup: func(t *testing.T) *testutils.TestEnvironment {
			t.Helper()
			env := testutils.NewTestEnvironment(t)

			// Create realistic configuration with a mock repository
			configFactory := testutils.NewConfigFactory(t)
			config := configFactory.MinimalConfig()
			_ = config // used via env.WithConfig below or intentionally ignored for setup clarity

			// Use the builder to add a mock repository
			builder := testutils.NewConfigBuilder(t).
				WithGitHubForge("test-github", "test-token", "test-org").
				WithRepository("test-repo", "file:///nonexistent/repo.git", "main")
			config = builder.Build()
			config.Output.Directory = env.OutputDir

			return env.WithConfig(config)
		},
		Execute: func(t *testing.T, env *testutils.TestEnvironment) *testutils.TestResult {
			t.Helper()
			cliEnv := testutils.NewMockCLIEnvironment(t).
				WithBinaryPath("../../bin/docbuilder")
			cliEnv.Config = env.Config

			if err := cliEnv.WriteConfigFile(); err != nil {
				t.Fatalf("Failed to write config file: %v", err)
			}

			// Execute build command - expect it to start properly but may fail due to repo access
			result := cliEnv.RunCommand("build", "--config", cliEnv.ConfigPath())

			// Check that the build process started correctly
			if !strings.Contains(result.Stdout, "Starting DocBuilder build") {
				t.Errorf("Expected output to contain 'Starting DocBuilder build', got: %s", result.Stdout)
				return &testutils.TestResult{Success: false}
			}

			// For now, accept either success or expected failure from missing repositories
			// The key test is that the CLI framework and binary work correctly
			if result.ExitCode != 0 {
				// If it failed, check it was due to repository issues (expected in test)
				if !strings.Contains(result.Stderr, "repository not found") && !strings.Contains(result.Stderr, "clone not found") {
					t.Errorf("Build failed with unexpected error: %s", result.Stderr)
					return &testutils.TestResult{Success: false}
				}
			} else {
				// If it succeeded, check for completion message and files
				if !strings.Contains(result.Stdout, "Build completed successfully") {
					t.Errorf("Expected successful build to contain completion message")
					return &testutils.TestResult{Success: false}
				}

				// RenderModeAuto intentionally skips invoking Hugo, so output/public may
				// not exist even on a successful build. If Hugo did run (or a renderer
				// produced output), the directory should be present and non-empty.
				if _, err := os.Stat(filepath.Join(env.OutputDir, "public")); err == nil {
					fileAssertions := testutils.NewFileAssertions(t, env.OutputDir)
					fileAssertions.AssertDirExists("public").
						AssertMinFileCount("public", 1)
				}
			}

			return &testutils.TestResult{
				Success:  true, // CLI framework working is the main test
				Duration: result.Duration,
				Output:   result.Stdout,
			}
		},
	}

	// Test Scenario 3: Auto-Discovery Configuration
	autoDiscoveryScenario := &testutils.TestScenario{
		Name:        "AutoDiscoveryConfiguration",
		Description: "Test configuration with auto-discovery enabled",
		Setup: func(t *testing.T) *testutils.TestEnvironment {
			t.Helper()
			env := testutils.NewTestEnvironment(t)

			// Create auto-discovery configuration
			configFactory := testutils.NewConfigFactory(t)
			config := configFactory.AutoDiscoveryConfig()

			return env.WithConfig(config)
		},
		Execute: func(t *testing.T, env *testutils.TestEnvironment) *testutils.TestResult {
			t.Helper()
			// Validate auto-discovery configuration
			forge := env.Config.Forges[0]
			if !forge.AutoDiscover {
				t.Error("Expected auto-discovery to be enabled")
			}

			if len(forge.Organizations) > 0 && len(forge.Groups) > 0 {
				t.Error("Auto-discovery forge should not have both organizations and groups pre-configured")
			}

			return &testutils.TestResult{
				Success: true,
				Output:  "Auto-discovery configuration validated",
			}
		},
	}

	// Run all scenarios
	initScenario.Run(t)
	buildScenario.Run(t)
	autoDiscoveryScenario.Run(t)
}

// TestRefactoredConfigValidation demonstrates complex configuration validation
// using the new testing framework patterns.
func TestRefactoredConfigValidation(t *testing.T) {
	configFactory := testutils.NewConfigFactory(t)

	validationTests := []*testutils.TestScenario{
		{
			Name: "ValidMinimalConfig",
			Setup: func(t *testing.T) *testutils.TestEnvironment {
				t.Helper()
				config := configFactory.MinimalConfig()
				return testutils.NewTestEnvironment(t).WithConfig(config)
			},
			Execute: func(t *testing.T, env *testutils.TestEnvironment) *testutils.TestResult {
				t.Helper()
				// Apply defaults first (like normal config loading does)
				applier := config.NewDefaultApplier()
				if err := applier.ApplyDefaults(env.Config); err != nil {
					t.Errorf("Failed to apply defaults: %v", err)
					return &testutils.TestResult{Success: false}
				}

				err := config.ValidateConfig(env.Config)
				if err != nil {
					t.Errorf("Expected valid config to pass validation: %v", err)
					return &testutils.TestResult{Success: false}
				}
				return &testutils.TestResult{Success: true}
			},
		},
		{
			Name: "InvalidEmptyForges",
			Setup: func(t *testing.T) *testutils.TestEnvironment {
				t.Helper()
				config := configFactory.ValidationTestConfig("empty_forges")
				return testutils.NewTestEnvironment(t).WithConfig(config)
			},
			Execute: func(t *testing.T, env *testutils.TestEnvironment) *testutils.TestResult {
				t.Helper()
				err := config.ValidateConfig(env.Config)
				if err == nil {
					t.Error("Expected config with no forges to fail validation")
					return &testutils.TestResult{Success: false}
				}

				expectedError := "either forges or repositories must be configured"
				if !containsString(err.Error(), expectedError) {
					t.Errorf("Expected error to contain %q, got: %v", expectedError, err)
					return &testutils.TestResult{Success: false}
				}
				return &testutils.TestResult{Success: true}
			},
		},
		{
			Name: "MultiForgeConfig",
			Setup: func(t *testing.T) *testutils.TestEnvironment {
				t.Helper()
				config := configFactory.MultiForgeConfig()
				return testutils.NewTestEnvironment(t).WithConfig(config)
			},
			Execute: func(t *testing.T, env *testutils.TestEnvironment) *testutils.TestResult {
				t.Helper()
				if len(env.Config.Forges) != 3 {
					t.Errorf("Expected 3 forges, got %d", len(env.Config.Forges))
					return &testutils.TestResult{Success: false}
				}

				forgeTypes := make(map[config.ForgeType]bool)
				for _, forge := range env.Config.Forges {
					forgeTypes[forge.Type] = true
				}

				expected := []config.ForgeType{
					config.ForgeGitHub,
					config.ForgeGitLab,
					config.ForgeForgejo,
				}

				for _, expectedType := range expected {
					if !forgeTypes[expectedType] {
						t.Errorf("Expected forge type %s not found", expectedType)
						return &testutils.TestResult{Success: false}
					}
				}

				return &testutils.TestResult{Success: true}
			},
		},
	}

	// Run all validation test scenarios
	for _, scenario := range validationTests {
		scenario.Run(t)
	}
}

// Helper function for string containment check.
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (len(substr) == 0 || stringContains(s, substr))
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
