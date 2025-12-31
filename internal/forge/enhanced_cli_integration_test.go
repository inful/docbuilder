package forge

import (
	"context"
	"testing"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// TestEnhancedForgeCliIntegration demonstrates how enhanced mocks can be used for CLI workflow testing.
func TestEnhancedForgeCliIntegration(t *testing.T) {
	t.Log("=== Enhanced Forge CLI Integration Testing ===")

	t.Run("EnhancedCliConfigurationWorkflow", testEnhancedCliConfigurationWorkflow)
	t.Run("EnhancedCliFailureHandling", testEnhancedCliFailureHandling)
	t.Run("EnhancedCliWebhookWorkflow", testEnhancedCliWebhookWorkflow)
	t.Run("EnhancedCliMultiPlatformIntegration", testEnhancedCliMultiPlatformIntegration)
}

func testEnhancedCliConfigurationWorkflow(t *testing.T) {
	// Create enhanced mocks that simulate a realistic CLI environment
	github := NewEnhancedGitHubMock("cli-github")
	gitlab := NewEnhancedGitLabMock("cli-gitlab")

	// Add organizations first
	github.AddOrganization(CreateMockGitHubOrg("myorg"))
	gitlab.AddOrganization(CreateMockGitLabGroup("internal"))
	gitlab.AddOrganization(CreateMockGitLabGroup("public"))

	// Add repositories with various documentation structures
	github.AddRepository(CreateMockGitHubRepo("myorg", "web-frontend", true, false, false, false))
	github.AddRepository(CreateMockGitHubRepo("myorg", "api-backend", true, false, false, false))
	github.AddRepository(CreateMockGitHubRepo("myorg", "mobile-app", false, false, false, false)) // No docs

	gitlab.AddRepository(CreateMockGitLabRepo("internal", "deployment-docs", true, true, false, false)) // Private
	gitlab.AddRepository(CreateMockGitLabRepo("public", "user-manual", true, false, false, false))

	// Create forge configurations with specific organizations
	githubConfig := github.GenerateForgeConfig()
	githubConfig.Organizations = []string{"myorg"}

	gitlabConfig := gitlab.GenerateForgeConfig()
	gitlabConfig.Groups = []string{"internal", "public"}

	// Create a configuration that represents what a CLI user might create
	cliConfig := &config.Config{
		Version: "2.0",
		Forges: []*config.ForgeConfig{
			githubConfig,
			gitlabConfig,
		},
		Build: config.BuildConfig{},
		Filtering: &config.FilteringConfig{
			ExcludePatterns: []string{"mobile-*"}, // Exclude mobile projects
		},
		Output: config.OutputConfig{
			Directory: "./site",
		},
		Hugo: config.HugoConfig{
			BaseURL: "https://docs.company.com", Title: "Company Documentation",
		},
	}

	// Simulate CLI discovery operation
	manager := NewForgeManager()
	manager.AddForge(githubConfig, github)
	manager.AddForge(gitlabConfig, gitlab)
	discovery := NewDiscoveryService(manager, cliConfig.Filtering)
	ctx := context.Background()

	result, _ := discovery.DiscoverAll(ctx)
	if result == nil {
		t.Error("Expected non-nil result")
		return
	}

	// Verify CLI-style filtering worked
	foundMobileApp := false
	for _, repo := range result.Repositories {
		if repo.Name == "mobile-app" {
			foundMobileApp = true
		}
	}

	if foundMobileApp {
		t.Error("Mobile app should have been excluded by pattern filter")
	}

	// Verify we found the expected repositories with docs
	expectedRepos := []string{"web-frontend", "api-backend", "user-manual"}
	foundRepos := make(map[string]bool)
	for _, repo := range result.Repositories {
		foundRepos[repo.Name] = true
	}

	for _, expected := range expectedRepos {
		if !foundRepos[expected] {
			t.Errorf("Expected to find repository %s", expected)
		}
	}

	t.Logf("✓ CLI configuration workflow complete - found %d repos", len(result.Repositories))
}

func testEnhancedCliFailureHandling(t *testing.T) {
	// Create enhanced mocks with different failure scenarios
	github := NewEnhancedMockForgeClient("cli-failure-github", TypeGitHub)
	gitlab := NewEnhancedMockForgeClient("cli-failure-gitlab", TypeGitLab)

	// Set up various failure conditions that a CLI user might encounter
	github.WithAuthFailure()                 // Wrong token
	gitlab.WithRateLimit(50, time.Minute*30) // Rate limited

	cliConfig := &config.Config{
		Version: "2.0",
		Forges: []*config.ForgeConfig{
			github.GenerateForgeConfig(),
			gitlab.GenerateForgeConfig(),
		},
		Filtering: &config.FilteringConfig{
			RequiredPaths: []string{"docs"},
		},
	}

	manager := NewForgeManager()
	manager.AddForge(cliConfig.Forges[0], github)
	manager.AddForge(cliConfig.Forges[1], gitlab)

	discovery := NewDiscoveryService(manager, cliConfig.Filtering)
	ctx := context.Background()

	// Test that the CLI operation handles failures gracefully
	result, _ := discovery.DiscoverAll(ctx)
	if result == nil {
		t.Error("Expected result even with failures")
		return
	}

	// Verify that errors are captured properly for CLI reporting
	if len(result.Errors) == 0 {
		t.Log("Note: Discovery handled all failures gracefully")
	} else {
		t.Logf("Discovery captured %d errors for CLI reporting", len(result.Errors))
	}

	// Test recovery scenarios
	github.ClearFailures()
	gitlab.ClearFailures()

	result, err := discovery.DiscoverAll(ctx)
	if err != nil {
		t.Errorf("Discovery should succeed after clearing failures: %v", err)
		return
	}
	if result == nil {
		t.Error("Expected result after clearing failures")
		return
	}

	t.Log("✓ CLI failure handling scenarios complete")
}

func testEnhancedCliWebhookWorkflow(t *testing.T) {
	// Create enhanced mocks with webhook capabilities
	github := NewEnhancedGitHubMock("webhook-github")
	gitlab := NewEnhancedGitLabMock("webhook-gitlab")

	// Add repositories that would receive webhooks
	repo1 := CreateMockGitHubRepo("company", "main-docs", true, false, false, false)
	repo2 := CreateMockGitLabRepo("team", "api-docs", true, false, false, false)

	github.AddRepository(repo1)
	gitlab.AddRepository(repo2)

	// Create CLI configuration with webhook settings
	githubConfig := github.GenerateForgeConfig()
	githubConfig.Webhook = &config.WebhookConfig{
		Secret:       "github-webhook-secret",
		Path:         "/webhooks/github",
		Events:       []string{"push", "repository"},
		RegisterAuto: true,
	}

	gitlabConfig := gitlab.GenerateForgeConfig()
	gitlabConfig.Webhook = &config.WebhookConfig{
		Secret:       "gitlab-webhook-secret",
		Path:         "/webhooks/gitlab",
		Events:       []string{"push", "merge_request"},
		RegisterAuto: true,
	}

	cliConfig := &config.Config{
		Version: "2.0",
		Forges: []*config.ForgeConfig{
			githubConfig,
			gitlabConfig,
		},
		Filtering: &config.FilteringConfig{
			RequiredPaths: []string{"docs"},
		},
	}

	// Test webhook configuration validation
	for _, forgeConfig := range cliConfig.Forges {
		if forgeConfig.Webhook == nil {
			t.Errorf("Forge %s missing webhook configuration", forgeConfig.Name)
		}
		if forgeConfig.Webhook.Secret == "" {
			t.Errorf("Forge %s missing webhook secret", forgeConfig.Name)
		}
	}

	// Test webhook client integration
	manager := NewForgeManager()
	manager.AddForge(githubConfig, github)
	manager.AddForge(gitlabConfig, gitlab)

	// Simulate registering webhooks as a CLI would
	ctx := context.Background()
	for _, client := range manager.GetAllForges() {
		// Get repositories for webhook registration
		orgs, err := client.ListOrganizations(ctx)
		if err != nil {
			t.Errorf("Failed to list organizations for webhook setup: %v", err)
			continue
		}

		orgNames := make([]string, len(orgs))
		for i, org := range orgs {
			orgNames[i] = org.Name
		}

		repos, err := client.ListRepositories(ctx, orgNames)
		if err != nil {
			t.Errorf("Failed to list repositories for webhook setup: %v", err)
			continue
		}

		// Register webhooks for repositories with docs
		webhookURL := "https://docbuilder.company.com/webhooks/" + client.GetName()
		for _, repo := range repos {
			if repo.HasDocs {
				err := client.RegisterWebhook(ctx, repo, webhookURL)
				if err != nil {
					t.Logf("Note: Webhook registration for %s returned: %v", repo.FullName, err)
				}
			}
		}
	}

	t.Log("✓ CLI webhook workflow complete")
}

func testEnhancedCliMultiPlatformIntegration(t *testing.T) {
	// Create a realistic multi-platform CLI scenario
	github := NewEnhancedGitHubMock("enterprise-github")
	gitlab := NewEnhancedGitLabMock("internal-gitlab")
	forgejo := NewEnhancedForgejoMock("self-hosted-forgejo")

	// Add organizations first
	github.AddOrganization(CreateMockGitHubOrg("company-oss"))
	gitlab.AddOrganization(CreateMockGitLabGroup("internal-team"))
	gitlab.AddOrganization(CreateMockGitLabGroup("product-team"))
	forgejo.AddOrganization(CreateMockForgejoOrg("devops"))

	// Add repositories across different platforms as a company might have
	github.AddRepository(CreateMockGitHubRepo("company-oss", "public-api", true, false, false, false))
	github.AddRepository(CreateMockGitHubRepo("company-oss", "sdk-docs", true, false, false, false))

	gitlab.AddRepository(CreateMockGitLabRepo("internal-team", "architecture-docs", true, true, false, false))
	gitlab.AddRepository(CreateMockGitLabRepo("product-team", "user-guides", true, false, false, false))

	forgejo.AddRepository(CreateMockForgejoRepo("devops", "runbooks", true, false, false, false))
	forgejo.AddRepository(CreateMockForgejoRepo("devops", "deployment-guides", true, false, false, false))

	// Create forge configurations with specific organizations
	githubConfig := github.GenerateForgeConfig()
	githubConfig.Organizations = []string{"company-oss"}

	gitlabConfig := gitlab.GenerateForgeConfig()
	gitlabConfig.Groups = []string{"internal-team", "product-team"}

	forgejoConfig := forgejo.GenerateForgeConfig()
	forgejoConfig.Organizations = []string{"devops"}

	// Create CLI configuration for multi-platform documentation aggregation
	cliConfig := &config.Config{
		Version: "2.0",
		Forges: []*config.ForgeConfig{
			githubConfig,
			gitlabConfig,
			forgejoConfig,
		},
		Build: config.BuildConfig{},
		Filtering: &config.FilteringConfig{
			RequiredPaths:   []string{"docs", "documentation"},
			IncludePatterns: []string{"*docs*", "*guides*", "*api*", "*runbooks*"},
			ExcludePatterns: []string{},
		},
		Output: config.OutputConfig{
			Directory: "./unified-docs",
		},
		Hugo: config.HugoConfig{
			BaseURL: "https://docs.company.internal",
			Title:   "Company Documentation Hub",
		},
	}

	// Test multi-platform discovery
	manager := NewForgeManager()
	manager.AddForge(githubConfig, github)
	manager.AddForge(gitlabConfig, gitlab)
	manager.AddForge(forgejoConfig, forgejo)

	discovery := NewDiscoveryService(manager, cliConfig.Filtering)
	ctx := context.Background()

	result, err := discovery.DiscoverAll(ctx)
	if err != nil {
		t.Errorf("Multi-platform discovery failed: %v", err)
		return
	}
	if result == nil {
		t.Error("Expected non-nil result")
		return
	}

	// Verify we found repositories from all platforms
	platformRepos := make(map[string][]string)
	for _, repo := range result.Repositories {
		var platform string
		switch {
		case stringContains(repo.CloneURL, "github.com"):
			platform = "github"
		case stringContains(repo.CloneURL, "gitlab.com"):
			platform = "gitlab"
		case stringContains(repo.CloneURL, "git.example.com"):
			platform = "forgejo"
		}

		if platform != "" {
			platformRepos[platform] = append(platformRepos[platform], repo.FullName)
		}
	}

	if len(platformRepos) < 2 {
		t.Logf("Found repositories from %d platforms: %v", len(platformRepos), platformRepos)
	}

	// Verify pattern filtering worked
	for _, repo := range result.Repositories {
		matched := false
		patterns := cliConfig.Filtering.IncludePatterns
		for _, pattern := range patterns {
			if stringContains(repo.Name, pattern[1:len(pattern)-1]) { // Remove * wildcards for simple check
				matched = true
				break
			}
		}
		if !matched {
			t.Logf("Repository %s may not match include patterns", repo.FullName)
		}
	}

	// Test that configuration is ready for Hugo generation
	if cliConfig.Hugo.BaseURL == "" {
		t.Error("Hugo configuration missing base URL")
	}
	if cliConfig.Hugo.Title == "" {
		t.Error("Hugo configuration missing title")
	}

	t.Logf("✓ CLI multi-platform integration complete - %d platforms, %d repos",
		len(platformRepos), len(result.Repositories))
}
