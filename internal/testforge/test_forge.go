// Package testforge provides a test implementation of forge interfaces for testing purposes.
package testforge

import (
	"context"
	"fmt"
	"strings"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/forge"
)

// TestForge implements a mock forge for testing purposes
type TestForge struct {
	name          string
	forgeType     config.ForgeType
	repositories  []TestRepository
	organizations []string
	failMode      FailMode
	delay         time.Duration
}

// TestRepository represents a mock repository
type TestRepository struct {
	ID            string
	Name          string
	FullName      string
	CloneURL      string
	SSHURL        string
	DefaultBranch string
	Description   string
	Topics        []string
	Language      string
	Private       bool
	Archived      bool
	Fork          bool
	HasDocs       bool
	HasDocIgnore  bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
	LastUpdated   time.Time
	Metadata      map[string]string
}

// TestOrganization represents a mock organization
type TestOrganization struct {
	ID          string
	Name        string
	DisplayName string
	Description string
	Type        string
}

// FailMode defines how the test forge should behave
type FailMode int

const (
	FailModeNone FailMode = iota
	FailModeAuth
	FailModeNetwork
	FailModeRateLimit
	FailModeNotFound
)

// NewTestForge creates a new test forge with default repositories
func NewTestForge(name string, forgeType config.ForgeType) *TestForge {
	return &TestForge{
		name:      name,
		forgeType: forgeType,
		repositories: []TestRepository{
			{
				ID:            "1",
				Name:          "docs-repo",
				FullName:      "test-org/docs-repo",
				CloneURL:      "https://test-forge.example.com/test-org/docs-repo.git",
				SSHURL:        "git@test-forge.example.com:test-org/docs-repo.git",
				DefaultBranch: "main",
				Description:   "Documentation repository for testing",
				Topics:        []string{"docs", "testing"},
				Language:      "Markdown",
				Private:       false,
				Archived:      false,
				Fork:          false,
				HasDocs:       true,
				HasDocIgnore:  false,
				CreatedAt:     time.Now().Add(-30 * 24 * time.Hour),
				UpdatedAt:     time.Now().Add(-1 * time.Hour),
				LastUpdated:   time.Now().Add(-1 * time.Hour),
				Metadata:      map[string]string{"test": "true"},
			},
			{
				ID:            "2",
				Name:          "api-docs",
				FullName:      "test-org/api-docs",
				CloneURL:      "https://test-forge.example.com/test-org/api-docs.git",
				SSHURL:        "git@test-forge.example.com:test-org/api-docs.git",
				DefaultBranch: "main",
				Description:   "API documentation",
				Topics:        []string{"api", "documentation"},
				Language:      "Markdown",
				Private:       false,
				Archived:      false,
				Fork:          false,
				HasDocs:       true,
				HasDocIgnore:  false,
				CreatedAt:     time.Now().Add(-20 * 24 * time.Hour),
				UpdatedAt:     time.Now().Add(-2 * time.Hour),
				LastUpdated:   time.Now().Add(-2 * time.Hour),
				Metadata:      map[string]string{"test": "true"},
			},
			{
				Name:        "archived-docs",
				FullName:    "test-org/archived-docs",
				CloneURL:    "https://test-forge.example.com/test-org/archived-docs.git",
				Description: "Archived documentation",
				Topics:      []string{"legacy"},
				Language:    "Markdown",
				Private:     false,
				Archived:    true,
				Fork:        false,
				CreatedAt:   time.Now().Add(-100 * 24 * time.Hour),
				UpdatedAt:   time.Now().Add(-50 * 24 * time.Hour),
			},
			{
				Name:        "private-docs",
				FullName:    "test-org/private-docs",
				CloneURL:    "https://test-forge.example.com/test-org/private-docs.git",
				Description: "Private documentation",
				Topics:      []string{"internal"},
				Language:    "Markdown",
				Private:     true,
				Archived:    false,
				Fork:        false,
				CreatedAt:   time.Now().Add(-10 * 24 * time.Hour),
				UpdatedAt:   time.Now().Add(-30 * time.Minute),
			},
		},
		organizations: []string{"test-org", "docs-org"},
		failMode:      FailModeNone,
		delay:         0,
	}
}

