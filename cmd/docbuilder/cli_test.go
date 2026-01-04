package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/forge"
)

// MockCLIEnvironment provides a comprehensive testing environment for CLI commands.
type MockCLIEnvironment struct {
	workspaceDir  string
	configPath    string
	outputCapture *bytes.Buffer
	errorCapture  *bytes.Buffer
	forgeClients  map[string]forge.Client
	forgeConfigs  []*config.ForgeConfig
	tempFiles     []string
	testConfig    *config.Config
}

// NewMockCLIEnvironment creates a new CLI testing environment.
func NewMockCLIEnvironment(t *testing.T) *MockCLIEnvironment {
	t.Helper()
	workspaceDir := t.TempDir()

	return &MockCLIEnvironment{
		workspaceDir:  workspaceDir,
		configPath:    filepath.Join(workspaceDir, "docbuilder.yaml"),
		outputCapture: &bytes.Buffer{},
		errorCapture:  &bytes.Buffer{},
		forgeClients:  make(map[string]forge.Client),
		forgeConfigs:  make([]*config.ForgeConfig, 0),
		tempFiles:     make([]string, 0),
	}
}

// WithForgeClient adds a forge client to the testing environment.
func (env *MockCLIEnvironment) WithForgeClient(name string, client forge.Client) *MockCLIEnvironment {
	env.forgeClients[name] = client

	// Generate and store forge configuration
	if enhancedClient, ok := client.(interface{ GenerateForgeConfig() *config.ForgeConfig }); ok {
		forgeConfig := enhancedClient.GenerateForgeConfig()
		forgeConfig.Name = name
		env.forgeConfigs = append(env.forgeConfigs, forgeConfig)
	}

	return env
}

// WithRealisticForgeEcosystem sets up a realistic multi-platform forge ecosystem.
func (env *MockCLIEnvironment) WithRealisticForgeEcosystem() *MockCLIEnvironment {
	// GitHub with enterprise repositories
	github := forge.CreateRealisticGitHubMock("enterprise-github")
	github.AddRepository(forge.CreateMockGitHubRepo("company", "api-gateway", true, false, false, false))
	github.AddRepository(forge.CreateMockGitHubRepo("company", "user-service", true, false, false, false))
	github.AddRepository(forge.CreateMockGitHubRepo("company", "web-frontend", true, false, false, false))
	github.AddRepository(forge.CreateMockGitHubRepo("opensource", "python-sdk", true, false, false, false))

	// GitLab with internal projects
	gitlab := forge.CreateRealisticGitLabMock("internal-gitlab")
	gitlab.AddRepository(forge.CreateMockGitLabRepo("devops", "infrastructure-docs", true, true, false, false))
	gitlab.AddRepository(forge.CreateMockGitLabRepo("product", "user-guides", true, false, false, false))
	gitlab.AddRepository(forge.CreateMockGitLabRepo("engineering", "architecture-decisions", true, true, false, false))

	// Forgejo for self-hosted projects
	forgejo := forge.CreateRealisticForgejoMock("self-hosted-forgejo")
	forgejo.AddRepository(forge.CreateMockForgejoRepo("admin", "runbooks", true, false, false, false))
	forgejo.AddRepository(forge.CreateMockForgejoRepo("admin", "deployment-guides", true, false, false, false))

	env.WithForgeClient("enterprise-github", github)
	env.WithForgeClient("internal-gitlab", gitlab)
	env.WithForgeClient("self-hosted-forgejo", forgejo)

	return env
}

// WithTestConfiguration creates a test configuration file
//
//nolint:unparam // Method returns receiver for chaining; result sometimes ignored in tests.
func (env *MockCLIEnvironment) WithTestConfiguration() *MockCLIEnvironment {
	env.testConfig = &config.Config{
		Version: "2.0",
		Forges:  env.forgeConfigs,
		Build: config.BuildConfig{
			CleanUntracked: true,
		},
		Filtering: &config.FilteringConfig{
			RequiredPaths:   []string{"docs", "documentation"},
			IncludePatterns: []string{"*"},
			ExcludePatterns: []string{"*legacy*", "*deprecated*"},
		},
		Output: config.OutputConfig{
			Directory: filepath.Join(env.workspaceDir, "site"),
		},
		Hugo: config.HugoConfig{
			BaseURL: "https://docs.company.internal",
		},
	}

	return env
}

