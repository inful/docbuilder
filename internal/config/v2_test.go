package config

import (
	"os"
	"strings"
	"testing"
)

func TestLoadV2Config(t *testing.T) {
	// Create a temporary v2 config file
	configContent := `version: "2.0"
daemon:
  http:
    docs_port: 9000
    webhook_port: 9001
    admin_port: 9002
  sync:
    schedule: "0 */6 * * *"
    concurrent_builds: 5
    queue_size: 200
  storage:
    state_file: "./custom-state.json"
    repo_cache_dir: "./custom-repos"
    output_dir: "./custom-site"
forges:
  - name: test-github
    type: github
    api_url: https://api.github.com
    base_url: https://github.com
    organizations:
      - test-org
    auth:
      type: token
      token: test-token
    webhook:
      secret: test-secret
      path: /webhooks/github
      events:
        - push
        - repository
filtering:
  required_paths:
    - docs
    - documentation
  ignore_files:
    - .docignore
    - .nodocs
  include_patterns:
    - "docs-*"
    - "*-documentation"
  exclude_patterns:
    - "*-deprecated"
    - "legacy-*"
versioning:
  strategy: branches_and_tags
  default_branch_only: false
  branch_patterns:
    - main
    - master
    - release/*
  tag_patterns:
    - v*.*.*
    - release-*
  max_versions_per_repo: 15
hugo:
  title: Test Documentation
  description: Test description
  base_url: https://test.example.com
  theme: hextra
monitoring:
  metrics:
    enabled: true
    path: /custom-metrics
  health:
    path: /custom-health
  logging:
    level: debug
    format: text
output:
  directory: ./custom-output
  clean: true`

	// Write to temporary file
	tmpFile, err := os.CreateTemp("", "test-v2-config-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(configContent); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}
	tmpFile.Close()

	// Test loading
	config, err := LoadV2(tmpFile.Name())
	if err != nil {
		t.Fatalf("LoadV2() error: %v", err)
	}

	// Verify loaded configuration
	if config.Version != "2.0" {
		t.Errorf("Version = %v, want 2.0", config.Version)
	}

	// Test daemon config
	if config.Daemon.HTTP.DocsPort != 9000 {
		t.Errorf("DocsPort = %v, want 9000", config.Daemon.HTTP.DocsPort)
	}

	if config.Daemon.Sync.Schedule != "0 */6 * * *" {
		t.Errorf("Schedule = %v, want '0 */6 * * *'", config.Daemon.Sync.Schedule)
	}

	// Test forges
	if len(config.Forges) != 1 {
		t.Fatalf("Forges count = %v, want 1", len(config.Forges))
	}

	forge := config.Forges[0]
	if forge.Name != "test-github" {
		t.Errorf("Forge name = %v, want test-github", forge.Name)
	}

	if forge.Type != "github" {
		t.Errorf("Forge type = %v, want github", forge.Type)
	}

	if len(forge.Organizations) != 1 || forge.Organizations[0] != "test-org" {
		t.Errorf("Forge organizations = %v, want [test-org]", forge.Organizations)
	}

	// Test filtering
	if len(config.Filtering.RequiredPaths) != 2 {
		t.Errorf("RequiredPaths count = %v, want 2", len(config.Filtering.RequiredPaths))
	}

	if len(config.Filtering.IncludePatterns) != 2 {
		t.Errorf("IncludePatterns count = %v, want 2", len(config.Filtering.IncludePatterns))
	}

	// Test versioning
	if config.Versioning.Strategy != StrategyBranchesAndTags {
		t.Errorf("Versioning strategy = %v, want %s", config.Versioning.Strategy, StrategyBranchesAndTags)
	}

	if config.Versioning.MaxVersionsPerRepo != 15 {
		t.Errorf("MaxVersionsPerRepo = %v, want 15", config.Versioning.MaxVersionsPerRepo)
	}

	// Test monitoring
	if config.Monitoring.Metrics.Path != "/custom-metrics" {
		t.Errorf("Metrics path = %v, want /custom-metrics", config.Monitoring.Metrics.Path)
	}

	if config.Monitoring.Logging.Level != "debug" {
		t.Errorf("Logging level = %v, want debug", config.Monitoring.Logging.Level)
	}
}

func TestV2ConfigDefaults(t *testing.T) {
	// Create minimal v2 config
	configContent := `version: "2.0"
forges:
  - name: minimal-github
    type: github
    organizations:
      - test-org
    auth:
      type: token
      token: test-token
hugo:
  title: Minimal Config`

	tmpFile, err := os.CreateTemp("", "test-v2-minimal-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(configContent); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}
	tmpFile.Close()

	config, err := LoadV2(tmpFile.Name())
	if err != nil {
		t.Fatalf("LoadV2() error: %v", err)
	}

	// Verify defaults were applied
	if config.Hugo.Theme != "hextra" {
		t.Errorf("Default theme = %v, want hextra", config.Hugo.Theme)
	}

	if config.Output.Directory != "./site" {
		t.Errorf("Default output directory = %v, want ./site", config.Output.Directory)
	}

	if !config.Output.Clean {
		t.Error("Output clean should default to true in daemon mode")
	}

	// Daemon should be nil since not specified
	if config.Daemon != nil {
		t.Error("Daemon should be nil when not specified")
	}

	// Filtering should have defaults
	if len(config.Filtering.RequiredPaths) != 1 || config.Filtering.RequiredPaths[0] != "docs" {
		t.Errorf("Default required paths = %v, want [docs]", config.Filtering.RequiredPaths)
	}

	if len(config.Filtering.IgnoreFiles) != 1 || config.Filtering.IgnoreFiles[0] != ".docignore" {
		t.Errorf("Default ignore files = %v, want [.docignore]", config.Filtering.IgnoreFiles)
	}

	// Versioning should have defaults
	if config.Versioning.Strategy != StrategyBranchesAndTags {
		t.Errorf("Default versioning strategy = %v, want %s", config.Versioning.Strategy, StrategyBranchesAndTags)
	}

	if config.Versioning.MaxVersionsPerRepo != 10 {
		t.Errorf("Default max versions = %v, want 10", config.Versioning.MaxVersionsPerRepo)
	}

	// Monitoring should have defaults
	if config.Monitoring.Metrics.Path != "/metrics" {
		t.Errorf("Default metrics path = %v, want /metrics", config.Monitoring.Metrics.Path)
	}
}

func TestV2ConfigValidation(t *testing.T) {
	tests := []struct {
		name          string
		configContent string
		expectedError string
	}{
		{
			name: "Wrong version",
			configContent: `version: "1.0"
forges:
  - name: test
    type: github
    auth:
      type: token
      token: test`,
			expectedError: "unsupported configuration version",
		},
		{
			name: "No forges",
			configContent: `version: "2.0"
hugo:
  title: Test`,
			expectedError: "at least one forge must be configured",
		},
		{
			name: "Duplicate forge names",
			configContent: `version: "2.0"
forges:
  - name: duplicate
    type: github
    organizations: [test-org]
    auth:
      type: token
      token: test1
  - name: duplicate
    type: gitlab
    groups: [test-group]
    auth:
      type: token
      token: test2`,
			expectedError: "duplicate forge name",
		},
		{
			name: "Invalid forge type",
			configContent: `version: "2.0"
forges:
  - name: invalid
    type: invalid-type
    auth:
      type: token
      token: test`,
			expectedError: "unsupported forge type",
		},
		{
			name: "No authentication",
			configContent: `version: "2.0"
forges:
  - name: no-auth
    type: github
    organizations: [test-org]`,
			expectedError: "must have authentication configured",
		},
		{
			name: "No organizations or groups",
			configContent: `version: "2.0"
forges:
  - name: no-orgs
    type: github
    auth:
      type: token
      token: test`,
			expectedError: "must have at least one organization or group configured",
		},
		{
			name: "Invalid versioning strategy",
			configContent: `version: "2.0"
forges:
  - name: test
    type: github
    organizations: [test-org]
    auth:
      type: token
      token: test
versioning:
  strategy: invalid-strategy`,
			expectedError: "invalid versioning strategy",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpFile, err := os.CreateTemp("", "test-v2-validation-*.yaml")
			if err != nil {
				t.Fatalf("Failed to create temp file: %v", err)
			}
			defer os.Remove(tmpFile.Name())

			if _, err := tmpFile.WriteString(tt.configContent); err != nil {
				t.Fatalf("Failed to write config: %v", err)
			}
			tmpFile.Close()

			_, err = LoadV2(tmpFile.Name())
			if err == nil {
				t.Errorf("LoadV2() expected error, got nil")
				return
			}

			if !strings.Contains(err.Error(), tt.expectedError) {
				t.Errorf("LoadV2() error = %v, want to contain %v", err.Error(), tt.expectedError)
			}
		})
	}
}

func TestInitV2(t *testing.T) {
	tmpDir := os.TempDir()
	configPath := tmpDir + "/test-init-v2.yaml"

	// Clean up
	defer os.Remove(configPath)

	// Test initialization
	err := InitV2(configPath, false)
	if err != nil {
		t.Fatalf("InitV2() error: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatal("InitV2() did not create config file")
	}

	// Test loading the initialized config
	config, err := LoadV2(configPath)
	if err != nil {
		t.Fatalf("Failed to load initialized config: %v", err)
	}

	if config.Version != "2.0" {
		t.Errorf("Initialized config version = %v, want 2.0", config.Version)
	}

	if len(config.Forges) == 0 {
		t.Error("Initialized config should have example forges")
	}

	// Test overwrite protection
	err = InitV2(configPath, false)
	if err == nil {
		t.Error("InitV2() should fail when file exists and force=false")
	}

	// Test force overwrite
	err = InitV2(configPath, true)
	if err != nil {
		t.Errorf("InitV2() with force should succeed: %v", err)
	}
}

func TestIsV2Config(t *testing.T) {
	tests := []struct {
		name          string
		configContent string
		expectedV2    bool
		expectedError bool
	}{
		{
			name: "V2 config",
			configContent: `version: "2.0"
forges:
  - name: test
    type: github`,
			expectedV2: true,
		},
		{
			name: "V2.1 config",
			configContent: `version: "2.1"
forges:
  - name: test
    type: github`,
			expectedV2: true,
		},
		{
			name: "V1 config",
			configContent: `repositories:
  - url: https://github.com/test/test.git
    name: test`,
			expectedV2: false,
		},
		{
			name:          "Invalid YAML",
			configContent: `invalid: yaml: content: [[[`,
			expectedV2:    false,
			expectedError: false, // Should not error, just return false
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpFile, err := os.CreateTemp("", "test-is-v2-*.yaml")
			if err != nil {
				t.Fatalf("Failed to create temp file: %v", err)
			}
			defer os.Remove(tmpFile.Name())

			if _, err := tmpFile.WriteString(tt.configContent); err != nil {
				t.Fatalf("Failed to write config: %v", err)
			}
			tmpFile.Close()

			isV2, err := IsV2Config(tmpFile.Name())

			if tt.expectedError && err == nil {
				t.Errorf("IsV2Config() expected error, got nil")
				return
			}

			if !tt.expectedError && err != nil {
				t.Errorf("IsV2Config() unexpected error: %v", err)
				return
			}

			if isV2 != tt.expectedV2 {
				t.Errorf("IsV2Config() = %v, want %v", isV2, tt.expectedV2)
			}
		})
	}

	// Test non-existent file
	isV2, err := IsV2Config("/non/existent/file.yaml")
	if err == nil {
		t.Error("IsV2Config() should error for non-existent file")
	}
	if isV2 {
		t.Error("IsV2Config() should return false for non-existent file")
	}
}

func TestEnvironmentVariableExpansion(t *testing.T) {
	// Set test environment variable
	os.Setenv("TEST_TOKEN", "expanded-token-value")
	defer os.Unsetenv("TEST_TOKEN")

	configContent := `version: "2.0"
forges:
  - name: env-test
    type: github
    organizations: [test-org]
    auth:
      type: token
      token: "${TEST_TOKEN}"
hugo:
  title: Environment Test`

	tmpFile, err := os.CreateTemp("", "test-env-expansion-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(configContent); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}
	tmpFile.Close()

	config, err := LoadV2(tmpFile.Name())
	if err != nil {
		t.Fatalf("LoadV2() error: %v", err)
	}

	if config.Forges[0].Auth.Token != "expanded-token-value" {
		t.Errorf("Token = %v, want expanded-token-value", config.Forges[0].Auth.Token)
	}
}