// SetFailMode configures how the test forge should fail
func (tf *TestForge) SetFailMode(mode FailMode) {
	tf.failMode = mode
}

// SetDelay adds artificial delay to simulate network latency
func (tf *TestForge) SetDelay(delay time.Duration) {
	tf.delay = delay
}

// AddRepository adds a test repository
func (tf *TestForge) AddRepository(repo TestRepository) {
	tf.repositories = append(tf.repositories, repo)
}

// AddOrganization adds a test organization
func (tf *TestForge) AddOrganization(org string) {
	tf.organizations = append(tf.organizations, org)
}

// ClearRepositories removes all repositories
func (tf *TestForge) ClearRepositories() {
	tf.repositories = nil
}

// ClearOrganizations removes all organizations
func (tf *TestForge) ClearOrganizations() {
	tf.organizations = nil
}

// simulate adds delay and checks for failure modes
func (tf *TestForge) simulate() error {
	if tf.delay > 0 {
		time.Sleep(tf.delay)
	}

	switch tf.failMode {
	case FailModeAuth:
		return fmt.Errorf("authentication failed: invalid credentials")
	case FailModeNetwork:
		return fmt.Errorf("network error: connection timeout")
	case FailModeRateLimit:
		return fmt.Errorf("rate limit exceeded: try again later")
	case FailModeNotFound:
		return fmt.Errorf("not found: resource does not exist")
	default:
		return nil
	}
}

// GetUserOrganizations returns test organizations
func (tf *TestForge) GetUserOrganizations(ctx context.Context) ([]forge.Organization, error) {
	if err := tf.simulate(); err != nil {
		return nil, err
	}

	var orgs []forge.Organization
	for _, orgName := range tf.organizations {
		orgs = append(orgs, forge.Organization{
			Name:        orgName,
			DisplayName: strings.Title(orgName),
			Description: fmt.Sprintf("Test organization: %s", orgName),
		})
	}

	return orgs, nil
}

// GetRepositoriesForOrganization returns test repositories for an organization
func (tf *TestForge) GetRepositoriesForOrganization(ctx context.Context, orgName string) ([]forge.Repository, error) {
	if err := tf.simulate(); err != nil {
		return nil, err
	}

	var repos []forge.Repository
	for _, testRepo := range tf.repositories {
		// Check if repository belongs to the organization
		if strings.HasPrefix(testRepo.FullName, orgName+"/") {
			repos = append(repos, forge.Repository{
				ID:            fmt.Sprintf("test-%d", len(repos)+1),
				Name:          testRepo.Name,
				FullName:      testRepo.FullName,
				CloneURL:      testRepo.CloneURL,
				SSHURL:        strings.Replace(testRepo.CloneURL, "https://", "git@", 1),
				DefaultBranch: "main",
				Description:   testRepo.Description,
				Private:       testRepo.Private,
				Archived:      testRepo.Archived,
				HasDocs:       true,
				HasDocIgnore:  false,
				LastUpdated:   testRepo.UpdatedAt,
				Topics:        testRepo.Topics,
				Language:      testRepo.Language,
				Metadata: map[string]string{
					"created_at": testRepo.CreatedAt.Format(time.RFC3339),
					"is_fork":    fmt.Sprintf("%t", testRepo.Fork),
				},
			})
		}
	}

	return repos, nil
}

// GetRepositoriesForGroup is an alias for GetRepositoriesForOrganization (GitLab terminology)
func (tf *TestForge) GetRepositoriesForGroup(ctx context.Context, groupName string) ([]forge.Repository, error) {
	return tf.GetRepositoriesForOrganization(ctx, groupName)
}

// Name returns the forge name
func (tf *TestForge) Name() string {
	return tf.name
}

// Type returns the forge type
func (tf *TestForge) Type() config.ForgeType {
	return tf.forgeType
}