// WriteConfigFile writes the test configuration to disk.
func (env *MockCLIEnvironment) WriteConfigFile() error {
	if env.testConfig == nil {
		env.WithTestConfiguration()
	}

	configBytes, err := yaml.Marshal(env.testConfig)
	if err != nil {
		return err
	}

	return os.WriteFile(env.configPath, configBytes, 0o600)
}

// CreateProjectStructure creates a realistic project structure for testing.
func (env *MockCLIEnvironment) CreateProjectStructure() error {
	// Create basic project directories
	dirs := []string{
		"docs",
		"config",
		"scripts",
		".github/workflows",
	}

	for _, dir := range dirs {
		dirPath := filepath.Join(env.workspaceDir, dir)
		if err := os.MkdirAll(dirPath, 0o750); err != nil {
			return err
		}
	}

	// Create sample files
	files := map[string]string{
		"README.md":               "# DocBuilder CLI Test Project\n\nThis is a test project for CLI testing.",
		"docs/index.md":           "# Documentation\n\nWelcome to the documentation.",
		"docs/getting-started.md": "# Getting Started\n\nHow to get started with this project.",
		".gitignore":              "site/\n*.tmp\n.env.local\n",
	}

	for filePath, content := range files {
		fullPath := filepath.Join(env.workspaceDir, filePath)
		if err := os.WriteFile(fullPath, []byte(content), 0o600); err != nil {
			return err
		}
		env.tempFiles = append(env.tempFiles, fullPath)
	}

	return nil
}

// CLIResult represents the result of a CLI command execution.
type CLIResult struct {
	ExitCode       int
	Stdout         string
	Stderr         string
	Duration       time.Duration
	ConfigPath     string
	OutputDir      string
	GeneratedFiles []string
}

// RunCommand simulates running a DocBuilder CLI command.
func (env *MockCLIEnvironment) RunCommand(args ...string) *CLIResult {
	start := time.Now()

	// Reset capture buffers
	env.outputCapture.Reset()
	env.errorCapture.Reset()

	// Simulate command execution based on command type
	var exitCode int
	var generatedFiles []string

	if len(args) == 0 {
		return &CLIResult{
			ExitCode: 1,
			Stderr:   "No command specified",
			Duration: time.Since(start),
		}
	}

	command := args[0]
	switch command {
	case "init":
		exitCode, generatedFiles = env.simulateInitCommand(args[1:])
	case "build":
		exitCode, generatedFiles = env.simulateBuildCommand(args[1:])
	case "discover":
		exitCode = env.simulateDiscoverCommand(args[1:])
	case "daemon":
		exitCode = env.simulateDaemonCommand(args[1:])
	case "validate":
		exitCode = env.simulateValidateCommand(args[1:])
	default:
		env.errorCapture.WriteString("Unknown command: " + command)
		exitCode = 1
	}

	return &CLIResult{
		ExitCode:       exitCode,
		Stdout:         env.outputCapture.String(),
		Stderr:         env.errorCapture.String(),
		Duration:       time.Since(start),
		ConfigPath:     env.configPath,
		OutputDir:      filepath.Join(env.workspaceDir, "site"),
		GeneratedFiles: generatedFiles,
	}
}

// simulateInitCommand simulates the 'docbuilder init' command.
func (env *MockCLIEnvironment) simulateInitCommand(args []string) (int, []string) {
	env.outputCapture.WriteString("Initializing DocBuilder project...\n")

	// Parse flags
	autoDiscover := false
	configPath := env.configPath

	for i, arg := range args {
		switch arg {
		case "--auto-discover":
			autoDiscover = true
		case "--config":
			if i+1 < len(args) {
				configPath = args[i+1]
			}
		}
	}

	// Generate configuration
	if autoDiscover {
		env.outputCapture.WriteString("Auto-discovering forge configurations...\n")
		env.WithRealisticForgeEcosystem()
	}

	env.WithTestConfiguration()
	env.configPath = configPath

	if err := env.WriteConfigFile(); err != nil {
		env.errorCapture.WriteString("Failed to write configuration: " + err.Error())
		return 1, nil
	}

	env.outputCapture.WriteString("✓ Configuration written to: " + configPath + "\n")
	env.outputCapture.WriteString("✓ Found " + strconv.Itoa(len(env.forgeConfigs)) + " forge configurations\n")
	env.outputCapture.WriteString("✓ DocBuilder project initialized successfully\n")

	return 0, []string{configPath}
}

