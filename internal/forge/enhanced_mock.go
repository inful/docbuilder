package forge

import (
	"context"
	"fmt"
	"strings"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// EnhancedMockForgeClient provides advanced mock capabilities for testing forge integrations
// This is the production-ready version of the enhanced mock system developed in Phase 1 & 2
type EnhancedMockForgeClient struct {
	name          string
	forgeType     ForgeType
	repositories  []*Repository
	organizations []*Organization

	// Failure simulation state
	authFailure    bool
	rateLimit      *RateLimitConfig
	networkTimeout time.Duration
	responseDelay  time.Duration

	// Configuration
	webhookSecret string

	// Error tracking
	lastError      error
	operationCount int
}

// RateLimitConfig defines rate limiting parameters
type RateLimitConfig struct {
	RequestsPerHour int
	ResetTime       time.Time
}

// NewEnhancedMockForgeClient creates a new enhanced mock forge client
func NewEnhancedMockForgeClient(name string, forgeType ForgeType) *EnhancedMockForgeClient {
	return &EnhancedMockForgeClient{
		name:          name,
		forgeType:     forgeType,
		repositories:  make([]*Repository, 0),
		organizations: make([]*Organization, 0),
	}
}

// NewEnhancedGitHubMock creates a pre-configured GitHub mock with realistic defaults
func NewEnhancedGitHubMock(name string) *EnhancedMockForgeClient {
	mock := NewEnhancedMockForgeClient(name, ForgeTypeGitHub)

	// Add default GitHub organization
	mock.AddOrganization(&Organization{
		ID:          "github-org-1",
		Name:        "github-org",
		DisplayName: "GitHub Organization",
		Type:        "organization",
	})

	// Add default repository with documentation
	mock.AddRepository(CreateMockGitHubRepo("github-org", "docs-repo", true, false, false, false))

	return mock
}

// NewEnhancedGitLabMock creates a pre-configured GitLab mock with realistic defaults
func NewEnhancedGitLabMock(name string) *EnhancedMockForgeClient {
	mock := NewEnhancedMockForgeClient(name, ForgeTypeGitLab)

	// Add default GitLab group
	mock.AddOrganization(&Organization{
		ID:          "gitlab-group-1",
		Name:        "gitlab-group",
		DisplayName: "GitLab Group",
		Type:        "group",
	})

	// Add default repository with documentation
	mock.AddRepository(CreateMockGitLabRepo("gitlab-group", "docs-repo", true, false, false, false))

	return mock
}

// NewEnhancedForgejoMock creates a pre-configured Forgejo mock with realistic defaults
func NewEnhancedForgejoMock(name string) *EnhancedMockForgeClient {
	mock := NewEnhancedMockForgeClient(name, ForgeTypeForgejo)

	// Add default Forgejo organization
	mock.AddOrganization(&Organization{
		ID:          "forgejo-org-1",
		Name:        "forgejo-org",
		DisplayName: "Forgejo Organization",
		Type:        "organization",
	})

	// Add default repository with documentation
	mock.AddRepository(CreateMockForgejoRepo("forgejo-org", "docs-repo", true, false, false, false))

	return mock
}

// Repository Management Methods

// AddRepository adds a repository to the mock
func (m *EnhancedMockForgeClient) AddRepository(repo *Repository) {
	m.repositories = append(m.repositories, repo)
}

// AddOrganization adds an organization to the mock
func (m *EnhancedMockForgeClient) AddOrganization(org *Organization) {
	m.organizations = append(m.organizations, org)
}

// ClearRepositories removes all repositories
func (m *EnhancedMockForgeClient) ClearRepositories() {
	m.repositories = make([]*Repository, 0)
}

// ClearOrganizations removes all organizations
func (m *EnhancedMockForgeClient) ClearOrganizations() {
	m.organizations = make([]*Organization, 0)
}

// Failure Simulation Methods

// WithWebhookSecret sets the webhook secret for testing
func (m *EnhancedMockForgeClient) WithWebhookSecret(secret string) *EnhancedMockForgeClient {
	m.webhookSecret = secret
	return m
}

// WithAuthFailure enables authentication failure simulation
func (m *EnhancedMockForgeClient) WithAuthFailure() *EnhancedMockForgeClient {
	m.authFailure = true
	return m
}

// WithRateLimit enables rate limiting simulation
func (m *EnhancedMockForgeClient) WithRateLimit(requestsPerHour int, resetDuration time.Duration) *EnhancedMockForgeClient {
	m.rateLimit = &RateLimitConfig{
		RequestsPerHour: requestsPerHour,
		ResetTime:       time.Now().Add(resetDuration),
	}
	return m
}

// WithNetworkTimeout enables network timeout simulation
func (m *EnhancedMockForgeClient) WithNetworkTimeout(timeout time.Duration) *EnhancedMockForgeClient {
	m.networkTimeout = timeout
	return m
}

// WithDelay enables response delay simulation
func (m *EnhancedMockForgeClient) WithDelay(delay time.Duration) *EnhancedMockForgeClient {
	m.responseDelay = delay
	return m
}

// ClearFailures removes all failure conditions
func (m *EnhancedMockForgeClient) ClearFailures() {
	m.authFailure = false
	m.rateLimit = nil
	m.networkTimeout = 0
	m.responseDelay = 0
	m.lastError = nil
	m.operationCount = 0
}

// ForgeClient Interface Implementation

// GetType returns the forge type
func (m *EnhancedMockForgeClient) GetType() ForgeType {
	return m.forgeType
}

// GetName returns the forge name
func (m *EnhancedMockForgeClient) GetName() string {
	return m.name
}

// ListOrganizations returns mock organizations with failure simulation
func (m *EnhancedMockForgeClient) ListOrganizations(ctx context.Context) ([]*Organization, error) {
	if err := m.simulateFailures(); err != nil {
		return nil, err
	}

	return m.organizations, nil
}

// ListRepositories returns mock repositories with failure simulation
func (m *EnhancedMockForgeClient) ListRepositories(ctx context.Context, organizations []string) ([]*Repository, error) {
	if err := m.simulateFailures(); err != nil {
		return nil, err
	}

	// Filter repositories by organizations if specified
	if len(organizations) == 0 {
		return m.repositories, nil
	}

	var filtered []*Repository
	for _, repo := range m.repositories {
		for _, org := range organizations {
			if containsOrganization(repo.FullName, org) {
				filtered = append(filtered, repo)
				break
			}
		}
	}

	return filtered, nil
}

// GetRepository returns a specific repository
func (m *EnhancedMockForgeClient) GetRepository(ctx context.Context, owner, repo string) (*Repository, error) {
	if err := m.simulateFailures(); err != nil {
		return nil, err
	}

	fullName := fmt.Sprintf("%s/%s", owner, repo)
	for _, r := range m.repositories {
		if r.FullName == fullName {
			return r, nil
		}
	}

	return nil, fmt.Errorf("repository %s not found", fullName)
}

// CheckDocumentation checks if repository has documentation
func (m *EnhancedMockForgeClient) CheckDocumentation(ctx context.Context, repo *Repository) error {
	if err := m.simulateFailures(); err != nil {
		return err
	}

	// The mock doesn't need to do anything special here - the repository's HasDocs field
	// is already set when it's created. This method is just for simulating API calls.
	// Real implementations would check for docs folder existence and set HasDocs accordingly.

	return nil
}

// ValidateWebhook validates webhook signatures
func (m *EnhancedMockForgeClient) ValidateWebhook(payload []byte, signature string, secret string) bool {
	// Empty signature should fail
	if signature == "" {
		return false
	}

	// Simulate platform-specific validation
	switch m.forgeType {
	case ForgeTypeGitHub:
		// GitHub uses HMAC-SHA256 with 'sha256=' prefix, but also supports SHA1
		// For testing, we accept both "sha256=valid-signature" and "sha1=valid-signature" with correct secret
		validSignature := signature == "sha256=valid-signature" || signature == "sha1=valid-signature"

		// Use the configured webhook secret if available, otherwise use a default test secret
		expectedSecret := "test-secret"
		if m.webhookSecret != "" {
			expectedSecret = m.webhookSecret
		}
		correctSecret := secret == expectedSecret

		return validSignature && correctSecret
	case ForgeTypeGitLab:
		// GitLab uses token-based validation - signature IS the token
		expectedSecret := secret
		if m.webhookSecret != "" {
			expectedSecret = m.webhookSecret
		}
		return signature == expectedSecret
	case ForgeTypeForgejo:
		// Forgejo uses HMAC-SHA1 similar to Gitea
		// For testing, we accept valid signatures with correct secret
		validSignature := signature != "sha256=invalid"
		expectedSecret := secret
		if m.webhookSecret != "" {
			expectedSecret = m.webhookSecret
		}
		correctSecret := secret == expectedSecret
		return validSignature && correctSecret
	default:
		return false
	}
}

// ParseWebhookEvent parses webhook events
func (m *EnhancedMockForgeClient) ParseWebhookEvent(payload []byte, eventType string) (*WebhookEvent, error) {
	if err := m.simulateFailures(); err != nil {
		return nil, err
	}

	event := &WebhookEvent{
		Type:       WebhookEventType(eventType),
		Repository: &Repository{Name: "mock-repo", FullName: "test-org/mock-repo"},
		Branch:     "main",
		Timestamp:  time.Now(),
		Metadata: map[string]string{
			"forge_type": string(m.forgeType),
			"mock":       "true",
		},
	}

	// Add event-specific data
	switch eventType {
	case "push":
		event.Type = WebhookEventPush
		event.Branch = "main"
		// Add mock commits for push events
		event.Commits = []WebhookCommit{
			{
				ID:        "abc123",
				Message:   "Update documentation",
				Author:    "Test Author",
				Timestamp: time.Now(),
				Modified:  []string{"docs/README.md"},
			},
			{
				ID:        "def456",
				Message:   "Fix typos",
				Author:    "Test Author",
				Timestamp: time.Now(),
				Modified:  []string{"docs/guide.md"},
			},
		}
	case "pull_request":
		event.Type = WebhookEventPush // Use available type
		event.Branch = "feature-branch"
		event.Action = "opened"
		event.Metadata["pull_request_number"] = "42"
	case "release":
		event.Type = WebhookEventTag // Use available type
		event.Branch = "main"
		event.Action = "published"
		event.Metadata["tag"] = "v1.0.0"
	}

	return event, nil
}

// RegisterWebhook registers webhooks (mock implementation)
func (m *EnhancedMockForgeClient) RegisterWebhook(ctx context.Context, repo *Repository, webhookURL string) error {
	if err := m.simulateFailures(); err != nil {
		return err
	}

	// Validate webhook URL format
	if webhookURL == "" {
		return fmt.Errorf("webhook URL cannot be empty")
	}

	// Simple URL validation - must start with http:// or https://
	if !strings.HasPrefix(webhookURL, "http://") && !strings.HasPrefix(webhookURL, "https://") {
		return fmt.Errorf("invalid webhook URL format: %s", webhookURL)
	}

	return nil // Mock success
}

// GetEditURL generates platform-specific edit URLs
func (m *EnhancedMockForgeClient) GetEditURL(repo *Repository, filePath string, branch string) string {
	switch m.forgeType {
	case ForgeTypeGitHub:
		return fmt.Sprintf("https://github.com/%s/edit/%s/%s", repo.FullName, branch, filePath)
	case ForgeTypeGitLab:
		return fmt.Sprintf("https://gitlab.com/%s/-/edit/%s/%s", repo.FullName, branch, filePath)
	case ForgeTypeForgejo:
		return fmt.Sprintf("https://git.example.com/%s/_edit/%s/%s", repo.FullName, branch, filePath)
	default:
		return fmt.Sprintf("https://forge.example.com/%s/edit/%s/%s", repo.FullName, branch, filePath)
	}
}

// Configuration Generation

// GenerateForgeConfig generates a complete forge configuration
func (m *EnhancedMockForgeClient) GenerateForgeConfig() *ForgeConfig {
	var apiURL, baseURL string

	switch m.forgeType {
	case ForgeTypeGitHub:
		apiURL = "https://api.github.com"
		baseURL = "https://github.com"
	case ForgeTypeGitLab:
		apiURL = "https://gitlab.com/api/v4"
		baseURL = "https://gitlab.com"
	case ForgeTypeForgejo:
		apiURL = "https://git.example.com/api/v1"
		baseURL = "https://git.example.com"
	default:
		apiURL = fmt.Sprintf("https://api.%s.example.com", m.name)
		baseURL = fmt.Sprintf("https://%s.example.com", m.name)
	}

	return &ForgeConfig{
		Name:    m.name,
		Type:    ForgeType(m.forgeType),
		APIURL:  apiURL,
		BaseURL: baseURL,
		Organizations: func() []string {
			var orgs []string
			for _, org := range m.organizations {
				orgs = append(orgs, org.Name)
			}
			return orgs
		}(),
		Auth: &config.AuthConfig{
			Type:  config.AuthTypeToken,
			Token: fmt.Sprintf("%s-test-token", m.name),
		},
		Webhook: &WebhookConfig{
			Secret: func() string {
				if m.webhookSecret != "" {
					return m.webhookSecret
				}
				return fmt.Sprintf("%s-webhook-secret", m.name)
			}(),
			Path:   fmt.Sprintf("/webhooks/%s", m.name),
			Events: []string{"push", "repository"},
		},
	}
}

// Private helper methods

// simulateFailures simulates various failure modes
func (m *EnhancedMockForgeClient) simulateFailures() error {
	m.operationCount++

	// Simulate response delay
	if m.responseDelay > 0 {
		time.Sleep(m.responseDelay)
	}

	// Simulate network timeout
	if m.networkTimeout > 0 && m.networkTimeout < time.Millisecond*100 {
		switch m.forgeType {
		case ForgeTypeGitHub:
			return fmt.Errorf("network timeout: connection to https://api.github.com timed out")
		case ForgeTypeGitLab:
			return fmt.Errorf("network timeout: connection to https://gitlab.com/api/v4 timed out")
		case ForgeTypeForgejo:
			return fmt.Errorf("network timeout: connection to https://forgejo.org/api/v1 timed out")
		default:
			return fmt.Errorf("network timeout after %v", m.networkTimeout)
		}
	}

	// Simulate authentication failure
	if m.authFailure {
		return fmt.Errorf("authentication failed: invalid credentials")
	}

	// Simulate rate limiting
	if m.rateLimit != nil {
		if time.Now().Before(m.rateLimit.ResetTime) {
			return fmt.Errorf("rate limit exceeded: %d requests per hour", m.rateLimit.RequestsPerHour)
		}
	}

	return nil
}

// containsOrganization checks if a repository full name contains the organization
func containsOrganization(fullName, org string) bool {
	return len(fullName) > len(org) && fullName[:len(org)] == org
}
