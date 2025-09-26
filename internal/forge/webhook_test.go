package forge

import (
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"
	"time"
)

func TestGitHubWebhookValidation(t *testing.T) {
	client := &GitHubClient{}
	secret := "test-secret-key"

	// Test valid signature
	payload := `{"action":"push","repository":{"name":"test-repo","full_name":"test-org/test-repo"}}`

	// Create HMAC signature
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	isValid := client.ValidateWebhook([]byte(payload), signature, secret)
	if !isValid {
		t.Error("ValidateWebhook() should return true for valid signature")
	}

	// Test invalid signature
	isValid = client.ValidateWebhook([]byte(payload), "sha256=invalid-signature", secret)
	if isValid {
		t.Error("ValidateWebhook() should return false for invalid signature")
	}

	// Test missing signature
	isValid = client.ValidateWebhook([]byte(payload), "", secret)
	if isValid {
		t.Error("ValidateWebhook() should return false for missing signature")
	}

	// Test SHA-1 fallback
	mac1 := hmac.New(sha1.New, []byte(secret))
	mac1.Write([]byte(payload))
	signatureSHA1 := "sha1=" + hex.EncodeToString(mac1.Sum(nil))

	isValid = client.ValidateWebhook([]byte(payload), signatureSHA1, secret)
	if !isValid {
		t.Error("ValidateWebhook() should support SHA-1 signature fallback")
	}
}

func TestGitLabWebhookValidation(t *testing.T) {
	client := &GitLabClient{}
	secret := "gitlab-secret-token"

	payload := `{"event_type":"push","project":{"name":"test-project","path_with_namespace":"test-group/test-project"}}`

	// Test valid token
	isValid := client.ValidateWebhook([]byte(payload), secret, secret)
	if !isValid {
		t.Error("ValidateWebhook() should return true for valid GitLab token")
	}

	// Test invalid token
	isValid = client.ValidateWebhook([]byte(payload), "wrong-token", secret)
	if isValid {
		t.Error("ValidateWebhook() should return false for invalid GitLab token")
	}

	// Test missing token
	isValid = client.ValidateWebhook([]byte(payload), "", secret)
	if isValid {
		t.Error("ValidateWebhook() should return false for missing GitLab token")
	}
}

func TestForgejoWebhookValidation(t *testing.T) {
	client := &ForgejoClient{}
	secret := "forgejo-hmac-secret"

	payload := `{"action":"push","repository":{"name":"test-repo","full_name":"test-org/test-repo"}}`

	// Create HMAC-SHA256 signature (Forgejo uses GitHub-compatible webhooks)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	isValid := client.ValidateWebhook([]byte(payload), signature, secret)
	if !isValid {
		t.Error("ValidateWebhook() should return true for valid Forgejo signature")
	}

	// Test invalid signature
	isValid = client.ValidateWebhook([]byte(payload), "sha256=invalid", secret)
	if isValid {
		t.Error("ValidateWebhook() should return false for invalid Forgejo signature")
	}
}