// simulateBuildCommand simulates the 'docbuilder build' command.
func (env *MockCLIEnvironment) simulateBuildCommand(args []string) (int, []string) {
	env.outputCapture.WriteString("Starting DocBuilder build...\n")

	// Parse configuration
	currentConfigPath := env.configPath
	for i, arg := range args {
		if arg == "--config" && i+1 < len(args) {
			currentConfigPath = args[i+1]
		}
	}

	// Validate config path
	if _, err := os.Stat(currentConfigPath); err != nil {
		env.errorCapture.WriteString("Configuration file not found: " + currentConfigPath + "\n")
		return 1, nil
	}

	// Simulate discovery phase
	env.outputCapture.WriteString("Discovering documentation repositories...\n")

	for _, client := range env.forgeClients {
		ctx := context.Background()
		repos, err := client.ListRepositories(ctx, []string{})
		if err != nil {
			env.errorCapture.WriteString("Discovery failed: " + err.Error())
			return 1, nil
		}
		env.outputCapture.WriteString("✓ " + client.GetName() + ": found " + strconv.Itoa(len(repos)) + " repositories\n")
	}

	// Simulate Hugo generation
	env.outputCapture.WriteString("Generating Hugo static site...\n")

	outputDir := filepath.Join(env.workspaceDir, "site")
	if err := os.MkdirAll(outputDir, 0o750); err != nil {
		env.errorCapture.WriteString("Failed to create output directory: " + err.Error())
		return 1, nil
	}

	// Create sample generated files
	generatedFiles := []string{
		"index.html",
		"sitemap.xml",
		"robots.txt",
		"css/style.css",
		"js/app.js",
	}

	var fullPaths []string
	for _, file := range generatedFiles {
		fullPath := filepath.Join(outputDir, file)

		// Create directory if needed
		dir := filepath.Dir(fullPath)
		if err := os.MkdirAll(dir, 0o750); err != nil {
			continue
		}

		// Create sample content
		content := "<!-- Generated by DocBuilder CLI Test -->\n"
		if strings.HasSuffix(file, ".html") {
			content = "<!DOCTYPE html><html><head><title>DocBuilder Test</title></head><body><h1>Generated Site</h1></body></html>"
		}

		if err := os.WriteFile(fullPath, []byte(content), 0o600); err == nil {
			fullPaths = append(fullPaths, fullPath)
		}
	}

	env.outputCapture.WriteString("✓ Hugo site generated successfully\n")
	env.outputCapture.WriteString("✓ Output directory: " + outputDir + "\n")
	env.outputCapture.WriteString("✓ Generated " + strconv.Itoa(len(fullPaths)) + " files\n")
	env.outputCapture.WriteString("Build completed successfully!\n")

	return 0, fullPaths
}

// simulateDiscoverCommand simulates the 'docbuilder discover' command.
func (env *MockCLIEnvironment) simulateDiscoverCommand(_ []string) int {
	env.outputCapture.WriteString("Discovering documentation repositories...\n")

	ctx := context.Background()
	totalRepos := 0
	totalWithDocs := 0

	for forgeName, client := range env.forgeClients {
		repos, err := client.ListRepositories(ctx, []string{})
		if err != nil {
			env.errorCapture.WriteString("Failed to discover " + forgeName + ": " + err.Error())
			continue
		}

		withDocs := 0
		for _, repo := range repos {
			if repo.HasDocs {
				withDocs++
			}
		}

		totalRepos += len(repos)
		totalWithDocs += withDocs

		env.outputCapture.WriteString("✓ " + forgeName + ": " + strconv.Itoa(len(repos)) + " repositories (" + strconv.Itoa(withDocs) + " with docs)\n")
	}

	env.outputCapture.WriteString("Discovery Summary:\n")
	env.outputCapture.WriteString("  Total repositories: " + strconv.Itoa(totalRepos) + "\n")
	env.outputCapture.WriteString("  With documentation: " + strconv.Itoa(totalWithDocs) + "\n")
	if totalRepos > 0 {
		coverage := (totalWithDocs * 100) / totalRepos
		env.outputCapture.WriteString("  Coverage: " + strconv.Itoa(coverage) + "%\n")
	}

	return 0
}

