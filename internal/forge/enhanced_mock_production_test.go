package forge

import (
	"context"
	"testing"
	"time"
)

// TestEnhancedMockProductionSystem validates the production-ready enhanced mock system
func TestEnhancedMockProductionSystem(t *testing.T) {
	t.Log("=== Phase 3A: Production-Ready Enhanced Mock System Validation ===")

	t.Run("ProductionEnhancedMockCreation", func(t *testing.T) {
		// Test basic enhanced mock creation
		mock := NewEnhancedMockForgeClient("production-test", TypeGitHub)
		if mock == nil {
			t.Fatal("Failed to create enhanced mock")
		}

		if mock.GetName() != "production-test" {
			t.Errorf("Expected name 'production-test', got '%s'", mock.GetName())
		}

		if mock.GetType() != TypeGitHub {
			t.Errorf("Expected GitHub forge type, got %v", mock.GetType())
		}

		t.Log("✓ Production enhanced mock creation works")
	})

	t.Run("ProductionRepositoryFactory", func(t *testing.T) {
		// Test repository factory functions
		githubRepo := CreateMockGitHubRepo("test-org", "test-repo", true, false, false, false)
		if githubRepo.Name != "test-repo" {
			t.Errorf("Expected repo name 'test-repo', got '%s'", githubRepo.Name)
		}
		if githubRepo.FullName != "test-org/test-repo" {
			t.Errorf("Expected full name 'test-org/test-repo', got '%s'", githubRepo.FullName)
		}

		gitlabRepo := CreateMockGitLabRepo("test-group", "test-project", true, false, false, false)
		if gitlabRepo.Name != "test-project" {
			t.Errorf("Expected repo name 'test-project', got '%s'", gitlabRepo.Name)
		}

		forgejoRepo := CreateMockForgejoRepo("self-hosted", "wiki", true, false, false, false)
		if forgejoRepo.Name != "wiki" {
			t.Errorf("Expected repo name 'wiki', got '%s'", forgejoRepo.Name)
		}

		t.Log("✓ Production repository factory functions work")
	})

	t.Run("ProductionBuilderPattern", func(t *testing.T) {
		// Test fluent builder pattern
		mock := NewEnhancedMockBuilder("builder-test", TypeGitLab).
			WithRepositories(
				CreateMockGitLabRepo("company", "docs", true, false, false, false),
				CreateMockGitLabRepo("company", "api", true, false, false, false),
			).
			WithOrganizations(CreateMockGitLabGroup("company")).
			WithDelay(time.Millisecond * 10).
			Build()

		ctx := context.Background()
		repos, err := mock.ListRepositories(ctx, []string{"company"})
		if err != nil {
			t.Fatalf("Failed to list repositories: %v", err)
		}

		if len(repos) != 2 {
			t.Errorf("Expected 2 repositories, got %d", len(repos))
		}

		orgs, err := mock.ListOrganizations(ctx)
		if err != nil {
			t.Fatalf("Failed to list organizations: %v", err)
		}

		if len(orgs) != 1 {
			t.Errorf("Expected 1 organization, got %d", len(orgs))
		}

		t.Log("✓ Production builder pattern works")
	})

	t.Run("ProductionFailureSimulation", func(t *testing.T) {
		// Test failure simulation capabilities
		mock := NewEnhancedMockForgeClient("failure-test", TypeGitHub)
		ctx := context.Background()

		// Test auth failure
		mock.WithAuthFailure()
		_, err := mock.ListRepositories(ctx, []string{})
		if err == nil {
			t.Error("Expected auth failure, got nil")
		}
		if err.Error() != "authentication failed: invalid credentials" {
			t.Errorf("Expected auth failure message, got: %s", err.Error())
		}

		// Test rate limiting
		mock.ClearFailures()
		mock.WithRateLimit(100, time.Hour)
		_, err = mock.ListOrganizations(ctx)
		if err == nil {
			t.Error("Expected rate limit failure, got nil")
		}

		// Test network timeout
		mock.ClearFailures()
		mock.WithNetworkTimeout(time.Millisecond * 50)
		_, err = mock.ListRepositories(ctx, []string{})
		if err == nil {
			t.Error("Expected network timeout, got nil")
		}

		t.Log("✓ Production failure simulation works")
	})

	t.Run("ProductionRealisticMocks", func(t *testing.T) {
		// Test realistic mock creation
		githubMock := CreateRealisticGitHubMock("realistic-github")
		gitlabMock := CreateRealisticGitLabMock("realistic-gitlab")
		forgejoMock := CreateRealisticForgejoMock("realistic-forgejo")

		ctx := context.Background()

		// Validate GitHub mock
		githubRepos, err := githubMock.ListRepositories(ctx, []string{"company"})
		if err != nil {
			t.Fatalf("Failed to list GitHub repositories: %v", err)
		}
		if len(githubRepos) != 3 {
			t.Errorf("Expected 3 GitHub repositories, got %d", len(githubRepos))
		}

		// Validate GitLab mock
		gitlabRepos, err := gitlabMock.ListRepositories(ctx, []string{"team"})
		if err != nil {
			t.Fatalf("Failed to list GitLab repositories: %v", err)
		}
		if len(gitlabRepos) != 3 {
			t.Errorf("Expected 3 GitLab repositories, got %d", len(gitlabRepos))
		}

		// Validate Forgejo mock
		forgejoRepos, err := forgejoMock.ListRepositories(ctx, []string{"self-hosted"})
		if err != nil {
			t.Fatalf("Failed to list Forgejo repositories: %v", err)
		}
		if len(forgejoRepos) != 3 {
			t.Errorf("Expected 3 Forgejo repositories, got %d", len(forgejoRepos))
		}

		t.Log("✓ Production realistic mocks work")
	})

	t.Run("ProductionConfigGeneration", func(t *testing.T) {
		// Test configuration generation
		mock := NewEnhancedMockForgeClient("config-test", TypeGitHub)
		mock.AddOrganization(CreateMockGitHubOrg("test-org"))

		config := mock.GenerateForgeConfig()
		if config.Name != "config-test" {
			t.Errorf("Expected config name 'config-test', got '%s'", config.Name)
		}

		if config.Type != TypeGitHub {
			t.Errorf("Expected GitHub config type, got %v", config.Type)
		}

		if len(config.Organizations) != 1 {
			t.Errorf("Expected 1 organization in config, got %d", len(config.Organizations))
		}

		t.Log("✓ Production configuration generation works")
	})

	t.Run("ProductionBulkRepositoryCreation", func(t *testing.T) {
		// Test bulk repository creation
		repos := CreateMockRepositorySet(TypeGitHub, "bulk-org", 10)
		if len(repos) != 10 {
			t.Errorf("Expected 10 repositories, got %d", len(repos))
		}

		// Validate properties
		docsCount := 0
		privateCount := 0
		for _, repo := range repos {
			if repo.HasDocs {
				docsCount++
			}
			if repo.Private {
				privateCount++
			}
		}

		if docsCount != 5 {
			t.Errorf("Expected 5 repos with docs (50%%), got %d", docsCount)
		}

		if privateCount != 2 {
			t.Errorf("Expected 2 private repos (20%%), got %d", privateCount)
		}

		t.Log("✓ Production bulk repository creation works")
	})

	// Summary
	t.Log("")
	t.Log("=== Phase 3A Production Enhanced Mock System Summary ===")
	t.Log("✓ Enhanced mock client creation and configuration")
	t.Log("✓ Repository factory functions for all forge types")
	t.Log("✓ Fluent builder pattern for complex mock setup")
	t.Log("✓ Comprehensive failure simulation capabilities")
	t.Log("✓ Realistic mock data generation")
	t.Log("✓ Configuration generation for testing")
	t.Log("✓ Bulk repository creation utilities")
	t.Log("→ Phase 3A: Enhanced mock system successfully exported to production")
}

// TestEnhancedMockCompatibility validates that the production enhanced mocks are compatible
func TestEnhancedMockCompatibility(t *testing.T) {
	t.Log("=== Enhanced Mock Compatibility Testing ===")

	t.Run("ForgeClientInterface", func(t *testing.T) {
		// Ensure enhanced mock implements ForgeClient interface
		var client Client = NewEnhancedMockForgeClient("interface-test", TypeGitHub)

		ctx := context.Background()

		// Test interface methods
		if client.GetType() != TypeGitHub {
			t.Error("GetType() interface method failed")
		}

		if client.GetName() != "interface-test" {
			t.Error("GetName() interface method failed")
		}

		_, err := client.ListOrganizations(ctx)
		if err != nil {
			t.Errorf("ListOrganizations() interface method failed: %v", err)
		}

		_, err = client.ListRepositories(ctx, []string{})
		if err != nil {
			t.Errorf("ListRepositories() interface method failed: %v", err)
		}

		t.Log("✓ Enhanced mock implements ForgeClient interface correctly")
	})

	t.Log("→ Enhanced mock system is fully compatible with existing interfaces")
}
