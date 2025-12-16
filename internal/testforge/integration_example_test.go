package testforge_test

import (
	"context"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/testforge"
)

// TestTestForgeBasicUsage demonstrates basic test forge usage
func TestTestForgeBasicUsage(t *testing.T) {
	// Create a test forge with realistic data
	forge := testforge.NewTestForge("integration-test", config.ForgeGitHub)
	forge.ClearRepositories()
	forge.ClearOrganizations()

	// Add a realistic test repository
	testRepo := testforge.TestRepository{
		Name:        "user-guide",
		FullName:    "acme-corp/user-guide",
		CloneURL:    "https://github.com/acme-corp/user-guide.git",
		Description: "User guide documentation for Acme Corp products",
		Topics:      []string{"documentation", "user-guide", "acme"},
		Language:    "Markdown",
		Private:     false,
		Archived:    false,
		Fork:        false,
	}

	forge.AddRepository(testRepo)
	forge.AddOrganization("acme-corp")

	ctx := context.Background()

	// Test organization discovery
	orgs, err := forge.GetUserOrganizations(ctx)
	if err != nil {
		t.Fatalf("Failed to get organizations: %v", err)
	}

	if len(orgs) != 1 || orgs[0].Name != "acme-corp" {
		t.Errorf("Expected 1 organization 'acme-corp', got %d: %v", len(orgs), orgs)
	}

	// Test repository discovery
	repos, err := forge.GetRepositoriesForOrganization(ctx, "acme-corp")
	if err != nil {
		t.Fatalf("Failed to get repositories: %v", err)
	}

	if len(repos) != 1 || repos[0].Name != "user-guide" {
		t.Errorf("Expected 1 repository 'user-guide', got %d: %v", len(repos), repos)
	}

	// Verify repository attributes
	repo := repos[0]
	if repo.FullName != "acme-corp/user-guide" {
		t.Errorf("Expected full name 'acme-corp/user-guide', got '%s'", repo.FullName)
	}

	if !contains(repo.Topics, "documentation") {
		t.Errorf("Expected repository to have 'documentation' topic, got %v", repo.Topics)
	}
}

// TestTestForgeConfiguration demonstrates creating test configurations
func TestTestForgeConfiguration(t *testing.T) {
	// Create test forge configuration
	forgeConfig := testforge.CreateTestForgeConfig(
		"test-github",
		config.ForgeGitHub,
		[]string{"acme-corp"},
	)

	// Verify configuration
	if forgeConfig.Name != "test-github" {
		t.Errorf("Expected name 'test-github', got '%s'", forgeConfig.Name)
	}

	if forgeConfig.Type != config.ForgeGitHub {
		t.Errorf("Expected GitHub forge type, got %v", forgeConfig.Type)
	}

	if len(forgeConfig.Organizations) != 1 || forgeConfig.Organizations[0] != "acme-corp" {
		t.Errorf("Expected organizations ['acme-corp'], got %v", forgeConfig.Organizations)
	}

	if forgeConfig.Auth == nil || forgeConfig.Auth.Type != config.AuthTypeToken {
		t.Error("Expected token authentication")
	}

	// Use in a full configuration
	fullConfig := config.Config{
		Forges: []*config.ForgeConfig{&forgeConfig},
		Output: config.OutputConfig{
			Directory: "/tmp/test-output",
		},
		Hugo: config.HugoConfig{
			Title: "Test Documentation Site",
			Params: map[string]interface{}{
				"navbar": map[string]interface{}{
					"displayTitle": true,
				},
			},
		},
	}

	// Verify full config is valid
	if len(fullConfig.Forges) != 1 {
		t.Error("Expected 1 forge in configuration")
	}

	if fullConfig.Output.Directory != "/tmp/test-output" {
		t.Errorf("Expected output directory '/tmp/test-output', got '%s'", fullConfig.Output.Directory)
	}
}

// TestTestForgeFailureModes demonstrates testing error conditions
func TestTestForgeFailureModes(t *testing.T) {
	forge := testforge.NewTestForge("failure-test", config.ForgeGitHub)
	ctx := context.Background()

	testCases := []struct {
		name     string
		failMode testforge.FailMode
		wantErr  bool
	}{
		{"auth_failure", testforge.FailModeAuth, true},
		{"network_failure", testforge.FailModeNetwork, true},
		{"rate_limit", testforge.FailModeRateLimit, true},
		{"not_found", testforge.FailModeNotFound, true},
		{"success", testforge.FailModeNone, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			forge.SetFailMode(tc.failMode)

			_, err := forge.GetUserOrganizations(ctx)

			if tc.wantErr && err == nil {
				t.Error("Expected error but got success")
			}

			if !tc.wantErr && err != nil {
				t.Errorf("Expected success but got error: %v", err)
			}
		})
	}
}

// TestTestForgeScenarios demonstrates using predefined test scenarios
func TestTestForgeScenarios(t *testing.T) {
	scenarios := testforge.CreateTestScenarios()

	for _, scenario := range scenarios {
		t.Run(scenario.Name, func(t *testing.T) {
			if len(scenario.Forges) == 0 {
				t.Skip("Scenario has no forges")
			}

			// Verify scenario structure
			forge := scenario.Forges[0]
			if forge == nil {
				t.Error("Expected non-nil forge")
			}
		})
	}
}

// Helper function
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
