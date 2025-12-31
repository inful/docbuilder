package forge

import (
	"context"
	"testing"
	"time"
)

const authFailureMessage = "authentication failed: invalid credentials"

// TestPhase3BIntegrationDemo demonstrates how the enhanced mock system integrates
// with existing DocBuilder test patterns and provides backward compatibility.
func TestPhase3BIntegrationDemo(t *testing.T) {
	t.Log("=== Phase 3B: Enhanced Mock Integration Demonstration ===")

	t.Run("DirectForgeClientReplacement", testDirectForgeClientReplacement)
	t.Run("RealisticTestData", testRealisticTestData)
	t.Run("AdvancedFailureSimulation", testAdvancedFailureSimulation)
	t.Run("BulkDataGeneration", testBulkDataGeneration)
	t.Run("FluentBuilderPatterns", testFluentBuilderPatterns)
}

func testDirectForgeClientReplacement(t *testing.T) {
	// Demonstrate direct replacement of existing ForgeClient usage
	var client Client = NewEnhancedMockForgeClient("integration-demo", TypeGitHub)
	ctx := context.Background()

	// Add test data using enhanced system
	enhanced := client.(*EnhancedMockForgeClient)
	enhanced.AddRepository(CreateMockGitHubRepo("company", "docs", true, false, false, false))
	enhanced.AddRepository(CreateMockGitHubRepo("company", "api", true, false, false, false))
	enhanced.AddOrganization(CreateMockGitHubOrg("company"))

	// Use exactly like any ForgeClient
	orgs, err := client.ListOrganizations(ctx)
	if err != nil {
		t.Fatalf("Failed to list organizations: %v", err)
	}
	if len(orgs) != 1 {
		t.Errorf("Expected 1 organization, got %d", len(orgs))
	}

	repos, err := client.ListRepositories(ctx, []string{"company"})
	if err != nil {
		t.Fatalf("Failed to list repositories: %v", err)
	}
	if len(repos) != 2 {
		t.Errorf("Expected 2 repositories, got %d", len(repos))
	}

	// Test specific repository retrieval
	repo, err := client.GetRepository(ctx, "company", "docs")
	if err != nil {
		t.Fatalf("Failed to get repository: %v", err)
	}
	if repo.Name != "docs" {
		t.Errorf("Expected repository name 'docs', got '%s'", repo.Name)
	}

	t.Log("✓ Enhanced mock works as drop-in ForgeClient replacement")
}

func testRealisticTestData(t *testing.T) {
	// Demonstrate realistic test data generation
	github := CreateRealisticGitHubMock("demo-github")
	gitlab := CreateRealisticGitLabMock("demo-gitlab")
	forgejo := CreateRealisticForgejoMock("demo-forgejo")

	ctx := context.Background()

	// GitHub realistic data
	githubRepos, err := github.ListRepositories(ctx, []string{"company"})
	if err != nil {
		t.Fatalf("GitHub realistic mock failed: %v", err)
	}
	if len(githubRepos) != 3 {
		t.Errorf("Expected 3 GitHub repos, got %d", len(githubRepos))
	}

	// Verify realistic repository properties
	for _, repo := range githubRepos {
		if repo.CloneURL == "" {
			t.Error("Repository missing clone URL")
		}
		if repo.DefaultBranch == "" {
			t.Error("Repository missing default branch")
		}
		if len(repo.Topics) == 0 {
			t.Error("Repository missing topics")
		}
	}

	// GitLab realistic data
	gitlabRepos, err := gitlab.ListRepositories(ctx, []string{"team"})
	if err != nil {
		t.Fatalf("GitLab realistic mock failed: %v", err)
	}
	if len(gitlabRepos) != 3 {
		t.Errorf("Expected 3 GitLab repos, got %d", len(gitlabRepos))
	}

	// Forgejo realistic data
	forgejoRepos, err := forgejo.ListRepositories(ctx, []string{"self-hosted"})
	if err != nil {
		t.Fatalf("Forgejo realistic mock failed: %v", err)
	}
	if len(forgejoRepos) != 3 {
		t.Errorf("Expected 3 Forgejo repos, got %d", len(forgejoRepos))
	}

	t.Log("✓ Realistic test data generation works across all platforms")
}

