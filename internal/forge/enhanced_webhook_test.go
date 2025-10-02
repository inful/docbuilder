package forge

import (
	"context"
	"testing"
	"time"
)

// TestEnhancedWebhookWorkflow demonstrates advanced webhook testing capabilities with enhanced mocks
func TestEnhancedWebhookWorkflow(t *testing.T) {
	t.Log("=== Enhanced Webhook Testing with Advanced Mock System ===")

	// Test GitHub webhook workflow
	t.Run("EnhancedGitHubWebhookWorkflow", func(t *testing.T) {
		client := NewEnhancedGitHubMock("webhook-github").
			WithWebhookSecret("github-webhook-secret")

		ctx := context.Background()

		// Create a test repository
		repo := CreateMockGitHubRepo("webhook-org", "webhook-repo", true, false, false, false)
		client.AddRepository(repo)

		// Test webhook registration
		webhookURL := "https://docbuilder.example.com/webhooks/github"
		err := client.RegisterWebhook(ctx, repo, webhookURL)
		if err != nil {
			t.Errorf("RegisterWebhook() error: %v", err)
		}

		// Test webhook validation with correct secret
		payload := []byte(`{"ref": "refs/heads/main", "repository": {"name": "webhook-repo", "full_name": "webhook-org/webhook-repo"}, "commits": [{"id": "abc123", "message": "Update docs"}]}`)
		isValid := client.ValidateWebhook(payload, "sha256=valid-signature", "github-webhook-secret")
		if !isValid {
			t.Error("GitHub webhook validation should succeed with correct secret")
		}

		// Test webhook validation with wrong secret
		isValid = client.ValidateWebhook(payload, "sha256=valid-signature", "wrong-secret")
		if isValid {
			t.Error("GitHub webhook validation should fail with wrong secret")
		}

		// Test push event parsing
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

		if len(event.Commits) != 2 { // Enhanced mock creates 2 commits for push events
			t.Errorf("Expected 2 commits in push event, got %d", len(event.Commits))
		}

		// Test pull request event parsing
		prEvent, err := client.ParseWebhookEvent(payload, "pull_request")
		if err != nil {
			t.Errorf("ParseWebhookEvent() for PR error: %v", err)
		}

		if prEvent.Branch != "feature-branch" {
			t.Errorf("PR event branch = %s, want feature-branch", prEvent.Branch)
		}

		if prEvent.Metadata["pull_request_number"] != "42" {
			t.Errorf("PR number = %s, want 42", prEvent.Metadata["pull_request_number"])
		}

		t.Log("✓ Enhanced GitHub webhook workflow complete")
	})

	// Test GitLab webhook workflow
	t.Run("EnhancedGitLabWebhookWorkflow", func(t *testing.T) {
		client := NewEnhancedGitLabMock("webhook-gitlab").
			WithWebhookSecret("gitlab-webhook-token")

		ctx := context.Background()

		// Create a test project
		repo := CreateMockGitLabRepo("webhook-group", "webhook-project", true, false, false, false)
		client.AddRepository(repo)

		// Test webhook registration
		webhookURL := "https://docbuilder.example.com/webhooks/gitlab"
		err := client.RegisterWebhook(ctx, repo, webhookURL)
		if err != nil {
			t.Errorf("RegisterWebhook() error: %v", err)
		}

		// Test GitLab-style webhook validation (token-based)
		payload := []byte(`{"object_kind": "push", "ref": "refs/heads/main", "project": {"name": "webhook-project", "path_with_namespace": "webhook-group/webhook-project"}}`)
		isValid := client.ValidateWebhook(payload, "gitlab-webhook-token", "gitlab-webhook-token")
		if !isValid {
			t.Error("GitLab webhook validation should succeed with correct token")
		}

		// Test webhook validation with wrong token
		isValid = client.ValidateWebhook(payload, "wrong-token", "gitlab-webhook-token")
		if isValid {
			t.Error("GitLab webhook validation should fail with wrong token")
		}

		// Test push event parsing
		event, err := client.ParseWebhookEvent(payload, "push")
		if err != nil {
			t.Errorf("ParseWebhookEvent() error: %v", err)
		}

		if event.Type != WebhookEventPush {
			t.Errorf("Event type = %s, want %s", event.Type, WebhookEventPush)
		}

		// Test release event parsing
		releaseEvent, err := client.ParseWebhookEvent(payload, "release")
		if err != nil {
			t.Errorf("ParseWebhookEvent() for release error: %v", err)
		}

		if releaseEvent.Metadata["tag"] != "v1.0.0" {
			t.Errorf("Release tag = %s, want v1.0.0", releaseEvent.Metadata["tag"])
		}

		t.Log("✓ Enhanced GitLab webhook workflow complete")
	})

	// Test multi-platform webhook handling
	t.Run("EnhancedMultiPlatformWebhookHandling", func(t *testing.T) {
		// Create enhanced mocks for different platforms
		github := NewEnhancedGitHubMock("multi-github").WithWebhookSecret("github-secret")
		gitlab := NewEnhancedGitLabMock("multi-gitlab").WithWebhookSecret("gitlab-secret")
		forgejo := NewEnhancedForgejoMock("multi-forgejo").WithWebhookSecret("forgejo-secret")

		ctx := context.Background()

		// Add repositories to each platform
		githubRepo := CreateMockGitHubRepo("multi-org", "github-docs", true, false, false, false)
		gitlabRepo := CreateMockGitLabRepo("multi-group", "gitlab-docs", true, false, false, false)
		forgejoRepo := CreateMockForgejoRepo("multi-org", "forgejo-docs", true, false, false, false)

		github.AddRepository(githubRepo)
		gitlab.AddRepository(gitlabRepo)
		forgejo.AddRepository(forgejoRepo)

		// Test webhook registration across platforms
		for _, test := range []struct {
			name   string
			client *EnhancedMockForgeClient
			repo   *Repository
			url    string
		}{
			{"GitHub", github, githubRepo, "https://docbuilder.example.com/webhooks/github"},
			{"GitLab", gitlab, gitlabRepo, "https://docbuilder.example.com/webhooks/gitlab"},
			{"Forgejo", forgejo, forgejoRepo, "https://docbuilder.example.com/webhooks/forgejo"},
		} {
			t.Run(test.name, func(t *testing.T) {
				err := test.client.RegisterWebhook(ctx, test.repo, test.url)
				if err != nil {
					t.Errorf("%s RegisterWebhook() error: %v", test.name, err)
				}

				// Test webhook event parsing
				payload := []byte(`{"test": "payload"}`)
				event, err := test.client.ParseWebhookEvent(payload, "push")
				if err != nil {
					t.Errorf("%s ParseWebhookEvent() error: %v", test.name, err)
				}

				if event.Type != WebhookEventPush {
					t.Errorf("%s event type = %s, want %s", test.name, event.Type, WebhookEventPush)
				}
			})
		}

		t.Log("✓ Enhanced multi-platform webhook handling complete")
	})

	// Test webhook failure scenarios
	t.Run("EnhancedWebhookFailureScenarios", func(t *testing.T) {
		client := NewEnhancedMockForgeClient("failure-webhook", ForgeTypeGitHub).
			WithWebhookSecret("test-secret")

		ctx := context.Background()
		repo := CreateMockGitHubRepo("failure-org", "failure-repo", true, false, false, false)
		client.AddRepository(repo)

		// Test auth failure during webhook registration
		client.WithAuthFailure()
		err := client.RegisterWebhook(ctx, repo, "https://example.com/webhook")
		if err == nil {
			t.Error("Expected auth failure during webhook registration")
		}

		// Test network timeout during webhook operations
		client.ClearFailures()
		client.WithNetworkTimeout(time.Millisecond * 50)
		_, err = client.ParseWebhookEvent([]byte(`{"test": "payload"}`), "push")
		if err == nil {
			t.Error("Expected network timeout during webhook event parsing")
		}

		// Test rate limiting
		client.ClearFailures()
		client.WithRateLimit(100, time.Hour)
		err = client.RegisterWebhook(ctx, repo, "https://example.com/webhook")
		if err == nil {
			t.Error("Expected rate limit error during webhook registration")
		}

		// Test recovery after clearing failures
		client.ClearFailures()
		err = client.RegisterWebhook(ctx, repo, "https://example.com/webhook")
		if err != nil {
			t.Errorf("Should succeed after clearing failures: %v", err)
		}

		event, err := client.ParseWebhookEvent([]byte(`{"test": "payload"}`), "push")
		if err != nil {
			t.Errorf("Should succeed after clearing failures: %v", err)
		}

		if event.Type != WebhookEventPush {
			t.Errorf("Event type after recovery = %s, want %s", event.Type, WebhookEventPush)
		}

		t.Log("✓ Enhanced webhook failure scenarios complete")
	})

	// Test webhook validation edge cases
	t.Run("EnhancedWebhookValidationEdgeCases", func(t *testing.T) {
		client := NewEnhancedMockForgeClient("validation-test", ForgeTypeGitHub).
			WithWebhookSecret("validation-secret")

		payload := []byte(`{"test": "validation"}`)

		// Test various signature formats
		testCases := []struct {
			name      string
			signature string
			secret    string
			expected  bool
		}{
			{"Valid SHA256", "sha256=valid-signature", "validation-secret", true},
			{"Valid SHA1", "sha1=valid-signature", "validation-secret", true},
			{"Invalid format", "invalid-signature", "validation-secret", false},
			{"Empty signature", "", "validation-secret", false},
			{"Wrong secret", "sha256=valid-signature", "wrong-secret", false},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				isValid := client.ValidateWebhook(payload, tc.signature, tc.secret)
				if isValid != tc.expected {
					t.Errorf("ValidateWebhook() = %v, want %v", isValid, tc.expected)
				}
			})
		}

		// Test GitLab-style validation
		gitlabClient := NewEnhancedMockForgeClient("gitlab-validation", ForgeTypeGitLab).
			WithWebhookSecret("gitlab-token")

		gitlabTestCases := []struct {
			name     string
			token    string
			secret   string
			expected bool
		}{
			{"Valid token", "gitlab-token", "gitlab-token", true},
			{"Invalid token", "wrong-token", "gitlab-token", false},
		}

		for _, tc := range gitlabTestCases {
			t.Run("GitLab_"+tc.name, func(t *testing.T) {
				isValid := gitlabClient.ValidateWebhook(payload, tc.token, tc.secret)
				if isValid != tc.expected {
					t.Errorf("GitLab ValidateWebhook() = %v, want %v", isValid, tc.expected)
				}
			})
		}

		t.Log("✓ Enhanced webhook validation edge cases complete")
	})

	t.Log("\n=== Enhanced Webhook Testing Summary ===")
	t.Log("✓ Multi-platform webhook workflows (GitHub, GitLab, Forgejo)")
	t.Log("✓ Platform-specific validation mechanisms")
	t.Log("✓ Advanced failure scenarios (auth, network, rate limit)")
	t.Log("✓ Error recovery and resilience testing")
	t.Log("✓ Comprehensive validation edge cases")
	t.Log("✓ Realistic webhook event parsing with metadata")
	t.Log("→ Enhanced webhook testing infrastructure complete")
}
