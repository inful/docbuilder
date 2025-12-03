package forge

import (
	"context"
	"fmt"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// Type re-exports config.Type for convenience within forge package.
type Type = config.ForgeType

const (
	TypeGitHub  Type = config.ForgeGitHub
	TypeGitLab  Type = config.ForgeGitLab
	TypeForgejo Type = config.ForgeForgejo
)

// Repository represents a repository discovered from a forge
type Repository struct {
	ID            string            `json:"id"`             // Unique ID from the forge
	Name          string            `json:"name"`           // Repository name
	FullName      string            `json:"full_name"`      // Full name (org/repo)
	CloneURL      string            `json:"clone_url"`      // Git clone URL
	SSHURL        string            `json:"ssh_url"`        // SSH clone URL
	DefaultBranch string            `json:"default_branch"` // Default branch name
	Description   string            `json:"description"`    // Repository description
	Private       bool              `json:"private"`        // Is repository private
	Archived      bool              `json:"archived"`       // Is repository archived
	HasDocs       bool              `json:"has_docs"`       // Does repository have docs folder
	HasDocIgnore  bool              `json:"has_docignore"`  // Does repository have .docignore
	LastUpdated   time.Time         `json:"last_updated"`   // Last update timestamp
	Topics        []string          `json:"topics"`         // Repository topics/tags
	Language      string            `json:"language"`       // Primary programming language
	Metadata      map[string]string `json:"metadata"`       // Additional forge-specific metadata
}

// Organization represents an organization/group on a forge
type Organization struct {
	ID          string            `json:"id"`           // Unique ID from the forge
	Name        string            `json:"name"`         // Organization name
	DisplayName string            `json:"display_name"` // Display name
	Description string            `json:"description"`  // Organization description
	Type        string            `json:"type"`         // Type (org, group, user, etc.)
	Metadata    map[string]string `json:"metadata"`     // Additional forge-specific metadata
}

// Config represents configuration for a specific forge instance
// Note: This is now defined in config.Config to avoid import cycles
type Config = config.ForgeConfig

// WebhookConfig represents webhook configuration for a forge
// Note: This is now defined in config.WebhookConfig to avoid import cycles
type WebhookConfig = config.WebhookConfig

// Client interface defines the contract for forge platform clients
type Client interface {
	// GetType returns the type of this forge
	GetType() Type

	// GetName returns the configured name of this forge instance
	GetName() string

	// ListOrganizations returns all accessible organizations/groups
	ListOrganizations(ctx context.Context) ([]*Organization, error)

	// ListRepositories returns all repositories for the specified organizations/groups
	ListRepositories(ctx context.Context, organizations []string) ([]*Repository, error)

	// GetRepository gets detailed information about a specific repository
	GetRepository(ctx context.Context, owner, repo string) (*Repository, error)

	// CheckDocumentation checks if a repository has documentation and .docignore
	CheckDocumentation(ctx context.Context, repo *Repository) error

	// ValidateWebhook validates a webhook request signature
	ValidateWebhook(payload []byte, signature string, secret string) bool

	// ParseWebhookEvent parses a webhook payload into a standard event
	ParseWebhookEvent(payload []byte, event string) (*WebhookEvent, error)

	// RegisterWebhook registers a webhook for a repository (if supported)
	RegisterWebhook(ctx context.Context, repo *Repository, webhookURL string) error

	// GetEditURL returns the URL for editing a file in the web interface
	GetEditURL(repo *Repository, filePath string, branch string) string
}

// WebhookEvent represents a standardized webhook event
type WebhookEvent struct {
	Type       WebhookEventType  `json:"type"`
	Repository *Repository       `json:"repository"`
	Branch     string            `json:"branch"`
	Commits    []WebhookCommit   `json:"commits"`
	Action     string            `json:"action"`  // For repository events
	Changes    map[string]string `json:"changes"` // For rename events
	Timestamp  time.Time         `json:"timestamp"`
	Metadata   map[string]string `json:"metadata"` // Platform-specific data
}

// WebhookEventType represents the type of webhook event
type WebhookEventType string

const (
	WebhookEventPush       WebhookEventType = "push"
	WebhookEventRepository WebhookEventType = "repository" // created, deleted, renamed, archived
	WebhookEventBranch     WebhookEventType = "branch"     // created, deleted
	WebhookEventTag        WebhookEventType = "tag"        // created, deleted
)

// WebhookCommit represents commit information from a webhook
type WebhookCommit struct {
	ID        string    `json:"id"`
	Message   string    `json:"message"`
	Author    string    `json:"author"`
	Timestamp time.Time `json:"timestamp"`
	Added     []string  `json:"added"`
	Modified  []string  `json:"modified"`
	Removed   []string  `json:"removed"`
}

// Manager manages multiple forge clients
type Manager struct {
	clients map[string]Client
	configs map[string]*Config
}

// NewForgeManager creates a new forge manager
func NewForgeManager() *Manager {
	return &Manager{
		clients: make(map[string]Client),
		configs: make(map[string]*Config),
	}
}

// AddForge adds a forge client to the manager
func (m *Manager) AddForge(config *Config, client Client) {
	m.configs[config.Name] = config
	m.clients[config.Name] = client
}

// GetForge returns a forge client by name
func (m *Manager) GetForge(name string) Client {
	return m.clients[name]
}

// GetAllForges returns all registered forge clients
func (m *Manager) GetAllForges() map[string]Client {
	return m.clients
}

// GetForgeConfigs returns all forge configurations
func (m *Manager) GetForgeConfigs() map[string]*Config {
	return m.configs
}

// ToConfigRepository converts a forge repository to a config repository
func (r *Repository) ToConfigRepository(auth *config.AuthConfig) config.Repository {
	// Use SSH URL if available and auth is SSH, otherwise use clone URL
	url := r.CloneURL
	if auth != nil && auth.Type == "ssh" && r.SSHURL != "" {
		url = r.SSHURL
	}

	return config.Repository{
		URL:    url,
		Name:   r.Name,
		Branch: r.DefaultBranch,
		Auth:   auth,
		Paths:  []string{"docs"}, // Default paths
		Tags: map[string]string{
			"forge_id":     r.ID,
			"full_name":    r.FullName,
			"description":  r.Description,
			"language":     r.Language,
			"private":      fmt.Sprintf("%t", r.Private),
			"has_docs":     fmt.Sprintf("%t", r.HasDocs),
			"last_updated": r.LastUpdated.Format(time.RFC3339),
			// forge_type is injected later by discovery when we know the client type; added defensively here if metadata contains it
			// leaving empty if absent avoids breaking existing configs but enables downstream hugo edit link logic to prefer explicit type.
			"forge_type": r.Metadata["forge_type"],
		},
	}
}
