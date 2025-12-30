package forge

import (
	"context"
	"errors"
	"strings"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// TestRealForgeIntegration tests DocBuilder functionality using mock forges.
func TestRealForgeIntegration(t *testing.T) {
	t.Log("=== Real DocBuilder Forge Integration Testing ===")

	// Test single forge repository discovery
	t.Run("SingleForgeRepositoryDiscovery", func(t *testing.T) {
		// Create a mock forge with realistic repositories
		mockForge := NewMockForgeClient("test-github", TypeGitHub)

		// Add an organization
		mockForge.AddOrganization(&Organization{
			ID:          "123",
			Name:        "test-org",
			DisplayName: "Test Organization",
			Description: "Test organization for documentation",
			Type:        "organization",
		})

		// Add repositories - use names that work with mock CheckDocumentation logic
		mockForge.AddRepository(&Repository{
			ID:            "repo1",
			Name:          "api-docs", // Contains "docs" - will be detected as HasDocs=true
			FullName:      "test-org/api-docs",
			CloneURL:      "https://github.com/test-org/api-docs.git",
			SSHURL:        "git@github.com:test-org/api-docs.git",
			DefaultBranch: "main",
			Description:   "API documentation repository",
			Private:       false,
			Archived:      false,
			Topics:        []string{"api", "documentation"},
			Language:      "Markdown",
		})

		mockForge.AddRepository(&Repository{
			ID:            "repo2",
			Name:          "user-docs", // Contains "docs" - will be detected as HasDocs=true
			FullName:      "test-org/user-docs",
			CloneURL:      "https://github.com/test-org/user-docs.git",
			SSHURL:        "git@github.com:test-org/user-docs.git",
			DefaultBranch: "main",
			Description:   "User documentation",
			Private:       false,
			Archived:      false,
			Topics:        []string{"guide", "documentation"},
			Language:      "Markdown",
		})

		// Add a repository without documentation (name doesn't contain "docs")
		mockForge.AddRepository(&Repository{
			ID:            "repo3",
			Name:          "backend-service", // No "docs" in name - will be detected as HasDocs=false
			FullName:      "test-org/backend-service",
			CloneURL:      "https://github.com/test-org/backend-service.git",
			SSHURL:        "git@github.com:test-org/backend-service.git",
			DefaultBranch: "main",
			Description:   "Backend service without docs",
			Private:       false,
			Archived:      false,
			Topics:        []string{"backend", "service"},
			Language:      "Go",
		})

		// Create forge configuration
		forgeConfig := &config.ForgeConfig{
			Name:          "test-github",
			Type:          config.ForgeGitHub,
			BaseURL:       "https://github.com",
			APIURL:        "https://api.github.com",
			Organizations: []string{"test-org"},
			Auth: &config.AuthConfig{
				Type:  config.AuthTypeToken,
				Token: "test-token",
			},
		}

		// Create forge manager and add the mock forge
		manager := NewForgeManager()
		manager.AddForge(forgeConfig, mockForge)

		// Create discovery service with filtering that requires documentation
		filtering := &config.FilteringConfig{
			RequiredPaths: []string{"docs"}, // This will filter out repos without HasDocs=true
		}
		discovery := NewDiscoveryService(manager, filtering)
		ctx := context.Background()

		// Test the actual discovery process
		result, err := discovery.DiscoverAll(ctx)
		if err != nil {
			t.Fatalf("Discovery failed: %v", err)
		}

		// Verify discovery results - should find repositories with docs
		expectedRepos := 2 // Only repos with HasDocs=true
		if len(result.Repositories) != expectedRepos {
			t.Errorf("Expected %d repositories with docs, got %d", expectedRepos, len(result.Repositories))
		}

		// Verify that only repositories with documentation were discovered
		for _, repo := range result.Repositories {
			if !repo.HasDocs {
				t.Errorf("Repository %s should have documentation", repo.FullName)
			}
		}

		// Test conversion to config repositories
		configRepos := discovery.ConvertToConfigRepositories(result.Repositories, manager)

		if len(configRepos) != expectedRepos {
			t.Errorf("Expected %d config repositories, got %d", expectedRepos, len(configRepos))
		}

		// Verify the converted repositories have correct structure for DocBuilder
		for _, repo := range configRepos {
			if repo.URL == "" {
				t.Errorf("Repository %s missing URL", repo.Name)
			}
			if repo.Name == "" {
				t.Errorf("Repository missing name")
			}
			if repo.Branch == "" {
				t.Errorf("Repository %s missing branch", repo.Name)
			}

			// Verify URL is cloneable
			if !strings.HasPrefix(repo.URL, "https://") && !strings.HasPrefix(repo.URL, "git@") {
				t.Errorf("Repository %s has invalid clone URL: %s", repo.Name, repo.URL)
			}
		}

		t.Logf("✓ Successfully discovered %d repositories", len(result.Repositories))
		t.Logf("✓ Successfully converted %d repositories to DocBuilder config format", len(configRepos))
	})

	// Test multiple forges working together
	t.Run("MultiForgeDiscovery", func(t *testing.T) {
		// Create GitHub mock forge
		githubMock := NewMockForgeClient("github", TypeGitHub)
		githubMock.AddOrganization(&Organization{
			ID:   "github-org",
			Name: "github-org",
			Type: "organization",
		})
		githubMock.AddRepository(&Repository{
			ID:            "gh-repo1",
			Name:          "web-docs",
			FullName:      "github-org/web-docs",
			CloneURL:      "https://github.com/github-org/web-docs.git",
			DefaultBranch: "main",
			HasDocs:       true,
			Topics:        []string{"documentation", "web"},
		})

		// Create GitLab mock forge
		gitlabMock := NewMockForgeClient("gitlab", TypeGitLab)
		gitlabMock.AddOrganization(&Organization{
			ID:   "gitlab-group",
			Name: "gitlab-group",
			Type: "group",
		})
		gitlabMock.AddRepository(&Repository{
			ID:            "gl-repo1",
			Name:          "api-reference",
			FullName:      "gitlab-group/api-reference",
			CloneURL:      "https://gitlab.com/gitlab-group/api-reference.git",
			DefaultBranch: "main",
			HasDocs:       true,
			Topics:        []string{"api", "reference"},
		})

		// Create forge configurations
		githubConfig := &config.ForgeConfig{
			Name:          "github",
			Type:          config.ForgeGitHub,
			Organizations: []string{"github-org"},
			Auth: &config.AuthConfig{
				Type:  config.AuthTypeToken,
				Token: "github-token",
			},
		}

		gitlabConfig := &config.ForgeConfig{
			Name:   "gitlab",
			Type:   config.ForgeGitLab,
			Groups: []string{"gitlab-group"},
			Auth: &config.AuthConfig{
				Type:  config.AuthTypeToken,
				Token: "gitlab-token",
			},
		}

		// Create manager and add both forges
		manager := NewForgeManager()
		manager.AddForge(githubConfig, githubMock)
		manager.AddForge(gitlabConfig, gitlabMock)

		// Create discovery service with empty filtering
		discovery := NewDiscoveryService(manager, &config.FilteringConfig{})
		ctx := context.Background()

		// Run discovery
		result, err := discovery.DiscoverAll(ctx)
		if err != nil {
			t.Fatalf("Multi-forge discovery failed: %v", err)
		}

		// Should find repositories from both forges
		if len(result.Repositories) != 2 {
			t.Errorf("Expected 2 repositories from both forges, got %d", len(result.Repositories))
		}

		// Verify we have repositories from both forges
		foundGitHub := false
		foundGitLab := false
		for _, repo := range result.Repositories {
			if strings.Contains(repo.CloneURL, "github.com") {
				foundGitHub = true
			}
			if strings.Contains(repo.CloneURL, "gitlab.com") {
				foundGitLab = true
			}
		}

		if !foundGitHub {
			t.Error("Should have found GitHub repository")
		}
		if !foundGitLab {
			t.Error("Should have found GitLab repository")
		}

		// Test that converted repositories are ready for DocBuilder pipeline
		configRepos := discovery.ConvertToConfigRepositories(result.Repositories, manager)
		if len(configRepos) != 2 {
			t.Errorf("Expected 2 config repositories, got %d", len(configRepos))
		}

		t.Logf("✓ Successfully discovered repositories from %d forges", len(manager.GetAllForges()))
	})

	// Test filtering functionality
	t.Run("RepositoryFiltering", func(t *testing.T) {
		mockForge := NewMockForgeClient("test-filtering", TypeGitHub)

		mockForge.AddOrganization(&Organization{
			ID:   "test-org",
			Name: "test-org",
			Type: "organization",
		})

		// Add only one repo that should be included and one that should be excluded
		mockForge.AddRepository(&Repository{
			Name:     "api-docs", // Contains "docs" - should be included
			FullName: "test-org/api-docs",
			CloneURL: "https://github.com/test-org/api-docs.git",
			Topics:   []string{"api", "documentation"},
		})

		mockForge.AddRepository(&Repository{
			Name:     "backend-service", // No "docs" - should be filtered
			FullName: "test-org/backend-service",
			CloneURL: "https://github.com/test-org/backend-service.git",
			Topics:   []string{"backend", "service"},
		})

		forgeConfig := &config.ForgeConfig{
			Name:          "test-filtering",
			Type:          config.ForgeGitHub,
			Organizations: []string{"test-org"},
			Auth: &config.AuthConfig{
				Type:  config.AuthTypeToken,
				Token: "test-token",
			},
		}

		// Create filtering config - try pattern that matches the full name
		filtering := &config.FilteringConfig{
			IncludePatterns: []string{"*api*"}, // Should match "api-docs" or "test-org/api-docs"
		}

		manager := NewForgeManager()
		manager.AddForge(forgeConfig, mockForge)

		discovery := NewDiscoveryService(manager, filtering)
		ctx := context.Background()

		result, err := discovery.DiscoverAll(ctx)
		if err != nil {
			t.Fatalf("Filtered discovery failed: %v", err)
		}

		// Should find only the api-docs repository (matches pattern)
		if len(result.Repositories) != 1 {
			t.Errorf("Expected 1 repository with *docs* pattern, got %d", len(result.Repositories))
			for i, repo := range result.Repositories {
				t.Logf("Found Repository %d: %s, HasDocs=%v", i, repo.FullName, repo.HasDocs)
			}
			for i, repo := range result.Filtered {
				t.Logf("Filtered Repository %d: %s, HasDocs=%v", i, repo.FullName, repo.HasDocs)
			}
		}

		// Should have filtered repositories
		if len(result.Filtered) == 0 {
			t.Error("Expected some repositories to be filtered out")
		}

		t.Logf("✓ Filtering worked: %d included, %d filtered", len(result.Repositories), len(result.Filtered))
	})

	// Test error handling
	t.Run("ForgeErrorHandling", func(t *testing.T) {
		// Create a mock forge that will return errors
		errorMock := NewMockForgeClient("error-forge", TypeGitHub)
		errorMock.SetError("ListRepositories", errors.New("network error"))

		forgeConfig := &config.ForgeConfig{
			Name:          "error-forge",
			Type:          config.ForgeGitHub,
			Organizations: []string{"test-org"},
			Auth: &config.AuthConfig{
				Type:  config.AuthTypeToken,
				Token: "test-token",
			},
		}

		manager := NewForgeManager()
		manager.AddForge(forgeConfig, errorMock)

		discovery := NewDiscoveryService(manager, &config.FilteringConfig{})
		ctx := context.Background()

		// Discovery should handle errors gracefully
		result, err := discovery.DiscoverAll(ctx)
		if err != nil {
			t.Fatalf("Discovery should handle forge errors gracefully, got: %v", err)
		}

		// Should record the error but not fail completely
		if len(result.Errors) == 0 {
			t.Error("Expected error to be recorded in result")
		}

		if result.Errors["error-forge"] == nil {
			t.Error("Expected error for error-forge to be recorded")
		}

		t.Logf("✓ Error handling works correctly: %v", result.Errors["error-forge"])
	})

	t.Log("=== Real DocBuilder Forge Integration Tests Complete ===")
}
