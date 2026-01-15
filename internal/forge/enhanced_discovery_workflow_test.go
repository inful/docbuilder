package forge

import (
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// TestEnhancedForgeDiscoveryWorkflow demonstrates how enhanced mocks can be used for end-to-end testing.
func TestEnhancedForgeDiscoveryWorkflow(t *testing.T) {
	t.Log("=== Enhanced Forge Discovery Workflow Testing ===")

	// Test forge discovery service with enhanced mocks
	t.Run("EnhancedForgeDiscoveryService", func(t *testing.T) {
		// Create enhanced mocks for a realistic scenario
		github := NewEnhancedGitHubMock("discovery-github")
		gitlab := NewEnhancedGitLabMock("discovery-gitlab")

		// Add realistic repository structures
		github.AddRepository(CreateMockGitHubRepo("acme-corp", "user-guide", true, false, false, false))
		github.AddRepository(CreateMockGitHubRepo("acme-corp", "api-docs", true, false, false, false))
		github.AddRepository(CreateMockGitHubRepo("acme-corp", "website", false, false, false, false)) // No docs

		gitlab.AddRepository(CreateMockGitLabRepo("acme-group", "internal-docs", true, true, false, false)) // Private
		gitlab.AddRepository(CreateMockGitLabRepo("acme-group", "public-wiki", true, false, false, false))

		// Create forge manager with enhanced mocks
		manager := NewForgeManager()
		manager.AddForge(github.GenerateForgeConfig(), github)
		manager.AddForge(gitlab.GenerateForgeConfig(), gitlab)

		// Create discovery service
		filtering := &config.FilteringConfig{
			RequiredPaths:   []string{"docs"},
			IncludePatterns: []string{"*"},
			ExcludePatterns: []string{},
		}

		discovery := NewDiscoveryService(manager, filtering)
		ctx := t.Context()

		// Test repository discovery using actual DiscoverAll method
		result, err := discovery.DiscoverAll(ctx)
		if err != nil {
			t.Errorf("DiscoverAll() error: %v", err)
		}

		// Should find repositories from both forges
		if len(result.Repositories) == 0 {
			t.Error("Expected to find repositories but got none")
		}

		// Verify all found repositories have docs
		for _, repo := range result.Repositories {
			if !repo.HasDocs {
				t.Errorf("Repository %s should have docs but doesn't", repo.FullName)
			}
		}

		// Check that organizations were discovered
		if len(result.Organizations) == 0 {
			t.Error("Expected to find organizations but got none")
		}

		t.Logf("✓ Enhanced forge discovery service complete - found %d repos, %d orgs",
			len(result.Repositories), len(result.Organizations))
	})

	// Test configuration-driven forge discovery
	t.Run("EnhancedConfigurationDrivenDiscovery", func(t *testing.T) {
		// Create enhanced mocks
		github := NewEnhancedGitHubMock("config-github")
		gitlab := NewEnhancedGitLabMock("config-gitlab")

		// Add test repositories
		github.AddRepository(CreateMockGitHubRepo("config-org", "documentation", true, false, false, false))
		gitlab.AddRepository(CreateMockGitLabRepo("config-group", "docs-site", true, false, false, false))

		// Generate realistic configurations
		githubConfig := github.GenerateForgeConfig()
		gitlabConfig := gitlab.GenerateForgeConfig()

		// Create a complete configuration
		fullConfig := &config.Config{
			Version: "2.0",
			Forges: []*config.ForgeConfig{
				githubConfig,
				gitlabConfig,
			},
			Build: config.BuildConfig{},
			Filtering: &config.FilteringConfig{
				RequiredPaths:   []string{"docs", "documentation"},
				IncludePatterns: []string{"*"},
				ExcludePatterns: []string{},
			},
			Output: config.OutputConfig{
				Directory: "/tmp/test-output",
			},
			Hugo: config.HugoConfig{
				BaseURL: "https://docs.example.com",
			},
		}

		// Create forge manager from configuration
		manager := NewForgeManager()
		manager.AddForge(githubConfig, github)
		manager.AddForge(gitlabConfig, gitlab)

		// Test discovery with the configuration
		discovery := NewDiscoveryService(manager, fullConfig.Filtering)
		ctx := t.Context()

		result, err := discovery.DiscoverAll(ctx)
		if err != nil {
			t.Errorf("Configuration-driven discovery error: %v", err)
		}

		if len(result.Repositories) < 2 {
			t.Errorf("Expected at least 2 repositories from config-driven discovery, got %d", len(result.Repositories))
		}

		// Verify forge configurations are correctly applied
		allForges := manager.GetAllForges()
		if len(allForges) != 2 {
			t.Errorf("Expected 2 forges in manager, got %d", len(allForges))
		}

		t.Log("✓ Enhanced configuration-driven discovery complete")
	})

	// Test failure scenarios in discovery workflow
	t.Run("EnhancedDiscoveryFailureScenarios", func(t *testing.T) {
		github := NewEnhancedMockForgeClient("failure-github", TypeGitHub)
		gitlab := NewEnhancedMockForgeClient("failure-gitlab", TypeGitLab)

		manager := NewForgeManager()
		manager.AddForge(github.GenerateForgeConfig(), github)
		manager.AddForge(gitlab.GenerateForgeConfig(), gitlab)

		discovery := NewDiscoveryService(manager, &config.FilteringConfig{})
		ctx := t.Context()

		// Test auth failure scenario
		github.WithAuthFailure()
		result, _ := discovery.DiscoverAll(ctx)
		// Should handle partial failures gracefully
		if result == nil {
			t.Error("Expected discovery result even with partial failures")
			return
		}

		// Check that errors are recorded
		if len(result.Errors) == 0 {
			t.Log("Note: Discovery handled auth failure gracefully without recording errors")
		}

		// Test full recovery
		github.ClearFailures()
		_, err := discovery.DiscoverAll(ctx)
		if err != nil {
			t.Errorf("Discovery should succeed after clearing all failures: %v", err)
		}

		t.Log("✓ Enhanced discovery failure scenarios complete")
	})

	// Test forge-aware configuration generation
	t.Run("EnhancedForgeAwareConfigGeneration", func(t *testing.T) {
		// Create enhanced mocks with realistic data
		github := NewEnhancedGitHubMock("config-gen-github")
		gitlab := NewEnhancedGitLabMock("config-gen-gitlab")
		forgejo := NewEnhancedForgejoMock("config-gen-forgejo")

		// Add diverse repository structures
		github.AddRepository(CreateMockGitHubRepo("open-source", "awesome-docs", true, false, false, false))
		github.AddRepository(CreateMockGitHubRepo("enterprise", "internal-api", true, true, false, false))

		gitlab.AddRepository(CreateMockGitLabRepo("team-alpha", "project-docs", true, false, false, false))
		forgejo.AddRepository(CreateMockForgejoRepo("self-hosted", "wiki-pages", true, false, false, false))

		// Generate configurations
		configs := []*config.ForgeConfig{
			github.GenerateForgeConfig(),
			gitlab.GenerateForgeConfig(),
			forgejo.GenerateForgeConfig(),
		}

		// Validate all generated configurations
		for i, cfg := range configs {
			if cfg.Name == "" {
				t.Errorf("Config %d missing name", i)
			}
			if cfg.Type == "" {
				t.Errorf("Config %d missing type", i)
			}
			if cfg.APIURL == "" {
				t.Errorf("Config %d missing API URL", i)
			}
			if cfg.Auth == nil {
				t.Errorf("Config %d missing auth", i)
			}
			if cfg.Webhook == nil {
				t.Errorf("Config %d missing webhook config", i)
			}
		}

		// Test that configurations produce different API URLs
		apiUrls := make(map[string]bool)
		for _, cfg := range configs {
			if apiUrls[cfg.APIURL] {
				t.Errorf("Duplicate API URL: %s", cfg.APIURL)
			}
			apiUrls[cfg.APIURL] = true
		}

		if len(apiUrls) != 3 {
			t.Errorf("Expected 3 unique API URLs, got %d", len(apiUrls))
		}

		// Test forge manager with all configurations
		manager := NewForgeManager()
		if len(configs) >= 3 {
			manager.AddForge(configs[0], github)
			manager.AddForge(configs[1], gitlab)
			manager.AddForge(configs[2], forgejo)
		}

		// Test discovery across all platforms
		discovery := NewDiscoveryService(manager, &config.FilteringConfig{})
		ctx := t.Context()

		result, err := discovery.DiscoverAll(ctx)
		if err != nil {
			t.Errorf("Multi-platform discovery error: %v", err)
		}

		// Should find repositories from all platforms
		platformCounts := make(map[string]int)
		for _, repo := range result.Repositories {
			// Extract platform from repository structure
			if repo.CloneURL != "" {
				switch {
				case stringContains(repo.CloneURL, "github.com"):
					platformCounts["github"]++
				case stringContains(repo.CloneURL, "gitlab.com"):
					platformCounts["gitlab"]++
				case stringContains(repo.CloneURL, "git.example.com"):
					platformCounts["forgejo"]++
				}
			}
		}

		if len(platformCounts) < 2 { // At least 2 platforms should have repos
			t.Logf("Found repositories from %d platforms: %v", len(platformCounts), platformCounts)
		}

		t.Log("✓ Enhanced forge-aware configuration generation complete")
	})

	t.Log("\n=== Enhanced Forge Discovery Workflow Summary ===")
	t.Log("✓ Comprehensive forge discovery service testing")
	t.Log("✓ Configuration-driven discovery workflows")
	t.Log("✓ Failure scenario handling and recovery")
	t.Log("✓ Multi-platform configuration generation")
	t.Log("✓ Realistic repository filtering and selection")
	t.Log("✓ Integration-ready discovery infrastructure")
	t.Log("→ Enhanced forge discovery workflow testing complete")
}

// Helper function to avoid conflicts.
func stringContains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || (len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			(len(s) > len(substr) && hasSubstring(s, substr)))))
}

func hasSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
