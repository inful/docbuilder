package forge

import (
	"context"
	"fmt"
	"strings"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// MockForgeClient implements ForgeClient for testing.
type MockForgeClient struct {
	name          string
	forgeType     Type
	organizations []*Organization
	repositories  []*Repository
	errors        map[string]error // Map of method names to errors to simulate failures
}

// NewMockForgeClient creates a new mock forge client.
func NewMockForgeClient(name string, forgeType Type) *MockForgeClient {
	return &MockForgeClient{
		name:          name,
		forgeType:     forgeType,
		organizations: make([]*Organization, 0),
		repositories:  make([]*Repository, 0),
		errors:        make(map[string]error),
	}
}

// GetType returns the forge type.
func (m *MockForgeClient) GetType() Type {
	return m.forgeType
}

// GetName returns the configured name.
func (m *MockForgeClient) GetName() string {
	return m.name
}

// AddOrganization adds a mock organization.
func (m *MockForgeClient) AddOrganization(org *Organization) {
	m.organizations = append(m.organizations, org)
}

// AddRepository adds a mock repository.
func (m *MockForgeClient) AddRepository(repo *Repository) {
	m.repositories = append(m.repositories, repo)
}

// SetError sets an error to be returned by a specific method.
func (m *MockForgeClient) SetError(method string, err error) {
	m.errors[method] = err
}

// ListOrganizations returns mock organizations.
func (m *MockForgeClient) ListOrganizations(_ context.Context) ([]*Organization, error) {
	if err, exists := m.errors["ListOrganizations"]; exists {
		return nil, err
	}
	return m.organizations, nil
}

// ListRepositories returns mock repositories for specified organizations.
func (m *MockForgeClient) ListRepositories(_ context.Context, organizations []string) ([]*Repository, error) {
	if err, exists := m.errors["ListRepositories"]; exists {
		return nil, err
	}

	var filteredRepos []*Repository
	for _, repo := range m.repositories {
		// Check if repository belongs to any of the requested organizations
		for _, org := range organizations {
			if strings.HasPrefix(repo.FullName, org+"/") {
				filteredRepos = append(filteredRepos, repo)
				break
			}
		}
	}

	return filteredRepos, nil
}

// GetRepository gets detailed information about a specific repository.
func (m *MockForgeClient) GetRepository(_ context.Context, owner, repo string) (*Repository, error) {
	if err, exists := m.errors["GetRepository"]; exists {
		return nil, err
	}

	fullName := owner + "/" + repo
	for _, r := range m.repositories {
		if r.FullName == fullName {
			return r, nil
		}
	}

	return nil, fmt.Errorf("repository not found: %s", fullName)
}

// CheckDocumentation checks if repository has docs folder and .docignore.
func (m *MockForgeClient) CheckDocumentation(_ context.Context, repo *Repository) error {
	if err, exists := m.errors["CheckDocumentation"]; exists {
		return err
	}

	// Mock logic: check if repository name contains "docs" for HasDocs
	repo.HasDocs = strings.Contains(strings.ToLower(repo.Name), "docs") ||
		strings.Contains(strings.ToLower(repo.FullName), "docs")

	// Mock logic: check if repository name contains "ignore" for HasDocIgnore
	repo.HasDocIgnore = strings.Contains(strings.ToLower(repo.Name), "ignore")

	return nil
}

// ValidateWebhook validates webhook signature (always returns true for mock).
func (m *MockForgeClient) ValidateWebhook(_ []byte, signature string, _ string) bool {
	return signature == "valid_signature"
}

// ParseWebhookEvent parses webhook payload (returns mock event).
func (m *MockForgeClient) ParseWebhookEvent(_ []byte, eventType string) (*WebhookEvent, error) {
	if err, exists := m.errors["ParseWebhookEvent"]; exists {
		return nil, err
	}

	return &WebhookEvent{
		Type:      WebhookEventType(eventType),
		Timestamp: time.Now(),
		Repository: &Repository{
			ID:       "mock-repo-id",
			Name:     "mock-repo",
			FullName: "mock-org/mock-repo",
		},
		Branch: "main",
		Metadata: map[string]string{
			"mock": "true",
		},
	}, nil
}

// RegisterWebhook registers a webhook (mock implementation).
func (m *MockForgeClient) RegisterWebhook(_ context.Context, _ *Repository, _ string) error {
	if err, exists := m.errors["RegisterWebhook"]; exists {
		return err
	}
	return nil
}

// GetEditURL returns a mock edit URL.
func (m *MockForgeClient) GetEditURL(repo *Repository, filePath string, branch string) string {
	return fmt.Sprintf("https://mock-forge.com/%s/edit/%s/%s", repo.FullName, branch, filePath)
}

// Test helper functions

// CreateMockOrganization creates a mock organization.
func CreateMockOrganization(id, name, displayName, orgType string) *Organization {
	return &Organization{
		ID:          id,
		Name:        name,
		DisplayName: displayName,
		Description: fmt.Sprintf("Mock organization %s", displayName),
		Type:        orgType,
		Metadata: map[string]string{
			"mock": "true",
		},
	}
}

// CreateMockForgeConfig creates a mock forge configuration.
func CreateMockForgeConfig(name string, forgeType config.ForgeType, orgs, groups []string) *config.ForgeConfig {
	return &config.ForgeConfig{
		Name:          name,
		Type:          forgeType,
		APIURL:        fmt.Sprintf("https://api.%s.example.com", forgeType),
		BaseURL:       fmt.Sprintf("https://%s.example.com", forgeType),
		Organizations: orgs,
		Groups:        groups,
		Auth: &config.AuthConfig{
			Type:  "token",
			Token: "mock-token",
		},
		Webhook: &config.WebhookConfig{
			Secret: "mock-secret",
			Path:   fmt.Sprintf("/webhooks/%s", forgeType),
			Events: []string{"push", "repository"},
		},
	}
}