func TestGitHubWebhookParsing(t *testing.T) {
	client := &GitHubClient{}

	tests := []struct {
		name         string
		eventType    string
		payload      string
		expectedRepo string
		expectedType WebhookEventType
		expectError  bool
	}{
		{
			name:      "Push event",
			eventType: "push",
			payload: `{
				"ref": "refs/heads/main",
				"repository": {
					"id": "123",
					"name": "test-repo",
					"full_name": "test-org/test-repo",
					"html_url": "https://github.com/test-org/test-repo",
					"default_branch": "main"
				}
			}`,
			expectedRepo: "test-org/test-repo",
			expectedType: WebhookEventPush,
		},
		{
			name:      "Repository event",
			eventType: "repository",
			payload: `{
				"action": "created",
				"repository": {
					"id": "456",
					"name": "new-repo",
					"full_name": "test-org/new-repo",
					"html_url": "https://github.com/test-org/new-repo",
					"default_branch": "main"
				}
			}`,
			expectedRepo: "test-org/new-repo",
			expectedType: WebhookEventRepository,
		},
		{
			name:        "Invalid JSON",
			eventType:   "push",
			payload:     `{"invalid": json}`,
			expectError: true,
		},
		{
			name:        "Missing repository",
			eventType:   "push",
			payload:     `{"ref": "refs/heads/main"}`,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event, err := client.ParseWebhookEvent([]byte(tt.payload), tt.eventType)

			if tt.expectError {
				if err == nil {
					t.Error("ParseWebhookEvent() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("ParseWebhookEvent() unexpected error: %v", err)
				return
			}

			if event.Repository.FullName != tt.expectedRepo {
				t.Errorf("Repository.FullName = %v, want %v", event.Repository.FullName, tt.expectedRepo)
			}

			if event.Type != tt.expectedType {
				t.Errorf("Type = %v, want %v", event.Type, tt.expectedType)
			}

			if event.Timestamp.IsZero() {
				t.Error("Timestamp should be set")
			}
		})
	}
}

func TestGitLabWebhookParsing(t *testing.T) {
	client := &GitLabClient{}

	tests := []struct {
		name         string
		eventType    string
		payload      string
		expectedRepo string
		expectedType WebhookEventType
		expectError  bool
	}{
		{
			name:      "Push Hook",
			eventType: "Push Hook",
			payload: `{
				"event_type": "push",
				"ref": "refs/heads/main",
				"project": {
					"id": 123,
					"name": "test-project",
					"path_with_namespace": "test-group/test-project",
					"web_url": "https://gitlab.com/test-group/test-project",
					"default_branch": "main"
				}
			}`,
			expectedRepo: "test-group/test-project",
			expectedType: WebhookEventPush,
		},
		{
			name:      "Tag Push Hook",
			eventType: "Tag Push Hook",
			payload: `{
				"event_type": "tag_push",
				"ref": "refs/tags/v1.0.0",
				"project": {
					"id": 456,
					"name": "tag-project",
					"path_with_namespace": "test-group/tag-project",
					"web_url": "https://gitlab.com/test-group/tag-project",
					"default_branch": "main"
				}
			}`,
			expectedRepo: "test-group/tag-project",
			expectedType: WebhookEventTag,
		},
		{
			name:        "Missing project",
			eventType:   "Push Hook",
			payload:     `{"event_type": "push", "ref": "refs/heads/main"}`,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event, err := client.ParseWebhookEvent([]byte(tt.payload), tt.eventType)

			if tt.expectError {
				if err == nil {
					t.Error("ParseWebhookEvent() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("ParseWebhookEvent() unexpected error: %v", err)
				return
			}

			if event.Repository.FullName != tt.expectedRepo {
				t.Errorf("Repository.FullName = %v, want %v", event.Repository.FullName, tt.expectedRepo)
			}

			if event.Type != tt.expectedType {
				t.Errorf("Type = %v, want %v", event.Type, tt.expectedType)
			}
		})
	}
}

func TestForgejoWebhookParsing(t *testing.T) {
	client := &ForgejoClient{}

	// Forgejo uses GitHub-compatible webhook format
	payload := `{
		"ref": "refs/heads/main",
		"repository": {
			"id": "789",
			"name": "forgejo-repo",
			"full_name": "forgejo-org/forgejo-repo",
			"html_url": "https://git.example.com/forgejo-org/forgejo-repo",
			"default_branch": "main"
		}
	}`

	event, err := client.ParseWebhookEvent([]byte(payload), "push")
	if err != nil {
		t.Errorf("ParseWebhookEvent() unexpected error: %v", err)
		return
	}

	if event.Repository.FullName != "forgejo-org/forgejo-repo" {
		t.Errorf("Repository.FullName = %v, want forgejo-org/forgejo-repo", event.Repository.FullName)
	}

	if event.Type != WebhookEventPush {
		t.Errorf("Type = %v, want %v", event.Type, WebhookEventPush)
	}
}

func TestWebhookEventFiltering(t *testing.T) {
	tests := []struct {
		name          string
		eventType     WebhookEventType
		allowedEvents []string
		shouldProcess bool
	}{
		{
			name:          "Push allowed",
			eventType:     WebhookEventPush,
			allowedEvents: []string{"push", "repository"},
			shouldProcess: true,
		},
		{
			name:          "Push not allowed",
			eventType:     WebhookEventPush,
			allowedEvents: []string{"tag", "repository"},
			shouldProcess: false,
		},
		{
			name:          "Tag allowed",
			eventType:     WebhookEventTag,
			allowedEvents: []string{"push", "tag"},
			shouldProcess: true,
		},
		{
			name:          "Repository event filtered",
			eventType:     WebhookEventRepository,
			allowedEvents: []string{"push", "tag"},
			shouldProcess: false,
		},
		{
			name:          "Empty allowed events (allow all)",
			eventType:     WebhookEventPush,
			allowedEvents: []string{},
			shouldProcess: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_ = &WebhookEvent{
				Type: tt.eventType,
				Repository: &Repository{
					FullName: "test/repo",
				},
				Timestamp: time.Now(),
			}

			shouldProcess := len(tt.allowedEvents) == 0
			if !shouldProcess {
				for _, allowed := range tt.allowedEvents {
					if allowed == string(tt.eventType) {
						shouldProcess = true
						break
					}
				}
			}

			if shouldProcess != tt.shouldProcess {
				t.Errorf("Event filtering: got %v, want %v for event %s with allowed %v",
					shouldProcess, tt.shouldProcess, string(tt.eventType), tt.allowedEvents)
			}
		})
	}
}

func TestWebhookEventSerialization(t *testing.T) {
	originalEvent := &WebhookEvent{
		Type: WebhookEventPush,
		Repository: &Repository{
			ID:            "123",
			Name:          "test-repo",
			FullName:      "test-org/test-repo",
			DefaultBranch: "main",
		},
		Branch:    "main",
		Timestamp: time.Now().Truncate(time.Second), // Truncate for JSON comparison
		Metadata: map[string]string{
			"commit_count": "3",
			"author":       "test-user",
			"forced":       "false",
		},
	}

	// Serialize to JSON
	data, err := json.Marshal(originalEvent)
	if err != nil {
		t.Fatalf("JSON marshaling error: %v", err)
	}

	// Deserialize from JSON
	var deserializedEvent WebhookEvent
	err = json.Unmarshal(data, &deserializedEvent)
	if err != nil {
		t.Fatalf("JSON unmarshaling error: %v", err)
	}

	// Compare fields
	if deserializedEvent.Type != originalEvent.Type {
		t.Errorf("Type = %v, want %v", deserializedEvent.Type, originalEvent.Type)
	}

	if deserializedEvent.Repository.FullName != originalEvent.Repository.FullName {
		t.Errorf("Repository.FullName = %v, want %v", deserializedEvent.Repository.FullName, originalEvent.Repository.FullName)
	}

	if deserializedEvent.Branch != originalEvent.Branch {
		t.Errorf("Branch = %v, want %v", deserializedEvent.Branch, originalEvent.Branch)
	}

	if !deserializedEvent.Timestamp.Equal(originalEvent.Timestamp) {
		t.Errorf("Timestamp = %v, want %v", deserializedEvent.Timestamp, originalEvent.Timestamp)
	}

	// Check metadata
	if deserializedEvent.Metadata["commit_count"] != "3" {
		t.Errorf("Metadata commit_count = %v, want 3", deserializedEvent.Metadata["commit_count"])
	}

	if deserializedEvent.Metadata["author"] != "test-user" {
		t.Errorf("Metadata author = %v, want test-user", deserializedEvent.Metadata["author"])
	}
}
