package config

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	// Create a temporary v2 config file
	configContent := "version: \"2.0\"\n" +
		"daemon:\n" +
		"  http:\n" +
		"    docs_port: 9000\n" +
		"    webhook_port: 9001\n" +
		"    admin_port: 9002\n" +
		"  sync:\n" +
		"    schedule: \"0 */6 * * *\"\n" +
		"    concurrent_builds: 5\n" +
		"    queue_size: 200\n" +
		"  storage:\n" +
		"    state_file: \"./custom-state.json\"\n" +
		"    repo_cache_dir: \"./custom-repos\"\n" +
		"    output_dir: \"./custom-output\"\n" +
		"forges:\n" +
		"  - name: test-github\n" +
		"    type: github\n" +
		"    api_url: https://api.github.com\n" +
		"    base_url: https://github.com\n" +
		"    organizations:\n" +
		"      - test-org\n" +
		"    auth:\n" +
		"      type: token\n" +
		"      token: test-token\n" +
		"    webhook:\n" +
		"      secret: test-secret\n" +
		"      path: /webhooks/github\n" +
		"      events:\n" +
		"        - push\n" +
		"        - repository\n" +
		"filtering:\n" +
		"  required_paths:\n" +
		"    - docs\n" +
		"    - documentation\n" +
		"  ignore_files:\n" +
		"    - .docignore\n" +
		"    - .nodocs\n" +
		"  include_patterns:\n" +
		"    - \"docs-*\"\n" +
		"    - \"*-documentation\"\n" +
		"  exclude_patterns:\n" +
		"    - \"*-deprecated\"\n" +
		"    - \"legacy-*\"\n" +
		"versioning:\n" +
		"  strategy: branches_and_tags\n" +
		"  default_branch_only: false\n" +
		"  branch_patterns:\n" +
		"    - main\n" +
		"    - master\n" +
		"    - release/*\n" +
		"  tag_patterns:\n" +
		"    - v*.*.*\n" +
		"    - release-*\n" +
		"  max_versions_per_repo: 15\n" +
		"hugo:\n" +
		"  title: Test Documentation\n" +
		"  description: Test description\n" +
		"  base_url: https://test.example.com\n" +
		"  theme: relearn\n" +
		"monitoring:\n" +
		"  metrics:\n" +
		"    enabled: true\n" +
		"    path: /custom-metrics\n" +
		"  health:\n" +
		"    path: /custom-health\n" +
		"  logging:\n" +
		"    level: debug\n" +
		"    format: text\n" +
		"output:\n" +
		"  directory: ./custom-output\n" +
		"  clean: true"

	// Write to temporary file
	tmpFile, err := os.CreateTemp("", "test-v2-config-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	if _, err := tmpFile.WriteString(configContent); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}
	_ = tmpFile.Close()

	// Test loading
	config, err := Load(tmpFile.Name())
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

	if forge.Type != ForgeGitHub {
		t.Errorf("Forge type = %v, want %s", forge.Type, ForgeGitHub)
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

	if config.Monitoring.Logging.Level != LogLevelDebug {
		t.Errorf("Logging level = %v, want %s", config.Monitoring.Logging.Level, LogLevelDebug)
	}
}

func TestConfigDefaults(t *testing.T) {
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
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	if _, err := tmpFile.WriteString(configContent); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}
	_ = tmpFile.Close()

	config, err := Load(tmpFile.Name())
	if err != nil {
		t.Fatalf("LoadV2() error: %v", err)
	}

	// Verify defaults were applied
	// Theme is always Relearn (removed from config)

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

func TestConfigValidation(t *testing.T) {
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
			expectedError: "either forges or repositories must be configured",
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
			defer func() { _ = os.Remove(tmpFile.Name()) }()

			if _, err := tmpFile.WriteString(tt.configContent); err != nil {
				t.Fatalf("Failed to write config: %v", err)
			}
			_ = tmpFile.Close()

			_, err = Load(tmpFile.Name())
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

func TestInit(t *testing.T) {
	tmpDir := os.TempDir()
	configPath := tmpDir + "/test-init-v2.yaml"

	// Clean up
	defer func() { _ = os.Remove(configPath) }()

	// Test initialization
	err := Init(configPath, false)
	if err != nil {
		t.Fatalf("InitV2() error: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatal("InitV2() did not create config file")
	}

	// Test loading the initialized config
	config, err := Load(configPath)
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
	err = Init(configPath, false)
	if err == nil {
		t.Error("InitV2() should fail when file exists and force=false")
	}

	// Test force overwrite
	err = Init(configPath, true)
	if err != nil {
		t.Errorf("InitV2() with force should succeed: %v", err)
	}
}

func TestIsConfigVersion(t *testing.T) {
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
			defer func() { _ = os.Remove(tmpFile.Name()) }()

			if _, err := tmpFile.WriteString(tt.configContent); err != nil {
				t.Fatalf("Failed to write config: %v", err)
			}
			_ = tmpFile.Close()

			isV2, err := IsConfigVersion(tmpFile.Name())

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
	isV2, err := IsConfigVersion("/non/existent/file.yaml")
	if err == nil {
		t.Error("IsV2Config() should error for non-existent file")
	}
	if isV2 {
		t.Error("IsV2Config() should return false for non-existent file")
	}
}

func TestEnvironmentVariableExpansion(t *testing.T) {
	// Set test environment variable
	_ = os.Setenv("TEST_TOKEN", "expanded-token-value")
	defer func() { _ = os.Unsetenv("TEST_TOKEN") }()

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
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	if _, err := tmpFile.WriteString(configContent); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}
	_ = tmpFile.Close()

	config, err := Load(tmpFile.Name())
	if err != nil {
		t.Fatalf("LoadV2() error: %v", err)
	}

	if config.Forges[0].Auth.Token != "expanded-token-value" {
		t.Errorf("Token = %v, want expanded-token-value", config.Forges[0].Auth.Token)
	}
}

func TestConfigValidationWithTestForgeFactory(t *testing.T) {
	factory := NewTestForgeConfigFactory()

	t.Run("RealisticGitHubForgeConfiguration", func(t *testing.T) {
		githubForge := factory.CreateGitHubForge("config-validation")

		// Create a config with the realistic forge
		configContent := fmt.Sprintf(`version: "2.0"
forges:
  - name: %s
    type: %s
    api_url: %s
    base_url: %s
    organizations:
      - %s
    auth:
      type: %s
      token: %s
    webhook:
      secret: %s
      path: %s
      events:
        - push
        - repository
hugo:
  title: Test Documentation
  theme: relearn`,
			githubForge.Name,
			githubForge.Type,
			githubForge.APIURL,
			githubForge.BaseURL,
			githubForge.Organizations[0],
			githubForge.Auth.Type,
			githubForge.Auth.Token,
			githubForge.Webhook.Secret,
			githubForge.Webhook.Path)

		tmpFile, err := os.CreateTemp("", "test-realistic-github-*.yaml")
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}
		defer func() { _ = os.Remove(tmpFile.Name()) }()

		if _, err := tmpFile.WriteString(configContent); err != nil {
			t.Fatalf("Failed to write config: %v", err)
		}
		_ = tmpFile.Close()

		config, err := Load(tmpFile.Name())
		if err != nil {
			t.Fatalf("Failed to load realistic GitHub config: %v", err)
		}

		// Validate the forge was loaded correctly
		if len(config.Forges) != 1 {
			t.Fatalf("Expected 1 forge, got %d", len(config.Forges))
		}

		forge := config.Forges[0]
		if forge.Type != ForgeGitHub {
			t.Errorf("Expected GitHub forge, got %v", forge.Type)
		}

		if forge.Auth.Type != AuthTypeToken {
			t.Errorf("Expected token auth, got %v", forge.Auth.Type)
		}

		if len(forge.Organizations) == 0 {
			t.Error("Expected organizations to be configured")
		}

		if forge.Webhook == nil {
			t.Error("Expected webhook configuration")
		}
	})

	t.Run("RealisticGitLabForgeConfiguration", func(t *testing.T) {
		gitlabForge := factory.CreateGitLabForge("config-validation")

		configContent := fmt.Sprintf(`version: "2.0"
forges:
  - name: %s
    type: %s
    api_url: %s
    base_url: %s
    groups:
      - %s
    auth:
      type: %s
      token: %s
    webhook:
      secret: %s
      path: %s
      events:
        - push
        - repository
hugo:
  title: Test Documentation
  theme: relearn`,
			gitlabForge.Name,
			gitlabForge.Type,
			gitlabForge.APIURL,
			gitlabForge.BaseURL,
			gitlabForge.Groups[0],
			gitlabForge.Auth.Type,
			gitlabForge.Auth.Token,
			gitlabForge.Webhook.Secret,
			gitlabForge.Webhook.Path)

		tmpFile, err := os.CreateTemp("", "test-realistic-gitlab-*.yaml")
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}
		defer func() { _ = os.Remove(tmpFile.Name()) }()

		if _, err := tmpFile.WriteString(configContent); err != nil {
			t.Fatalf("Failed to write config: %v", err)
		}
		_ = tmpFile.Close()

		config, err := Load(tmpFile.Name())
		if err != nil {
			t.Fatalf("Failed to load realistic GitLab config: %v", err)
		}

		forge := config.Forges[0]
		if forge.Type != ForgeGitLab {
			t.Errorf("Expected GitLab forge, got %v", forge.Type)
		}

		if len(forge.Groups) == 0 {
			t.Error("Expected groups to be configured for GitLab")
		}

		// Verify GitLab-specific API URL
		if !strings.Contains(forge.APIURL, "api/v4") {
			t.Errorf("Expected GitLab API URL with api/v4, got %s", forge.APIURL)
		}
	})

	t.Run("MultiPlatformForgeValidation", func(t *testing.T) {
		githubForge := factory.CreateGitHubForge("multi-github")
		gitlabForge := factory.CreateGitLabForge("multi-gitlab")
		forgejoForge := factory.CreateForgejoForge("multi-forgejo")

		configContent := fmt.Sprintf(`version: "2.0"
forges:
  - name: %s
    type: %s
    api_url: %s
    organizations:
      - %s
    auth:
      type: %s
      token: %s
  - name: %s
    type: %s
    api_url: %s
    groups:
      - %s
    auth:
      type: %s
      token: %s
  - name: %s
    type: %s
    api_url: %s
    organizations:
      - %s
    auth:
      type: %s
      token: %s
hugo:
  title: Multi-Platform Documentation`,
			githubForge.Name, githubForge.Type, githubForge.APIURL, githubForge.Organizations[0],
			githubForge.Auth.Type, githubForge.Auth.Token,
			gitlabForge.Name, gitlabForge.Type, gitlabForge.APIURL, gitlabForge.Groups[0],
			gitlabForge.Auth.Type, gitlabForge.Auth.Token,
			forgejoForge.Name, forgejoForge.Type, forgejoForge.APIURL, forgejoForge.Organizations[0],
			forgejoForge.Auth.Type, forgejoForge.Auth.Token)

		tmpFile, err := os.CreateTemp("", "test-multi-platform-*.yaml")
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}
		defer func() { _ = os.Remove(tmpFile.Name()) }()

		if _, err := tmpFile.WriteString(configContent); err != nil {
			t.Fatalf("Failed to write config: %v", err)
		}
		_ = tmpFile.Close()

		config, err := Load(tmpFile.Name())
		if err != nil {
			t.Fatalf("Failed to load multi-platform config: %v", err)
		}

		if len(config.Forges) != 3 {
			t.Fatalf("Expected 3 forges, got %d", len(config.Forges))
		}

		// Verify forge types
		forgeTypes := make(map[ForgeType]bool)
		for _, forge := range config.Forges {
			forgeTypes[forge.Type] = true
		}

		expectedTypes := []ForgeType{ForgeGitHub, ForgeGitLab, ForgeForgejo}
		for _, expectedType := range expectedTypes {
			if !forgeTypes[expectedType] {
				t.Errorf("Expected forge type %v not found", expectedType)
			}
		}

		t.Logf("✓ Multi-platform configuration validated: %d forges across GitHub/GitLab/Forgejo", len(config.Forges))
	})

	t.Run("RealisticWebhookConfiguration", func(t *testing.T) {
		forge := factory.CreateGitHubForge("webhook-test")

		// Test webhook validation with realistic data
		configContent := fmt.Sprintf(`version: "2.0"
forges:
  - name: %s
    type: %s
    api_url: %s
    organizations:
      - %s
    auth:
      type: %s
      token: %s
    webhook:
      secret: %s
      path: %s
      events:
        - %s
        - %s
hugo:
  title: Webhook Test Documentation`,
			forge.Name, forge.Type, forge.APIURL, forge.Organizations[0],
			forge.Auth.Type, forge.Auth.Token,
			forge.Webhook.Secret, forge.Webhook.Path,
			forge.Webhook.Events[0], forge.Webhook.Events[1])

		tmpFile, err := os.CreateTemp("", "test-webhook-config-*.yaml")
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}
		defer func() { _ = os.Remove(tmpFile.Name()) }()

		if _, err := tmpFile.WriteString(configContent); err != nil {
			t.Fatalf("Failed to write config: %v", err)
		}
		_ = tmpFile.Close()

		config, err := Load(tmpFile.Name())
		if err != nil {
			t.Fatalf("Failed to load webhook config: %v", err)
		}

		loadedForge := config.Forges[0]
		if loadedForge.Webhook == nil {
			t.Fatal("Expected webhook configuration")
		}

		if loadedForge.Webhook.Secret == "" {
			t.Error("Expected webhook secret to be set")
		}

		if loadedForge.Webhook.Path == "" {
			t.Error("Expected webhook path to be set")
		}

		if len(loadedForge.Webhook.Events) == 0 {
			t.Error("Expected webhook events to be configured")
		}

		// Verify webhook path format
		if !strings.HasPrefix(loadedForge.Webhook.Path, "/webhooks/") {
			t.Errorf("Expected webhook path to start with /webhooks/, got %s", loadedForge.Webhook.Path)
		}
	})
}