func testAdvancedFailureSimulation(t *testing.T) {
	// Demonstrate advanced failure simulation for robust testing
	client := NewEnhancedMockForgeClient("failure-demo", TypeGitHub)
	ctx := context.Background()

	// Add test data
	client.AddRepository(CreateMockGitHubRepo("test-org", "test-repo", true, false, false, false))
	client.AddOrganization(CreateMockGitHubOrg("test-org"))

	// Test normal operation first
	repos, err := client.ListRepositories(ctx, []string{"test-org"})
	if err != nil {
		t.Fatalf("Normal operation failed: %v", err)
	}
	if len(repos) != 1 {
		t.Errorf("Expected 1 repository, got %d", len(repos))
	}

	// Simulate authentication failure
	client.WithAuthFailure()
	_, err = client.ListRepositories(ctx, []string{"test-org"})
	if err == nil {
		t.Error("Expected authentication failure, got nil")
	}
	if err.Error() != authFailureMessage {
		t.Errorf("Expected specific auth error, got: %s", err.Error())
	}

	// Simulate network timeout
	client.ClearFailures()
	client.WithNetworkTimeout(time.Millisecond * 50)
	_, err = client.ListOrganizations(ctx)
	if err == nil {
		t.Error("Expected network timeout, got nil")
	}

	// Simulate rate limiting
	client.ClearFailures()
	client.WithRateLimit(100, time.Hour)
	_, err = client.ListRepositories(ctx, []string{"test-org"})
	if err == nil {
		t.Error("Expected rate limit failure, got nil")
	}

	// Verify recovery after clearing failures
	client.ClearFailures()
	repos, err = client.ListRepositories(ctx, []string{"test-org"})
	if err != nil {
		t.Errorf("Expected success after clearing failures, got: %v", err)
	}
	if len(repos) != 1 {
		t.Errorf("Expected 1 repository after recovery, got %d", len(repos))
	}

	t.Log("✓ Advanced failure simulation provides comprehensive error testing")
}

func testBulkDataGeneration(t *testing.T) {
	// Demonstrate bulk data generation for performance testing
	repos := CreateMockRepositorySet(TypeGitHub, "large-org", 25)
	if len(repos) != 25 {
		t.Errorf("Expected 25 repositories, got %d", len(repos))
	}

	// Verify pattern distribution
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

	// Should be roughly 50% with docs (12-13 repos)
	if docsCount < 10 || docsCount > 15 {
		t.Errorf("Expected 10-15 repos with docs, got %d", docsCount)
	}

	// Should be roughly 20% private (4-6 repos)
	if privateCount < 3 || privateCount > 7 {
		t.Errorf("Expected 3-7 private repos, got %d", privateCount)
	}

	// Test with enhanced mock
	client := NewEnhancedMockForgeClient("bulk-test", TypeGitHub)
	client.AddOrganization(CreateMockGitHubOrg("large-org"))
	for _, repo := range repos {
		client.AddRepository(repo)
	}

	ctx := context.Background()
	retrievedRepos, err := client.ListRepositories(ctx, []string{"large-org"})
	if err != nil {
		t.Fatalf("Failed to list bulk repositories: %v", err)
	}
	if len(retrievedRepos) != 25 {
		t.Errorf("Expected 25 retrieved repositories, got %d", len(retrievedRepos))
	}

	t.Log("✓ Bulk data generation supports performance testing scenarios")
}

