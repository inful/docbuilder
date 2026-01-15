package forge

import (
	"testing"
	"time"
)

func TestEnhancedMockForgeClient_BasicFunctionality(t *testing.T) {
	client := NewEnhancedMockForgeClient("test-enhanced", TypeGitHub)

	// Test basic properties
	if client.GetName() != "test-enhanced" {
		t.Errorf("GetName() = %s, want test-enhanced", client.GetName())
	}

	if client.GetType() != TypeGitHub {
		t.Errorf("GetType() = %s, want %s", client.GetType(), TypeGitHub)
	}

	// Test adding organizations
	org := CreateMockOrganization("1", "test-org", "Test Organization", "Organization")
	client.AddOrganization(org)

	ctx := t.Context()
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
}

func TestEnhancedMockForgeClient_FailureSimulation(t *testing.T) {
	client := NewEnhancedMockForgeClient("test-failures", TypeGitHub)

	// Test auth failure
	client.WithAuthFailure()
	ctx := t.Context()

	_, err := client.ListOrganizations(ctx)
	if err == nil {
		t.Error("Expected auth failure error, got nil")
	}

	if err.Error() != "authentication failed: invalid credentials" {
		t.Errorf("Expected auth failure message, got: %s", err.Error())
	}

	// Clear failures and test success
	client.ClearFailures()
	client.AddOrganization(CreateMockOrganization("1", "test-org", "Test Organization", "Organization"))

	orgs, err := client.ListOrganizations(ctx)
	if err != nil {
		t.Errorf("After clearing failures, ListOrganizations() error: %v", err)
	}

	if len(orgs) != 1 {
		t.Errorf("Expected 1 organization after clearing failures, got %d", len(orgs))
	}
}

func TestEnhancedMockForgeClient_RateLimitSimulation(t *testing.T) {
	client := NewEnhancedMockForgeClient("test-rate-limit", TypeGitHub)

	// Test rate limit
	client.WithRateLimit(100, time.Hour)
	ctx := t.Context()

	_, err := client.ListOrganizations(ctx)
	if err == nil {
		t.Error("Expected rate limit error, got nil")
	}

	if err.Error() != "rate limit exceeded: 100 requests per hour" {
		t.Errorf("Expected rate limit message, got: %s", err.Error())
	}
}

func TestEnhancedMockForgeClient_NetworkTimeoutSimulation(t *testing.T) {
	client := NewEnhancedMockForgeClient("test-timeout", TypeGitHub)

	// Test network timeout (but don't actually wait for the timeout)
	client.WithNetworkTimeout(time.Millisecond * 50)
	ctx := t.Context()

	_, err := client.ListOrganizations(ctx)
	if err == nil {
		t.Error("Expected network timeout error, got nil")
	}

	expectedMsg := "network timeout: connection to https://api.github.com timed out"
	if err.Error() != expectedMsg {
		t.Errorf("Expected network timeout message, got: %s", err.Error())
	}
}

func TestEnhancedMockForgeClient_RepositoryManagement(t *testing.T) {
	client := NewEnhancedMockForgeClient("test-repos", TypeGitHub)

	// Add organization and repository
	org := CreateMockOrganization("1", "test-org", "Test Organization", "Organization")
	client.AddOrganization(org)

	repo := CreateMockGitHubRepo("test-org", "docs-repo", true, false, false, false)
	client.AddRepository(repo)

	ctx := t.Context()

	// Test repository listing
	repos, err := client.ListRepositories(ctx, []string{"test-org"})
	if err != nil {
		t.Errorf("ListRepositories() error: %v", err)
	}

	if len(repos) != 1 {
		t.Errorf("Expected 1 repository, got %d", len(repos))
	}

	if repos[0].Name != "docs-repo" {
		t.Errorf("Repository name = %s, want docs-repo", repos[0].Name)
	}

	// Test getting specific repository
	foundRepo, err := client.GetRepository(ctx, "test-org", "docs-repo")
	if err != nil {
		t.Errorf("GetRepository() error: %v", err)
	}

	if foundRepo.FullName != "test-org/docs-repo" {
		t.Errorf("Repository full name = %s, want test-org/docs-repo", foundRepo.FullName)
	}

	// Test documentation checking
	err = client.CheckDocumentation(ctx, foundRepo)
	if err != nil {
		t.Errorf("CheckDocumentation() error: %v", err)
	}

	if !foundRepo.HasDocs {
		t.Error("Repository should be marked as having docs (name contains 'docs')")
	}
}

func TestEnhancedMockForgeClient_WebhookFunctionality(t *testing.T) {
	client := NewEnhancedMockForgeClient("test-webhooks", TypeGitHub).
		WithWebhookSecret("test-secret")

	// Test webhook validation
	payload := []byte(`{"test": "payload"}`)
	signature := "sha256=valid-signature"
	secret := "test-secret"

	isValid := client.ValidateWebhook(payload, signature, secret)
	if !isValid {
		t.Error("ValidateWebhook() should return true for valid GitHub signature")
	}

	// Test invalid secret
	isValid = client.ValidateWebhook(payload, signature, "wrong-secret")
	if isValid {
		t.Error("ValidateWebhook() should return false for wrong secret")
	}

	// Test webhook event parsing
	ctx := t.Context()
	event, err := client.ParseWebhookEvent(payload, "push")
	if err != nil {
		t.Errorf("ParseWebhookEvent() error: %v", err)
	}

	if event.Type != WebhookEventPush {
		t.Errorf("Event type = %s, want %s", event.Type, WebhookEventPush)
	}

	if event.Branch != "main" {
		t.Errorf("Event branch = %s, want main", event.Branch)
	}

	if len(event.Commits) != 2 {
		t.Errorf("Expected 2 commits for push event, got %d", len(event.Commits))
	}

	// Test webhook registration
	repo := CreateMockGitHubRepo("test-org", "test-repo", true, false, false, false)
	err = client.RegisterWebhook(ctx, repo, "https://example.com/webhook")
	if err != nil {
		t.Errorf("RegisterWebhook() error: %v", err)
	}

	// Test invalid webhook URL
	err = client.RegisterWebhook(ctx, repo, "invalid-url")
	if err == nil {
		t.Error("RegisterWebhook() should fail for invalid URL")
	}
}