// simulateDaemonCommand simulates the 'docbuilder daemon' command.
func (env *MockCLIEnvironment) simulateDaemonCommand(_ []string) int {
	env.outputCapture.WriteString("DocBuilder daemon mode\n")
	env.outputCapture.WriteString("✓ Daemon configuration loaded\n")
	env.outputCapture.WriteString("✓ Webhook endpoints configured\n")
	env.outputCapture.WriteString("✓ Background services started\n")
	env.outputCapture.WriteString("Daemon is running... (simulated)\n")

	return 0
}

// simulateValidateCommand simulates the 'docbuilder validate' command.
func (env *MockCLIEnvironment) simulateValidateCommand(_ []string) int {
	env.outputCapture.WriteString("Validating DocBuilder configuration...\n")

	if env.testConfig == nil {
		env.errorCapture.WriteString("No configuration found")
		return 1
	}

	// Simulate validation checks
	validationResults := []string{
		"✓ Configuration format is valid",
		"✓ Forge configurations are accessible",
		"✓ Output directory is writable",
		"✓ Hugo theme is available",
		"✓ All required paths exist",
	}

	for _, result := range validationResults {
		env.outputCapture.WriteString(result + "\n")
	}

	env.outputCapture.WriteString("Configuration validation passed!\n")

	return 0
}

// AssertExitCode checks that the command exited with the expected code.
func (result *CLIResult) AssertExitCode(t *testing.T, expected int) {
	t.Helper()
	if result.ExitCode != expected {
		t.Errorf("Expected exit code %d, got %d. Stderr: %s", expected, result.ExitCode, result.Stderr)
	}
}

// AssertOutputContains checks that the stdout contains the expected text.
func (result *CLIResult) AssertOutputContains(t *testing.T, expected string) {
	t.Helper()
	if !strings.Contains(result.Stdout, expected) {
		t.Errorf("Expected output to contain %q, got: %s", expected, result.Stdout)
	}
}

// AssertErrorContains checks that the stderr contains the expected text.
func (result *CLIResult) AssertErrorContains(t *testing.T, expected string) {
	t.Helper()
	if !strings.Contains(result.Stderr, expected) {
		t.Errorf("Expected error to contain %q, got: %s", expected, result.Stderr)
	}
}

// AssertConfigGenerated checks that a configuration file was generated.
func (result *CLIResult) AssertConfigGenerated(t *testing.T) {
	t.Helper()
	if result.ConfigPath == "" {
		t.Error("Expected configuration path to be set")
		return
	}

	if _, err := os.Stat(result.ConfigPath); os.IsNotExist(err) {
		t.Errorf("Expected configuration file to exist at %s", result.ConfigPath)
	}
}

// AssertFilesGenerated checks that the expected files were generated.
func (result *CLIResult) AssertFilesGenerated(t *testing.T, minFiles int) {
	t.Helper()
	if len(result.GeneratedFiles) < minFiles {
		t.Errorf("Expected at least %d generated files, got %d", minFiles, len(result.GeneratedFiles))
	}

	for _, file := range result.GeneratedFiles {
		if _, err := os.Stat(file); os.IsNotExist(err) {
			t.Errorf("Expected generated file to exist: %s", file)
		}
	}
}

// AssertPerformance checks that the command completed within the expected time.
func (result *CLIResult) AssertPerformance(t *testing.T, maxDuration time.Duration) {
	t.Helper()
	if result.Duration > maxDuration {
		t.Errorf("Command took too long: %v (max: %v)", result.Duration, maxDuration)
	}
}

// Cleanup removes temporary files and directories.
func (env *MockCLIEnvironment) Cleanup() {
	for _, file := range env.tempFiles {
		_ = os.Remove(file)
	}
	// Note: workspaceDir is managed by testing.T.TempDir() and will be cleaned up automatically
}