func testFluentBuilderPatterns(t *testing.T) {
	// Demonstrate fluent builder for complex test setup
	mock := NewEnhancedMockBuilder("builder-demo", TypeGitLab).
		WithRepositories(
			CreateMockGitLabRepo("team-alpha", "service-docs", true, false, false, false),
			CreateMockGitLabRepo("team-alpha", "api-reference", true, false, false, false),
			CreateMockGitLabRepo("team-beta", "user-guide", true, false, false, false),
		).
		WithOrganizations(
			CreateMockGitLabGroup("team-alpha"),
			CreateMockGitLabGroup("team-beta"),
		).
		WithDelay(time.Millisecond * 10).
		Build()

	ctx := context.Background()

	// Verify organizations
	orgs, err := mock.ListOrganizations(ctx)
	if err != nil {
		t.Fatalf("Failed to list organizations: %v", err)
	}
	if len(orgs) != 2 {
		t.Errorf("Expected 2 organizations, got %d", len(orgs))
	}

	// Verify repositories
	allRepos, err := mock.ListRepositories(ctx, []string{})
	if err != nil {
		t.Fatalf("Failed to list all repositories: %v", err)
	}
	if len(allRepos) != 3 {
		t.Errorf("Expected 3 repositories, got %d", len(allRepos))
	}

	// Verify filtered repositories
	alphaRepos, err := mock.ListRepositories(ctx, []string{"team-alpha"})
	if err != nil {
		t.Fatalf("Failed to list team-alpha repositories: %v", err)
	}
	if len(alphaRepos) != 2 {
		t.Errorf("Expected 2 team-alpha repositories, got %d", len(alphaRepos))
	}

	t.Log("✓ Fluent builder pattern enables complex test scenario setup")
}

// TestPhase3BMigrationPatterns demonstrates how to migrate existing tests.
func TestPhase3BMigrationPatterns(t *testing.T) {
	t.Log("=== Phase 3B: Test Migration Patterns ===")

	t.Run("BeforeAfterComparison", testBeforeAfterComparison)
	t.Run("EasyMigrationSteps", testEasyMigrationSteps)
}

func testBeforeAfterComparison(t *testing.T) {
	// BEFORE: Traditional mock setup (simulated)
	t.Log("Traditional pattern (BEFORE):")
	t.Log("  - Create basic mock struct")
	t.Log("  - Manually implement each interface method")
	t.Log("  - Limited failure simulation")
	t.Log("  - No realistic data generation")

	// AFTER: Enhanced mock system
	t.Log("Enhanced pattern (AFTER):")
	client := NewEnhancedMockForgeClient("migration-demo", TypeGitHub)
	client.AddRepository(CreateMockGitHubRepo("company", "docs", true, false, false, false))
	client.AddOrganization(CreateMockGitHubOrg("company"))

	ctx := context.Background()
	repos, err := client.ListRepositories(ctx, []string{"company"})
	if err != nil {
		t.Fatalf("Enhanced mock failed: %v", err)
	}
	if len(repos) != 1 {
		t.Errorf("Expected 1 repository, got %d", len(repos))
	}

	t.Log("  ✓ Comprehensive ForgeClient interface implementation")
	t.Log("  ✓ Advanced failure simulation capabilities")
	t.Log("  ✓ Realistic data generation with factory functions")
	t.Log("  ✓ Multi-platform support out of the box")
	t.Log("  ✓ Fluent builder pattern for complex scenarios")
}

func testEasyMigrationSteps(t *testing.T) {
	t.Log("Migration steps for existing tests:")
	t.Log("1. Replace basic mock creation:")
	t.Log("   OLD: mock := &BasicMockForgeClient{}")
	t.Log("   NEW: mock := NewEnhancedMockForgeClient(\"test\", ForgeTypeGitHub)")

	t.Log("2. Replace manual data setup:")
	t.Log("   OLD: mock.repos = []*Repository{...}")
	t.Log("   NEW: mock.AddRepository(CreateMockGitHubRepo(...))")

	t.Log("3. Add failure testing:")
	t.Log("   NEW: mock.WithAuthFailure() // or other failure modes")

	t.Log("4. Use realistic data:")
	t.Log("   NEW: mock := CreateRealisticGitHubMock(\"realistic\")")

	// Demonstrate the migration in action
	enhanced := NewEnhancedMockForgeClient("migrated-test", TypeGitHub)
	enhanced.AddRepository(CreateMockGitHubRepo("migrated-org", "migrated-repo", true, false, false, false))

	ctx := context.Background()
	repos, err := enhanced.ListRepositories(ctx, []string{"migrated-org"})
	if err != nil {
		t.Fatalf("Migrated test failed: %v", err)
	}
	if len(repos) != 1 {
		t.Errorf("Expected 1 repository in migrated test, got %d", len(repos))
	}

	t.Log("✓ Migration pattern successfully demonstrated")
}