func TestEnhancedMockForgeClient_FactoryMethods(t *testing.T) {
	// Test GitHub factory
	github := NewEnhancedGitHubMock("github-test")
	if github.GetType() != TypeGitHub {
		t.Errorf("GitHub mock type = %s, want %s", github.GetType(), TypeGitHub)
	}

	// Should have default organization and repository
	ctx := t.Context()
	orgs, err := github.ListOrganizations(ctx)
	if err != nil {
		t.Errorf("GitHub mock ListOrganizations() error: %v", err)
	}
	if len(orgs) != 1 {
		t.Errorf("Expected 1 default organization in GitHub mock, got %d", len(orgs))
	}

	repos, err := github.ListRepositories(ctx, []string{"github-org"})
	if err != nil {
		t.Errorf("GitHub mock ListRepositories() error: %v", err)
	}
	if len(repos) != 1 {
		t.Errorf("Expected 1 default repository in GitHub mock, got %d", len(repos))
	}

	// Test GitLab factory
	gitlab := NewEnhancedGitLabMock("gitlab-test")
	if gitlab.GetType() != TypeGitLab {
		t.Errorf("GitLab mock type = %s, want %s", gitlab.GetType(), TypeGitLab)
	}

	// Test Forgejo factory
	forgejo := NewEnhancedForgejoMock("forgejo-test")
	if forgejo.GetType() != TypeForgejo {
		t.Errorf("Forgejo mock type = %s, want %s", forgejo.GetType(), TypeForgejo)
	}
}

func TestEnhancedMockForgeClient_ConfigGeneration(t *testing.T) {
	client := NewEnhancedMockForgeClient("test-config", TypeGitHub).
		WithWebhookSecret("test-secret")

	// Add some organizations
	client.AddOrganization(CreateMockOrganization("1", "org1", "Organization 1", "Organization"))
	client.AddOrganization(CreateMockOrganization("2", "org2", "Organization 2", "Organization"))

	// Generate configuration
	config := client.GenerateForgeConfig()

	if config.Name != "test-config" {
		t.Errorf("Config name = %s, want test-config", config.Name)
	}

	if string(config.Type) != string(TypeGitHub) {
		t.Errorf("Config type = %s, want %s", config.Type, TypeGitHub)
	}

	if config.APIURL != "https://api.github.com" {
		t.Errorf("Config API URL = %s, want https://api.github.com", config.APIURL)
	}

	if len(config.Organizations) != 2 {
		t.Errorf("Expected 2 organizations in config, got %d", len(config.Organizations))
	}

	if config.Auth.Type != "token" {
		t.Errorf("Config auth type = %s, want token", config.Auth.Type)
	}

	if config.Webhook.Secret != "test-secret" {
		t.Errorf("Config webhook secret = %s, want test-secret", config.Webhook.Secret)
	}
}

func TestEnhancedMockForgeClient_EditURLGeneration(t *testing.T) {
	tests := []struct {
		name        string
		forgeType   Type
		expectedURL string
	}{
		{
			name:        "GitHub",
			forgeType:   TypeGitHub,
			expectedURL: "https://github.com/test-org/test-repo/edit/main/docs/file.md",
		},
		{
			name:        "GitLab",
			forgeType:   TypeGitLab,
			expectedURL: "https://gitlab.com/test-org/test-repo/-/edit/main/docs/file.md",
		},
		{
			name:        "Forgejo",
			forgeType:   TypeForgejo,
			expectedURL: "https://git.example.com/test-org/test-repo/_edit/main/docs/file.md",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewEnhancedMockForgeClient("test-edit-url", tt.forgeType)
			repo := &Repository{
				FullName: "test-org/test-repo",
			}

			url := client.GetEditURL(repo, "docs/file.md", "main")
			if url != tt.expectedURL {
				t.Errorf("GetEditURL() = %s, want %s", url, tt.expectedURL)
			}
		})
	}
}

func TestEnhancedMockForgeClient_DelaySimulation(t *testing.T) {
	client := NewEnhancedMockForgeClient("test-delay", TypeGitHub).
		WithDelay(100 * time.Millisecond)

	client.AddOrganization(CreateMockOrganization("1", "test-org", "Test Organization", "Organization"))

	ctx := t.Context()
	start := time.Now()

	_, err := client.ListOrganizations(ctx)
	if err != nil {
		t.Errorf("ListOrganizations() with delay error: %v", err)
	}

	elapsed := time.Since(start)
	if elapsed < 100*time.Millisecond {
		t.Errorf("Expected delay of at least 100ms, got %v", elapsed)
	}
}
