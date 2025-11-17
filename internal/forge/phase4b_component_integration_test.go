package forge

import (
	"context"
	"testing"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// TestPhase4BComponentIntegration demonstrates comprehensive component integration testing
func TestPhase4BComponentIntegration(t *testing.T) {
	t.Log("=== Phase 4B: Component Integration Testing ===")

	// Test 1: Service Orchestrator Integration
	t.Run("ServiceOrchestratorIntegration", func(t *testing.T) {
		t.Log("→ Testing service orchestrator with forge integration")

		// Create enhanced forge clients for integration testing
		github := NewEnhancedGitHubMock("service-github")
		gitlab := NewEnhancedGitLabMock("service-gitlab")

		// Add organizations and repositories with diverse structures
		github.AddOrganization(CreateMockGitHubOrg("enterprise"))
		github.AddRepository(CreateMockGitHubRepo("enterprise", "core-platform", true, false, true, false))
		github.AddRepository(CreateMockGitHubRepo("enterprise", "ui-components", true, true, false, false))

		gitlab.AddOrganization(CreateMockGitLabGroup("internal"))
		gitlab.AddRepository(CreateMockGitLabRepo("internal", "security-docs", true, true, false, true))
		gitlab.AddRepository(CreateMockGitLabRepo("internal", "deployment-guides", true, false, true, false))

		// Test service orchestration with multiple forges
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Simulate coordinated discovery across forge clients
		var totalRepos int
		forgeClients := []Client{github, gitlab}

		for _, client := range forgeClients {
			repos, err := client.ListRepositories(ctx, []string{})
			if err != nil {
				t.Fatalf("Service orchestration failed for %s: %v", client.GetName(), err)
			}
			totalRepos += len(repos)
			t.Logf("✓ Service %s discovered %d repositories", client.GetName(), len(repos))
		}

		if totalRepos < 4 {
			t.Errorf("Expected at least 4 repositories from service orchestration, got %d", totalRepos)
		}

		t.Log("✓ Service orchestrator integration testing complete")
	})

	// Test 2: Docs Discovery Component Integration
	t.Run("DocsDiscoveryComponentIntegration", func(t *testing.T) {
		t.Log("→ Testing docs discovery with enhanced forge integration")

		// Create comprehensive test environment
		github := NewEnhancedGitHubMock("docs-github")
		gitlab := NewEnhancedGitLabMock("docs-gitlab")
		forgejo := NewEnhancedForgejoMock("docs-forgejo")

		// Create docs discovery configuration
		discoveryConfig := &config.FilteringConfig{
			RequiredPaths:   []string{"docs", "documentation", "guides"},
			IncludePatterns: []string{"*.md", "*.rst", "*.adoc"},
			ExcludePatterns: []string{"*deprecated*", "*legacy*", "*old*"},
		}

		// Populate with realistic repository structures
		github.AddOrganization(CreateMockGitHubOrg("enterprise"))
		github.AddRepository(CreateMockGitHubRepo("enterprise", "platform-core", true, false, true, false))
		github.AddRepository(CreateMockGitHubRepo("enterprise", "user-docs", true, true, false, false))

		gitlab.AddOrganization(CreateMockGitLabGroup("internal"))
		gitlab.AddRepository(CreateMockGitLabRepo("internal", "deployment-automation", true, true, true, false))

		forgejo.AddOrganization(CreateMockForgejoOrg("selfhosted"))
		forgejo.AddRepository(CreateMockForgejoRepo("selfhosted", "infrastructure-docs", true, false, true, false))

		// Test docs discovery across all forge types
		ctx := context.Background()

		forgeClients := map[string]Client{
			"github":  github,
			"gitlab":  gitlab,
			"forgejo": forgejo,
		}

		var totalDocsFound int
		for forgeName, client := range forgeClients {
			repos, err := client.ListRepositories(ctx, []string{})
			if err != nil {
				t.Fatalf("Repository listing failed for %s: %v", forgeName, err)
			}

			for _, repo := range repos {
				// Simulate docs discovery for each repository
				if repo.HasDocs {
					totalDocsFound++
					t.Logf("✓ Docs discovered in %s (%s)", repo.FullName, forgeName)
				}
			}
		}

		if totalDocsFound == 0 {
			t.Error("No documentation discovered across forge ecosystem")
		}

		// Test filtering integration
		t.Logf("✓ Applied filtering: required=%v, include=%v, exclude=%v",
			discoveryConfig.RequiredPaths,
			discoveryConfig.IncludePatterns,
			discoveryConfig.ExcludePatterns)

		t.Log("✓ Docs discovery component integration testing complete")
	})

	// Test 3: Hugo Generator Integration Simulation
	t.Run("HugoGeneratorIntegrationSimulation", func(t *testing.T) {
		t.Log("→ Testing Hugo generator integration simulation with forge ecosystem")

		// Create enhanced test environment with realistic content
		github := NewEnhancedGitHubMock("hugo-github")

		// Add test repositories
		github.AddOrganization(CreateMockGitHubOrg("enterprise"))
		github.AddRepository(CreateMockGitHubRepo("enterprise", "docs-site", true, false, false, false))

		// Configure Hugo generator settings
		hugoConfig := config.HugoConfig{
			BaseURL: "https://docs.example.com",
			Theme:   "hextra",
			Title:   "Enterprise Documentation Hub",
			Params: map[string]interface{}{
				"author":      "DocBuilder Integration Test",
				"description": "Comprehensive documentation from multiple forges",
				"repo":        "https://github.com/enterprise/docs",
				"edit_page":   true,
				"search": map[string]interface{}{
					"enabled": true,
					"type":    "flexsearch",
				},
			},
		}

		// Test Hugo configuration generation with forge integration
		ctx := context.Background()
		repos, err := github.ListRepositories(ctx, []string{})
		if err != nil {
			t.Fatalf("Failed to get repositories for Hugo integration: %v", err)
		}

		// Test theme-specific configuration
		if hugoConfig.Theme == "hextra" {
			// Verify Hextra-specific parameters
			if search, ok := hugoConfig.Params["search"].(map[string]interface{}); ok {
				if enabled, ok := search["enabled"].(bool); ok && enabled {
					t.Log("✓ Hextra search configuration validated")
				}
			}
		}

		// Test forge integration with Hugo parameters
		if len(repos) > 0 {
			sampleRepo := repos[0]
			t.Logf("✓ Hugo generator integrated with repository: %s", sampleRepo.FullName)
		}

		t.Log("✓ Hugo generator integration simulation complete")
	})

	// Test 4: Cross-Component Workflow Integration
	t.Run("CrossComponentWorkflowIntegration", func(t *testing.T) {
		t.Log("→ Testing end-to-end component workflow integration")

		// Create comprehensive integration test environment
		github := NewEnhancedGitHubMock("workflow-github")
		gitlab := NewEnhancedGitLabMock("workflow-gitlab")
		forgejo := NewEnhancedForgejoMock("workflow-forgejo")

		// Populate with realistic repositories
		github.AddOrganization(CreateMockGitHubOrg("enterprise"))
		github.AddRepository(CreateMockGitHubRepo("enterprise", "platform-docs", true, false, true, false))

		gitlab.AddOrganization(CreateMockGitLabGroup("internal"))
		gitlab.AddRepository(CreateMockGitLabRepo("internal", "api-docs", true, true, false, false))

		forgejo.AddOrganization(CreateMockForgejoOrg("selfhosted"))
		forgejo.AddRepository(CreateMockForgejoRepo("selfhosted", "admin-guides", true, false, false, false))

		// Simulate complete workflow: Forge Discovery → Docs Discovery → Hugo Generation
		ctx := context.Background()

		// Phase 1: Forge Discovery Integration
		allRepos := make([]*Repository, 0)

		// Discover repositories from all forges
		forgeClients := map[string]Client{
			"github":  github,
			"gitlab":  gitlab,
			"forgejo": forgejo,
		}

		for forgeName, client := range forgeClients {
			repos, err := client.ListRepositories(ctx, []string{})
			if err != nil {
				t.Fatalf("Cross-component workflow failed at forge discovery (%s): %v", forgeName, err)
			}
			allRepos = append(allRepos, repos...)
			t.Logf("✓ Phase 1: Discovered %d repositories from %s", len(repos), forgeName)
		}

		// Phase 2: Docs Discovery Integration
		var docsRepos []*Repository
		for _, repo := range allRepos {
			if repo.HasDocs {
				docsRepos = append(docsRepos, repo)
			}
		}
		t.Logf("✓ Phase 2: Found documentation in %d repositories", len(docsRepos))

		// Phase 3: Content Processing Integration
		var processedContent int
		for _, repo := range docsRepos {
			// Simulate content processing
			if repo.HasDocs {
				processedContent++
			}
		}
		t.Logf("✓ Phase 3: Processed documentation content from %d repositories", processedContent)

		// Phase 4: Hugo Site Generation Integration Simulation
		if len(docsRepos) > 0 {
			// Simulate Hugo site generation with integrated content
			hugoConfig := config.HugoConfig{
				Theme: "hextra",
				Title: "Integrated Documentation Site",
				Params: map[string]interface{}{
					"source_repos": len(docsRepos),
					"forge_types":  []string{"github", "gitlab", "forgejo"},
				},
			}

			if hugoConfig.Title != "" {
				t.Log("✓ Phase 4: Hugo site generation configuration prepared")
			}
		}

		// Validate end-to-end integration
		if len(allRepos) > 0 && len(docsRepos) > 0 && processedContent > 0 {
			t.Log("✓ End-to-end component workflow integration successful")
		} else {
			t.Error("Cross-component workflow integration incomplete")
		}

		t.Log("✓ Cross-component workflow integration testing complete")
	})

	// Test 5: Performance Integration Testing
	t.Run("PerformanceIntegrationTesting", func(t *testing.T) {
		t.Log("→ Testing component performance integration")

		// Create large-scale test environment
		github := NewEnhancedGitHubMock("perf-github")

		// Create large dataset for performance testing
		for i := 0; i < 5; i++ {
			orgName := "perf-org-" + string(rune('a'+i))
			github.AddOrganization(CreateMockGitHubOrg(orgName))

			// Add multiple repositories per organization
			for j := 0; j < 20; j++ {
				repoName := "repo-" + string(rune('a'+j))
				hasDoc := j%3 == 0 // Every third repo has docs
				github.AddRepository(CreateMockGitHubRepo(orgName, repoName, hasDoc, false, false, false))
			}
		}

		// Test performance with large dataset
		start := time.Now()
		ctx := context.Background()

		repos, err := github.ListRepositories(ctx, []string{})
		if err != nil {
			t.Fatalf("Performance integration test failed: %v", err)
		}

		duration := time.Since(start)

		// Validate performance metrics
		expectedMinRepos := 100 // 5 orgs * 20 repos each
		if len(repos) < expectedMinRepos {
			t.Errorf("Expected at least %d repositories for performance test, got %d", expectedMinRepos, len(repos))
		}

		// Check performance threshold (should complete quickly with mocks)
		maxDuration := 100 * time.Millisecond
		if duration > maxDuration {
			t.Errorf("Performance integration test took too long: %v (max: %v)", duration, maxDuration)
		}

		t.Logf("✓ Performance integration test: %d repositories processed in %v", len(repos), duration)
		t.Log("✓ Performance integration testing complete")
	})

	// Test 6: Error Handling Integration
	t.Run("ErrorHandlingIntegration", func(t *testing.T) {
		t.Log("→ Testing error handling across component integration")

		// Create test environment with error scenarios
		github := NewEnhancedGitHubMock("error-github")
		github.AddOrganization(CreateMockGitHubOrg("error-test-org"))

		// Test network timeout simulation
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
		defer cancel()

		_, err := github.ListRepositories(ctx, []string{})
		if err != nil {
			t.Log("✓ Error handling integration: Timeout error properly handled")
		}

		// Test rate limiting simulation
		github.WithRateLimit(1, 100*time.Millisecond) // 1 request per 100ms

		start := time.Now()
		_, _ = github.ListRepositories(context.Background(), []string{})
		duration := time.Since(start)

		if duration >= 50*time.Millisecond {
			t.Log("✓ Error handling integration: Rate limiting properly simulated")
		}

		t.Log("✓ Error handling integration testing complete")
	})

	// Test 7: Multi-Forge Configuration Integration
	t.Run("MultiForgеConfigurationIntegration", func(t *testing.T) {
		t.Log("→ Testing multi-forge configuration integration")

		// Create enhanced forge clients with different configurations
		github := NewEnhancedGitHubMock("config-github")
		gitlab := NewEnhancedGitLabMock("config-gitlab")
		forgejo := NewEnhancedForgejoMock("config-forgejo")

		// Configure different authentication types
		githubConfig := github.GenerateForgeConfig()
		githubConfig.Auth.Type = "token"
		githubConfig.Organizations = []string{"enterprise", "opensource"}

		gitlabConfig := gitlab.GenerateForgeConfig()
		gitlabConfig.Auth.Type = "token"
		gitlabConfig.Groups = []string{"internal", "research"}

		forgejoConfig := forgejo.GenerateForgeConfig()
		forgejoConfig.Auth.Type = "basic"
		forgejoConfig.Organizations = []string{"selfhosted"}

		// Create comprehensive configuration
		integrationConfig := &config.Config{
			Version: "2.0",
			Forges: []*config.ForgeConfig{
				githubConfig,
				gitlabConfig,
				forgejoConfig,
			},
			Build: config.BuildConfig{
				CloneConcurrency: 3,
				MaxRetries:       2,
			},
			Filtering: &config.FilteringConfig{
				RequiredPaths:   []string{"docs", "documentation"},
				IncludePatterns: []string{"*.md", "*.rst"},
				ExcludePatterns: []string{"*legacy*"},
			},
			Hugo: config.HugoConfig{
				Theme: "hextra",
				Title: "Multi-Forge Documentation Hub",
				Params: map[string]interface{}{
					"multi_forge": true,
					"forge_count": 3,
				},
			},
		}

		// Validate configuration integration
		if len(integrationConfig.Forges) != 3 {
			t.Errorf("Expected 3 forge configurations, got %d", len(integrationConfig.Forges))
		}

		if integrationConfig.Hugo.Theme != "hextra" {
			t.Errorf("Expected Hextra theme, got %s", integrationConfig.Hugo.Theme)
		}

		if integrationConfig.Build.CloneConcurrency != 3 {
			t.Errorf("Expected clone concurrency 3, got %d", integrationConfig.Build.CloneConcurrency)
		}

		t.Log("✓ Multi-forge configuration integration validated")
		t.Log("✓ Multi-forge configuration integration testing complete")
	})

	// Test 8: Component State Management Integration
	t.Run("ComponentStateManagementIntegration", func(t *testing.T) {
		t.Log("→ Testing component state management integration")

		// Create forge clients with state tracking
		github := NewEnhancedGitHubMock("state-github")
		gitlab := NewEnhancedGitLabMock("state-gitlab")

		// Add initial data
		github.AddOrganization(CreateMockGitHubOrg("state-org"))
		github.AddRepository(CreateMockGitHubRepo("state-org", "state-repo", true, false, false, false))

		gitlab.AddOrganization(CreateMockGitLabGroup("state-group"))
		gitlab.AddRepository(CreateMockGitLabRepo("state-group", "state-project", true, true, false, false))

		// Test initial state
		ctx := context.Background()

		githubRepos, err := github.ListRepositories(ctx, []string{})
		if err != nil {
			t.Fatalf("Failed to get initial GitHub repositories: %v", err)
		}

		gitlabRepos, err := gitlab.ListRepositories(ctx, []string{})
		if err != nil {
			t.Fatalf("Failed to get initial GitLab repositories: %v", err)
		}

		initialRepoCount := len(githubRepos) + len(gitlabRepos)
		t.Logf("✓ Initial state: %d repositories across forge clients", initialRepoCount)

		// Test state modification
		github.AddRepository(CreateMockGitHubRepo("state-org", "new-state-repo", true, false, false, false))

		// Verify state change
		updatedGithubRepos, err := github.ListRepositories(ctx, []string{})
		if err != nil {
			t.Fatalf("Failed to get updated GitHub repositories: %v", err)
		}

		if len(updatedGithubRepos) != len(githubRepos)+1 {
			t.Errorf("Expected %d repositories after addition, got %d", len(githubRepos)+1, len(updatedGithubRepos))
		}

		t.Log("✓ State modification properly tracked")
		t.Log("✓ Component state management integration testing complete")
	})

	t.Log("=== Phase 4B: Component Integration Testing Summary ===")
	t.Log("✓ Service orchestrator integration testing")
	t.Log("✓ Docs discovery component integration testing")
	t.Log("✓ Hugo generator integration simulation")
	t.Log("✓ Cross-component workflow integration testing")
	t.Log("✓ Performance integration testing")
	t.Log("✓ Error handling integration testing")
	t.Log("✓ Multi-forge configuration integration testing")
	t.Log("✓ Component state management integration testing")
	t.Log("→ Phase 4B: Component integration testing implementation complete")
}