// TestForgeFactory creates test forges for different types
type TestForgeFactory struct{}

// NewTestForgeFactory creates a new test forge factory
func NewTestForgeFactory() *TestForgeFactory {
	return &TestForgeFactory{}
}

// CreateGitHubTestForge creates a GitHub-like test forge
func (tff *TestForgeFactory) CreateGitHubTestForge(name string) *TestForge {
	forge := NewTestForge(name, config.ForgeGitHub)

	// Add GitHub-specific test repositories
	forge.AddRepository(TestRepository{
		Name:        "awesome-docs",
		FullName:    "github-org/awesome-docs",
		CloneURL:    "https://github.com/github-org/awesome-docs.git",
		Description: "Awesome documentation project",
		Topics:      []string{"documentation", "awesome"},
		Language:    "Markdown",
		Private:     false,
		Archived:    false,
		Fork:        false,
		CreatedAt:   time.Now().Add(-60 * 24 * time.Hour),
		UpdatedAt:   time.Now().Add(-5 * time.Minute),
	})

	forge.AddOrganization("github-org")
	return forge
}

// CreateGitLabTestForge creates a GitLab-like test forge
func (tff *TestForgeFactory) CreateGitLabTestForge(name string) *TestForge {
	forge := NewTestForge(name, config.ForgeGitLab)

	// Add GitLab-specific test repositories
	forge.AddRepository(TestRepository{
		Name:        "project-docs",
		FullName:    "gitlab-group/project-docs",
		CloneURL:    "https://gitlab.example.com/gitlab-group/project-docs.git",
		Description: "Project documentation on GitLab",
		Topics:      []string{"project", "gitlab"},
		Language:    "Markdown",
		Private:     false,
		Archived:    false,
		Fork:        false,
		CreatedAt:   time.Now().Add(-45 * 24 * time.Hour),
		UpdatedAt:   time.Now().Add(-15 * time.Minute),
	})

	forge.AddOrganization("gitlab-group")
	return forge
}

// CreateForgejoTestForge creates a Forgejo-like test forge
func (tff *TestForgeFactory) CreateForgejoTestForge(name string) *TestForge {
	forge := NewTestForge(name, config.ForgeForgejo)

	// Add Forgejo-specific test repositories
	forge.AddRepository(TestRepository{
		Name:        "self-hosted-docs",
		FullName:    "forgejo-org/self-hosted-docs",
		CloneURL:    "https://forgejo.example.com/forgejo-org/self-hosted-docs.git",
		Description: "Self-hosted documentation",
		Topics:      []string{"self-hosted", "forgejo"},
		Language:    "Markdown",
		Private:     false,
		Archived:    false,
		Fork:        false,
		CreatedAt:   time.Now().Add(-25 * 24 * time.Hour),
		UpdatedAt:   time.Now().Add(-1 * time.Hour),
	})

	forge.AddOrganization("forgejo-org")
	return forge
}

// TestForgeConfig creates a test forge configuration
func CreateTestForgeConfig(name string, forgeType config.ForgeType, organizations []string) config.ForgeConfig {
	return config.ForgeConfig{
		Name:    name,
		Type:    forgeType,
		APIURL:  "https://api.github.com",
		BaseURL: "https://github.com",
		Auth: &config.AuthConfig{
			Type:  config.AuthTypeToken,
			Token: "test-token-" + name,
		},
		Organizations: organizations,
		AutoDiscover:  false,
		Options: map[string]interface{}{
			"include_archived": false,
			"include_private":  false,
			"include_forks":    false,
			"topic_filter":     []string{"docs", "documentation"},
		},
	}
}

// TestDiscoveryScenario provides predefined test scenarios
type TestDiscoveryScenario struct {
	Name        string
	Description string
	Forges      []*TestForge
	Expected    ExpectedResults
}

// ExpectedResults defines what to expect from a test scenario
type ExpectedResults struct {
	TotalRepositories    int
	PublicRepositories   int
	ArchivedRepositories int
	PrivateRepositories  int
	Organizations        []string
	Topics               []string
}