func TestAdvancedConfigurationScenarios(t *testing.T) {
	factory := NewTestForgeConfigFactory()

	t.Run("AutoDiscoveryConfiguration", func(t *testing.T) {
		// Test configuration with auto-discovery enabled
		autoDiscoverForge := factory.CreateForgeWithAutoDiscover(ForgeGitHub, "auto-discover")

		configContent := fmt.Sprintf(`version: "2.0"
forges:
  - name: %s
    type: %s
    api_url: %s
    auto_discover: true
    auth:
      type: %s
      token: %s
    options:
      auto_discover: true
hugo:
  title: Auto-Discovery Documentation`,
			autoDiscoverForge.Name, autoDiscoverForge.Type, autoDiscoverForge.APIURL,
			autoDiscoverForge.Auth.Type, autoDiscoverForge.Auth.Token)

		tmpFile, err := os.CreateTemp("", "test-auto-discover-*.yaml")
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}
		defer func() { _ = os.Remove(tmpFile.Name()) }()

		if _, err := tmpFile.WriteString(configContent); err != nil {
			t.Fatalf("Failed to write config: %v", err)
		}
		_ = tmpFile.Close()

		config, err := Load(tmpFile.Name())
		if err != nil {
			t.Fatalf("Failed to load auto-discovery config: %v", err)
		}

		forge := config.Forges[0]
		if !forge.AutoDiscover {
			t.Error("Expected auto-discovery to be enabled")
		}

		// With auto-discovery, organizations/groups should be empty or minimal
		if len(forge.Organizations) > 0 && len(forge.Groups) > 0 {
			t.Error("Auto-discovery forge should not have both organizations and groups pre-configured")
		}
	})

	t.Run("MonitoringConfigurationValidation", func(t *testing.T) {
		forge := factory.CreateGitHubForge("monitoring-test")

		configContent := fmt.Sprintf(`version: "2.0"
forges:
  - name: %s
    type: %s
    api_url: %s
    organizations:
      - %s
    auth:
      type: %s
      token: %s
hugo:
  title: Monitoring Test
monitoring:
  metrics:
    enabled: true
    path: /custom-metrics
  health:
    path: /custom-health
  logging:
    level: debug
    format: json`,
			forge.Name, forge.Type, forge.APIURL, forge.Organizations[0],
			forge.Auth.Type, forge.Auth.Token)

		tmpFile, err := os.CreateTemp("", "test-monitoring-*.yaml")
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}
		defer func() { _ = os.Remove(tmpFile.Name()) }()

		if _, err := tmpFile.WriteString(configContent); err != nil {
			t.Fatalf("Failed to write config: %v", err)
		}
		_ = tmpFile.Close()

		config, err := Load(tmpFile.Name())
		if err != nil {
			t.Fatalf("Failed to load monitoring config: %v", err)
		}

		// Verify monitoring configuration
		if !config.Monitoring.Metrics.Enabled {
			t.Error("Expected metrics to be enabled")
		}

		if config.Monitoring.Metrics.Path != "/custom-metrics" {
			t.Errorf("Expected custom metrics path, got %s", config.Monitoring.Metrics.Path)
		}

		if config.Monitoring.Logging.Level != LogLevelDebug {
			t.Errorf("Expected debug logging level, got %v", config.Monitoring.Logging.Level)
		}
	})
}
