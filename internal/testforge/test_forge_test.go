package testforge

import (
	"context"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

func TestTestForgeBasics(t *testing.T) {
	// Create a basic test forge
	forge := NewTestForge("test-forge", config.ForgeGitHub)

	// Clear default data first
	forge.ClearRepositories()
	forge.ClearOrganizations()

	// Add some test repositories
	testRepo := TestRepository{
		Name:        "test-docs",
		FullName:    "test-org/test-docs",
		CloneURL:    "https://github.com/test-org/test-docs.git",
		Description: "Test documentation repository",
		Topics:      []string{"docs", "documentation"},
		Language:    "Markdown",
		Private:     false,
		Archived:    false,
		Fork:        false,
	}

	forge.AddRepository(testRepo)
	forge.AddOrganization("test-org")

	// Test basic functionality
	orgs, err := forge.GetUserOrganizations(context.Background())
	if err != nil {
		t.Fatalf("Failed to list organizations: %v", err)
	}

	if len(orgs) != 1 || orgs[0].Name != "test-org" {
		t.Errorf("Expected 1 organization 'test-org', got %d orgs: %v", len(orgs), orgs)
	}

	// Test repository listing
	repos, err := forge.GetRepositoriesForOrganization(context.Background(), "test-org")
	if err != nil {
		t.Fatalf("Failed to list repositories: %v", err)
	}

	if len(repos) != 1 || repos[0].Name != "test-docs" {
		t.Errorf("Expected 1 repository 'test-docs', got %d repos: %v", len(repos), repos)
	}
}

func TestTestForgeFailureModes(t *testing.T) {
	forge := NewTestForge("failing-forge", config.ForgeGitHub)

	// Clear default data
	forge.ClearRepositories()
	forge.ClearOrganizations()

	// Enable failure modes
	forge.SetFailMode(FailModeAuth)

	// Test that failures are triggered
	_, err := forge.GetUserOrganizations(context.Background())
	if err == nil {
		t.Error("Expected failure due to auth error, but got success")
	}

	// Disable failure modes
	forge.SetFailMode(FailModeNone)

	// Should work now
	orgs, err := forge.GetUserOrganizations(context.Background())
	if err != nil {
		t.Fatalf("Expected success after disabling failures, got: %v", err)
	}

	if len(orgs) != 0 {
		t.Errorf("Expected 0 organizations in empty forge, got %d", len(orgs))
	}
}

func TestTestForgeFactory(t *testing.T) {
	factory := NewFactory()

	// Test GitHub forge creation
	githubForge := factory.CreateGitHubTestForge("test-github")
	if githubForge == nil {
		t.Fatal("Failed to create GitHub test forge")
	}

	// Test GitLab forge creation
	gitlabForge := factory.CreateGitLabTestForge("test-gitlab")
	if gitlabForge == nil {
		t.Fatal("Failed to create GitLab test forge")
	}

	// Test Forgejo forge creation
	forgejoForge := factory.CreateForgejoTestForge("test-forgejo")
	if forgejoForge == nil {
		t.Fatal("Failed to create Forgejo test forge")
	}
}

func TestCreateTestForgeConfig(t *testing.T) {
	// Test config creation
	testConfig := CreateTestForgeConfig("test", config.ForgeGitHub, []string{"test-org"})

	if testConfig.Name != "test" {
		t.Errorf("Expected name 'test', got '%s'", testConfig.Name)
	}

	if testConfig.Type != config.ForgeGitHub {
		t.Errorf("Expected GitHub forge type, got %v", testConfig.Type)
	}

	if len(testConfig.Organizations) != 1 || testConfig.Organizations[0] != "test-org" {
		t.Errorf("Expected organizations ['test-org'], got %v", testConfig.Organizations)
	}

	if testConfig.Auth == nil || testConfig.Auth.Type != config.AuthTypeToken {
		t.Error("Expected token authentication")
	}
}

func TestTestDiscoveryScenarios(t *testing.T) {
	scenarios := CreateTestScenarios()

	if len(scenarios) == 0 {
		t.Error("Expected test scenarios to be created")
	}

	// Test first scenario
	scenario := scenarios[0]
	if scenario.Name == "" {
		t.Error("Expected scenario to have a name")
	}

	if len(scenario.Forges) == 0 {
		t.Error("Expected scenario to have test forges")
	}

	if scenario.Expected.TotalRepositories <= 0 {
		t.Error("Expected scenario to have expected results")
	}
}
