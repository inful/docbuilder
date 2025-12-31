package forge

import (
	"context"
	"testing"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

func TestIntegrationSummary(t *testing.T) {
	t.Log("=== DocBuilder Forge Integration Testing Summary ===")

	t.Run("ForgeManager", testForgeManager)
	t.Run("EnhancedMockForgeClient", testEnhancedMockForgeClient)
	t.Run("EnhancedMultiPlatformDiscovery", testEnhancedMultiPlatformDiscovery)
	t.Run("DiscoveryService", testDiscoveryService)
	t.Run("ForgeFactory", testForgeFactory)
	t.Run("RepositoryConversion", testRepositoryConversion)
}

func testForgeManager(t *testing.T) {
	manager := NewForgeManager()
	if manager == nil {
		t.Fatal("NewForgeManager() should return a non-nil manager")
	}

	// Test adding a forge config
	githubConfig := &config.ForgeConfig{
		Name:          "test-github",
		Type:          "github",
		Organizations: []string{"test-org"},
		Auth: &config.AuthConfig{
			Type:  "token",
			Token: "test-token",
		},
	}

	mockClient := NewEnhancedMockForgeClient("test-github", TypeGitHub)
	manager.AddForge(githubConfig, mockClient)

	// Verify we can retrieve the forge
	client := manager.GetForge("test-github")
	if client == nil {
		t.Error("Should be able to retrieve added forge")
	}

	if client.GetName() != "test-github" {
		t.Errorf("Client name = %s, want test-github", client.GetName())
	}

	if client.GetType() != TypeGitHub {
		t.Errorf("Client type = %s, want %s", client.GetType(), TypeGitHub)
	}

	t.Log("✓ ForgeManager creation and configuration works")
}

func testEnhancedMockForgeClient(t *testing.T) {
	client := NewEnhancedMockForgeClient("enhanced-test", TypeGitHub)

	// Add test data
	org := CreateMockOrganization("1", "test-org", "Test Organization", "Organization")
	client.AddOrganization(org)

	repo := CreateMockGitHubRepo("test-org", "test-docs-repo", true, false, false, false)
	client.AddRepository(repo)

	// Test organization listing
	ctx := context.Background()
	orgs, err := client.ListOrganizations(ctx)
	if err != nil {
		t.Errorf("ListOrganizations() error: %v", err)
	}

	if len(orgs) != 1 {
		t.Errorf("Expected 1 organization, got %d", len(orgs))
	}

	if orgs[0].Name != "test-org" {
		t.Errorf("Organization name = %s, want test-org", orgs[0].Name)
	}

	// Test repository listing
	repos, err := client.ListRepositories(ctx, []string{"test-org"})
	if err != nil {
		t.Errorf("ListRepositories() error: %v", err)
	}

	if len(repos) != 1 {
		t.Errorf("Expected 1 repository, got %d", len(repos))
	}

	if repos[0].Name != "test-docs-repo" {
		t.Errorf("Repository name = %s, want test-docs-repo", repos[0].Name)
	}

	// Test documentation checking with enhanced logic
	err = client.CheckDocumentation(ctx, repos[0])
	if err != nil {
		t.Errorf("CheckDocumentation() error: %v", err)
	}

	if !repos[0].HasDocs {
		t.Error("Repository should be marked as having docs (name contains 'docs')")
	}

	// Test enhanced failure simulation capabilities
	client.WithAuthFailure()
	_, err = client.ListOrganizations(ctx)
	if err == nil {
		t.Error("Expected authentication failure with enhanced mock")
	}

	// Test recovery
	client.ClearFailures()
	orgs, err = client.ListOrganizations(ctx)
	if err != nil {
		t.Errorf("After clearing failures, ListOrganizations() error: %v", err)
	}

	if len(orgs) != 1 {
		t.Errorf("Expected 1 organization after recovery, got %d", len(orgs))
	}

	// Test configuration generation
	config := client.GenerateForgeConfig()
	if config.Name != "enhanced-test" {
		t.Errorf("Generated config name = %s, want enhanced-test", config.Name)
	}
	if config.Type != "github" {
		t.Errorf("Generated config type = %s, want github", config.Type)
	}

	t.Log("✓ Enhanced MockForgeClient functionality works")
}

func testEnhancedMultiPlatformDiscovery(t *testing.T) {
	manager := NewForgeManager()

	// Create enhanced mocks for different platforms
	github := NewEnhancedGitHubMock("enhanced-github")
	gitlab := NewEnhancedGitLabMock("enhanced-gitlab")
	forgejo := NewEnhancedForgejoMock("enhanced-forgejo")

	// Add additional test repositories to each platform
	github.AddRepository(CreateMockGitHubRepo("github-org", "user-docs", true, false, false, false))
	gitlab.AddRepository(CreateMockGitLabRepo("gitlab-group", "api-documentation", true, false, false, false))

	// Test forge factory with enhanced mocks
	githubConfig := github.GenerateForgeConfig()
	gitlabConfig := gitlab.GenerateForgeConfig()
	forgejoConfig := forgejo.GenerateForgeConfig()

	manager.AddForge(githubConfig, github)
	manager.AddForge(gitlabConfig, gitlab)
	manager.AddForge(forgejoConfig, forgejo)

	ctx := context.Background()

	// Test GitHub discovery
	githubRepos, err := github.ListRepositories(ctx, []string{"github-org"})
	if err != nil {
		t.Errorf("GitHub ListRepositories() error: %v", err)
	}
	if len(githubRepos) != 2 { // Pre-configured + additional
		t.Errorf("Expected 2 GitHub repositories, got %d", len(githubRepos))
	}

	// Test GitLab discovery
	gitlabRepos, err := gitlab.ListRepositories(ctx, []string{"gitlab-group"})
	if err != nil {
		t.Errorf("GitLab ListRepositories() error: %v", err)
	}
	if len(gitlabRepos) != 2 { // Pre-configured + additional
		t.Errorf("Expected 2 GitLab repositories, got %d", len(gitlabRepos))
	}

	// Test Forgejo discovery
	forgejoRepos, err := forgejo.ListRepositories(ctx, []string{"forgejo-org"})
	if err != nil {
		t.Errorf("Forgejo ListRepositories() error: %v", err)
	}
	if len(forgejoRepos) != 1 { // Pre-configured only
		t.Errorf("Expected 1 Forgejo repository, got %d", len(forgejoRepos))
	}

	// Test failure simulation across platforms
	github.WithRateLimit(100, time.Hour)
	_, err = github.ListOrganizations(ctx)
	if err == nil {
		t.Error("Expected rate limit error from GitHub mock")
	}

	gitlab.WithNetworkTimeout(time.Millisecond * 50)
	_, err = gitlab.ListOrganizations(ctx)
	if err == nil {
		t.Error("Expected network timeout from GitLab mock")
	}

	// Test recovery
	github.ClearFailures()
	gitlab.ClearFailures()

	_, err = github.ListOrganizations(ctx)
	if err != nil {
		t.Errorf("GitHub should recover after clearing failures: %v", err)
	}

	_, err = gitlab.ListOrganizations(ctx)
	if err != nil {
		t.Errorf("GitLab should recover after clearing failures: %v", err)
	}

	t.Log("✓ Enhanced multi-platform forge discovery works")
}

func testDiscoveryService(t *testing.T) {
	manager := NewForgeManager()
	filtering := &config.FilteringConfig{
		RequiredPaths: []string{"docs"},
	}

	discovery := NewDiscoveryService(manager, filtering)
	if discovery == nil {
		t.Fatal("NewDiscoveryService() should return a non-nil service")
	}

	t.Log("✓ DiscoveryService creation works")
}

func testForgeFactory(t *testing.T) {
	// Test GitHub client creation
	githubConfig := &config.ForgeConfig{
		Name:   "factory-github",
		Type:   "github",
		APIURL: "https://api.github.com",
		Auth:   &config.AuthConfig{Type: "token", Token: "test"},
	}

	client, err := NewForgeClient(githubConfig)
	if err != nil {
		t.Errorf("CreateForgeClient() error: %v", err)
	}

	if client == nil {
		t.Fatal("CreateForgeClient() should return a non-nil client")
	}

	if client.GetType() != TypeGitHub {
		t.Errorf("Client type = %s, want %s", client.GetType(), TypeGitHub)
	}

	// Test GitLab client creation
	gitlabConfig := &config.ForgeConfig{
		Name:   "factory-gitlab",
		Type:   "gitlab",
		APIURL: "https://gitlab.com/api/v4",
		Auth:   &config.AuthConfig{Type: "token", Token: "test"},
	}

	client, err = NewForgeClient(gitlabConfig)
	if err != nil {
		t.Errorf("CreateForgeClient() error: %v", err)
	}

	if client.GetType() != TypeGitLab {
		t.Errorf("Client type = %s, want %s", client.GetType(), TypeGitLab)
	}

	// Test Forgejo client creation
	forgejoConfig := &config.ForgeConfig{
		Name:   "factory-forgejo",
		Type:   "forgejo",
		APIURL: "https://git.example.com/api/v1",
		Auth:   &config.AuthConfig{Type: "token", Token: "test"},
	}

	client, err = NewForgeClient(forgejoConfig)
	if err != nil {
		t.Errorf("CreateForgeClient() error: %v", err)
	}

	if client.GetType() != TypeForgejo {
		t.Errorf("Client type = %s, want %s", client.GetType(), TypeForgejo)
	}

	t.Log("✓ Forge factory functionality works")
}

func testRepositoryConversion(t *testing.T) {
	repo := CreateMockGitHubRepo("test-org", "conversion-test", true, false, false, false)
	auth := &config.AuthConfig{Type: "ssh", KeyPath: "~/.ssh/id_rsa"}

	configRepo := repo.ToConfigRepository(auth)

	// Should use SSH URL when auth type is SSH
	if configRepo.URL != repo.SSHURL {
		t.Errorf("Config URL = %s, want SSH URL %s", configRepo.URL, repo.SSHURL)
	}

	if configRepo.Name != repo.Name {
		t.Errorf("Config name = %s, want %s", configRepo.Name, repo.Name)
	}

	if configRepo.Branch != repo.DefaultBranch {
		t.Errorf("Config branch = %s, want %s", configRepo.Branch, repo.DefaultBranch)
	}

	if configRepo.Auth != auth {
		t.Error("Config should have auth reference")
	}

	// Check metadata tags
	if configRepo.Tags["forge_id"] != repo.ID {
		t.Errorf("Config forge_id = %s, want %s", configRepo.Tags["forge_id"], repo.ID)
	}

	if configRepo.Tags["full_name"] != repo.FullName {
		t.Errorf("Config full_name = %s, want %s", configRepo.Tags["full_name"], repo.FullName)
	}

	t.Log("✓ Repository conversion functionality works")
}