// CreateTestScenarios returns predefined test scenarios
func CreateTestScenarios() []TestDiscoveryScenario {
	factory := NewTestForgeFactory()

	return []TestDiscoveryScenario{
		{
			Name:        "Multi-Forge Discovery",
			Description: "Test discovery across multiple forge types",
			Forges: []*TestForge{
				factory.CreateGitHubTestForge("github-test"),
				factory.CreateGitLabTestForge("gitlab-test"),
				factory.CreateForgejoTestForge("forgejo-test"),
			},
			Expected: ExpectedResults{
				TotalRepositories:    7, // 4 default + 3 specific
				PublicRepositories:   6,
				ArchivedRepositories: 1,
				PrivateRepositories:  1,
				Organizations:        []string{"test-org", "github-org", "gitlab-group", "forgejo-org"},
				Topics:               []string{"docs", "documentation", "api", "awesome", "project", "self-hosted"},
			},
		},
		{
			Name:        "Failure Recovery",
			Description: "Test system resilience to forge failures",
			Forges: []*TestForge{
				func() *TestForge {
					forge := factory.CreateGitHubTestForge("failing-github")
					forge.SetFailMode(FailModeAuth)
					return forge
				}(),
				factory.CreateGitLabTestForge("working-gitlab"),
			},
			Expected: ExpectedResults{
				TotalRepositories: 2, // Only from working forge
				Organizations:     []string{"gitlab-group"},
			},
		},
		{
			Name:        "Empty Discovery",
			Description: "Test discovery with no repositories",
			Forges: []*TestForge{
				func() *TestForge {
					forge := NewTestForge("empty-forge", config.ForgeGitHub)
					forge.ClearRepositories()
					return forge
				}(),
			},
			Expected: ExpectedResults{
				TotalRepositories: 0,
				Organizations:     []string{"test-org", "docs-org"},
			},
		},
	}
}

// ToConfigRepositories converts TestForge repositories to config.Repository format
func (tf *TestForge) ToConfigRepositories() []config.Repository {
	var configRepos []config.Repository

	for _, repo := range tf.repositories {
		configRepos = append(configRepos, config.Repository{
			Name:   repo.Name,
			URL:    repo.CloneURL,
			Branch: repo.DefaultBranch,
			Paths:  []string{"docs"}, // Default paths
			Tags: map[string]string{
				"description": repo.Description,
				"language":    repo.Language,
				"private":     fmt.Sprintf("%t", repo.Private),
				"archived":    fmt.Sprintf("%t", repo.Archived),
				"fork":        fmt.Sprintf("%t", repo.Fork),
			},
		})
	}

	return configRepos
}

// ToForgeConfig converts TestForge to a forge configuration
func (tf *TestForge) ToForgeConfig() *config.ForgeConfig {
	var apiURL, baseURL string

	switch tf.forgeType {
	case config.ForgeGitHub:
		apiURL = "https://api.github.com"
		baseURL = "https://github.com"
	case config.ForgeGitLab:
		apiURL = "https://gitlab.com/api/v4"
		baseURL = "https://gitlab.com"
	case config.ForgeForgejo:
		apiURL = "https://forgejo.example.com/api/v1"
		baseURL = "https://forgejo.example.com"
	default:
		apiURL = fmt.Sprintf("https://api.%s.example.com", tf.name)
		baseURL = fmt.Sprintf("https://%s.example.com", tf.name)
	}

	return &config.ForgeConfig{
		Name:          tf.name,
		Type:          tf.forgeType,
		APIURL:        apiURL,
		BaseURL:       baseURL,
		Organizations: tf.organizations,
		Auth: &config.AuthConfig{
			Type:  config.AuthTypeToken,
			Token: fmt.Sprintf("%s-test-token", tf.name),
		},
		Webhook: &config.WebhookConfig{
			Secret: fmt.Sprintf("%s-webhook-secret", tf.name),
			Path:   fmt.Sprintf("/webhooks/%s", tf.name),
			Events: []string{"push", "repository"},
		},
	}
}
