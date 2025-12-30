package forge

import (
	"context"
	"crypto/hmac"
	"crypto/sha1" // #nosec G505 -- SHA-1 needed for legacy GitHub webhook compatibility
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	cfg "git.home.luguber.info/inful/docbuilder/internal/config"
)

// GitHubClient implements ForgeClient for GitHub.
type GitHubClient struct {
	*BaseForge
	config  *Config
	baseURL string
}

// NewGitHubClient creates a new GitHub client.
func NewGitHubClient(fg *Config) (*GitHubClient, error) {
	if fg.Type != cfg.ForgeGitHub {
		return nil, fmt.Errorf("invalid forge type for GitHub client: %s", fg.Type)
	}

	// Set default URLs if not provided
	apiURL, baseURL := withDefaults(fg.APIURL, fg.BaseURL, "https://api.github.com", "https://github.com")

	// Extract token from auth config
	tok, err := tokenFromConfig(fg, "GitHub")
	if err != nil {
		return nil, err
	}

	// Create BaseForge with common HTTP operations
	baseForge := NewBaseForge(newHTTPClient30s(), apiURL, tok)

	// GitHub-specific headers
	baseForge.SetCustomHeader("Accept", "application/vnd.github+json")
	baseForge.SetCustomHeader("X-GitHub-Api-Version", "2022-11-28")

	return &GitHubClient{
		BaseForge: baseForge,
		config:    fg,
		baseURL:   baseURL,
	}, nil
}

// GetType returns the forge type.
func (c *GitHubClient) GetType() cfg.ForgeType { return cfg.ForgeGitHub }

// GetName returns the configured name.
func (c *GitHubClient) GetName() string {
	return c.config.Name
}

// githubOrg represents a GitHub organization.
type githubOrg struct {
	ID          int    `json:"id"`
	Login       string `json:"login"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Type        string `json:"type"`
}

// githubRepo represents a GitHub repository.
type githubRepo struct {
	ID            int       `json:"id"`
	Name          string    `json:"name"`
	FullName      string    `json:"full_name"`
	Description   string    `json:"description"`
	Private       bool      `json:"private"`
	CloneURL      string    `json:"clone_url"`
	SSHURL        string    `json:"ssh_url"`
	DefaultBranch string    `json:"default_branch"`
	Language      string    `json:"language"`
	Archived      bool      `json:"archived"`
	UpdatedAt     time.Time `json:"updated_at"`
	Topics        []string  `json:"topics"`
	Owner         githubOrg `json:"owner"`
}

// ListOrganizations returns accessible organizations.
func (c *GitHubClient) ListOrganizations(ctx context.Context) ([]*Organization, error) {
	var orgs []*Organization

	// Get user's organizations
	userOrgs, err := c.getUserOrganizations(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get user organizations: %w", err)
	}
	orgs = append(orgs, userOrgs...)

	return orgs, nil
}

// getUserOrganizations gets organizations for the authenticated user.
func (c *GitHubClient) getUserOrganizations(ctx context.Context) ([]*Organization, error) {
	req, err := c.NewRequest(ctx, "GET", "/user/orgs", nil)
	if err != nil {
		return nil, err
	}

	var githubOrgs []githubOrg
	if err := c.DoRequest(req, &githubOrgs); err != nil {
		return nil, err
	}

	var orgs []*Organization
	for _, gOrg := range githubOrgs {
		org := &Organization{
			ID:          strconv.Itoa(gOrg.ID),
			Name:        gOrg.Login,
			DisplayName: gOrg.Name,
			Description: gOrg.Description,
			Type:        gOrg.Type,
			Metadata: map[string]string{
				"github_id": strconv.Itoa(gOrg.ID),
				"type":      gOrg.Type,
			},
		}
		orgs = append(orgs, org)
	}

	return orgs, nil
}

// ListRepositories returns repositories for specified organizations.
func (c *GitHubClient) ListRepositories(ctx context.Context, organizations []string) ([]*Repository, error) {
	var allRepos []*Repository

	for _, org := range organizations {
		repos, err := c.getOrgRepositories(ctx, org)
		if err != nil {
			return nil, fmt.Errorf("failed to get repositories for org %s: %w", org, err)
		}
		allRepos = append(allRepos, repos...)
	}

	return allRepos, nil
}

// getOrgRepositories gets all repositories for an organization.
func (c *GitHubClient) getOrgRepositories(ctx context.Context, org string) ([]*Repository, error) {
	baseEndpoint := fmt.Sprintf("/orgs/%s/repos?sort=updated", org)
	return c.fetchAndConvertRepos(ctx, baseEndpoint, 100)
}

// fetchAndConvertRepos is a helper that fetches paginated repositories and converts them.
func (c *GitHubClient) fetchAndConvertRepos(ctx context.Context, endpoint string, pageSize int) ([]*Repository, error) {
	githubRepos, err := PaginatedFetchHelper(
		ctx,
		endpoint,
		"page",
		"per_page",
		pageSize,
		func(ep string) ([]githubRepo, bool, error) {
			req, err := c.NewRequest(ctx, "GET", ep, nil)
			if err != nil {
				return nil, false, err
			}

			var repos []githubRepo
			if err := c.DoRequest(req, &repos); err != nil {
				return nil, false, err
			}

			// Has more if we got a full page
			hasMore := len(repos) >= pageSize
			return repos, hasMore, nil
		},
	)

	if err != nil {
		return nil, err
	}

	// Convert to common Repository format
	allRepos := make([]*Repository, 0, len(githubRepos))
	for _, gRepo := range githubRepos {
		repo := c.convertGitHubRepo(&gRepo)
		allRepos = append(allRepos, repo)
	}

	return allRepos, nil
}

// GetRepository gets detailed information about a specific repository.
func (c *GitHubClient) GetRepository(ctx context.Context, owner, repo string) (*Repository, error) {
	endpoint := fmt.Sprintf("/repos/%s/%s", owner, repo)
	req, err := c.NewRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	var githubRepository githubRepo
	if err = c.DoRequest(req, &githubRepository); err != nil {
		return nil, err
	}

	return c.convertGitHubRepo(&githubRepository), nil
}

// CheckDocumentation checks if repository has docs folder and .docignore.
func (c *GitHubClient) CheckDocumentation(ctx context.Context, repo *Repository) error {
	owner, repoName := c.splitFullName(repo.FullName)

	// Check for docs folder
	hasDocs, err := c.checkPathExists(ctx, owner, repoName, "docs", repo.DefaultBranch)
	if err != nil {
		return fmt.Errorf("failed to check docs folder: %w", err)
	}
	repo.HasDocs = hasDocs

	// Check for .docignore file
	hasDocIgnore, err := c.checkPathExists(ctx, owner, repoName, ".docignore", repo.DefaultBranch)
	if err != nil {
		return fmt.Errorf("failed to check .docignore file: %w", err)
	}
	repo.HasDocIgnore = hasDocIgnore

	return nil
}

// checkPathExists checks if a path exists in the repository.
func (c *GitHubClient) checkPathExists(ctx context.Context, owner, repo, path, branch string) (bool, error) {
	endpoint := fmt.Sprintf("/repos/%s/%s/contents/%s?ref=%s", owner, repo, path, branch)
	req, err := c.NewRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return false, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return true, nil
}

// ValidateWebhook validates the GitHub webhook signature.
func (c *GitHubClient) ValidateWebhook(payload []byte, signature, secret string) bool {
	if signature == "" || secret == "" {
		return false
	}

	// Preferred SHA-256 format: sha256=<hash>
	if strings.HasPrefix(signature, "sha256=") {
		expected := signature[len("sha256="):]
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(payload)
		calc := hex.EncodeToString(mac.Sum(nil))
		return hmac.Equal([]byte(expected), []byte(calc))
	}

	// Fallback legacy SHA-1 format: sha1=<hash>
	if strings.HasPrefix(signature, "sha1=") {
		expected := signature[len("sha1="):]
		mac := hmac.New(sha1.New, []byte(secret)) //nolint:gosec // legacy GitHub webhook signature fallback
		mac.Write(payload)
		calc := hex.EncodeToString(mac.Sum(nil))
		return hmac.Equal([]byte(expected), []byte(calc))
	}

	return false
}

// ParseWebhookEvent parses GitHub webhook payload.
func (c *GitHubClient) ParseWebhookEvent(payload []byte, eventType string) (*WebhookEvent, error) {
	switch eventType {
	case "push":
		return c.parsePushEvent(payload)
	case "repository":
		return c.parseRepositoryEvent(payload)
	default:
		return nil, fmt.Errorf("unsupported event type: %s", eventType)
	}
}

// githubPushEvent represents a GitHub push event.
type githubPushEvent struct {
	Ref        string          `json:"ref"`
	Repository json.RawMessage `json:"repository"` // decode later to handle id as string/int
	Commits    []githubCommit  `json:"commits"`
	HeadCommit githubCommit    `json:"head_commit"`
}

// githubCommit represents a GitHub commit.
type githubCommit struct {
	ID        string    `json:"id"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
	Author    struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	} `json:"author"`
	Added    []string `json:"added"`
	Modified []string `json:"modified"`
	Removed  []string `json:"removed"`
}

// parsePushEvent parses a GitHub push event.
func (c *GitHubClient) parsePushEvent(payload []byte) (*WebhookEvent, error) {
	var pushEvent githubPushEvent
	if err := json.Unmarshal(payload, &pushEvent); err != nil {
		return nil, err
	}

	if len(pushEvent.Repository) == 0 {
		return nil, fmt.Errorf("missing repository in push event")
	}

	// Decode repository allowing id to be string or int
	var repoMap map[string]interface{}
	if err := json.Unmarshal(pushEvent.Repository, &repoMap); err != nil {
		return nil, err
	}
	// Normalize id to int if it's a string
	if rawID, ok := repoMap["id"].(string); ok {
		if intID, convErr := strconv.Atoi(rawID); convErr == nil {
			repoMap["id"] = intID
		}
	}
	repoBytes, _ := json.Marshal(repoMap)
	var repo githubRepo
	if err := json.Unmarshal(repoBytes, &repo); err != nil {
		return nil, err
	}

	// Extract branch name from ref (refs/heads/main -> main)
	branch := strings.TrimPrefix(pushEvent.Ref, "refs/heads/")

	// Convert commits
	var commits []WebhookCommit
	for _, commit := range pushEvent.Commits {
		commits = append(commits, WebhookCommit{
			ID:        commit.ID,
			Message:   commit.Message,
			Author:    commit.Author.Name,
			Timestamp: commit.Timestamp,
			Added:     commit.Added,
			Modified:  commit.Modified,
			Removed:   commit.Removed,
		})
	}

	return &WebhookEvent{
		Type:       WebhookEventPush,
		Repository: c.convertGitHubRepo(&repo),
		Branch:     branch,
		Commits:    commits,
		Timestamp:  time.Now(),
		Metadata: map[string]string{
			"ref":         pushEvent.Ref,
			"head_commit": pushEvent.HeadCommit.ID,
		},
	}, nil
}

// githubRepositoryEvent represents a GitHub repository event.
type githubRepositoryEvent struct {
	Action     string          `json:"action"`
	Repository json.RawMessage `json:"repository"`
	Changes    struct {
		Repository struct {
			Name struct {
				From string `json:"from"`
			} `json:"name"`
		} `json:"repository"`
	} `json:"changes"`
}

// parseRepositoryEvent parses a GitHub repository event.
func (c *GitHubClient) parseRepositoryEvent(payload []byte) (*WebhookEvent, error) {
	var repoEvent githubRepositoryEvent
	if err := json.Unmarshal(payload, &repoEvent); err != nil {
		return nil, err
	}

	if len(repoEvent.Repository) == 0 {
		return nil, fmt.Errorf("missing repository in repository event")
	}

	var repoMap map[string]interface{}
	if err := json.Unmarshal(repoEvent.Repository, &repoMap); err != nil {
		return nil, err
	}
	if rawID, ok := repoMap["id"].(string); ok {
		if intID, convErr := strconv.Atoi(rawID); convErr == nil {
			repoMap["id"] = intID
		}
	}
	repoBytes, _ := json.Marshal(repoMap)
	var repo githubRepo
	if err := json.Unmarshal(repoBytes, &repo); err != nil {
		return nil, err
	}

	event := &WebhookEvent{
		Type:       WebhookEventRepository,
		Repository: c.convertGitHubRepo(&repo),
		Action:     repoEvent.Action,
		Timestamp:  time.Now(),
		Changes:    make(map[string]string),
		Metadata: map[string]string{
			"action": repoEvent.Action,
		},
	}

	if repoEvent.Action == "renamed" && repoEvent.Changes.Repository.Name.From != "" {
		event.Changes["name_from"] = repoEvent.Changes.Repository.Name.From
		event.Changes["name_to"] = repo.Name
	}

	return event, nil
}

// RegisterWebhook registers a webhook for a repository.
func (c *GitHubClient) RegisterWebhook(ctx context.Context, repo *Repository, webhookURL string) error {
	if c.config.Webhook == nil {
		return fmt.Errorf("webhook not configured for forge %s", c.config.Name)
	}

	owner, repoName := c.splitFullName(repo.FullName)
	endpoint := fmt.Sprintf("/repos/%s/%s/hooks", owner, repoName)

	webhookConfig := map[string]interface{}{
		"url":          webhookURL,
		"content_type": "json",
		"secret":       c.config.Webhook.Secret,
	}

	events := c.config.Webhook.Events
	if len(events) == 0 {
		events = []string{"push", "repository"}
	}

	payload := map[string]interface{}{
		"config": webhookConfig,
		"events": events,
		"active": true,
	}

	req, err := c.NewRequest(ctx, "POST", endpoint, payload)
	if err != nil {
		return err
	}

	var result map[string]interface{}
	return c.DoRequest(req, &result)
}

// GetEditURL returns the URL to edit a file in GitHub.
func (c *GitHubClient) GetEditURL(repo *Repository, filePath, branch string) string {
	return GenerateEditURL(TypeGitHub, c.baseURL, repo.FullName, branch, filePath)
}

// Helper methods

func (c *GitHubClient) convertGitHubRepo(gRepo *githubRepo) *Repository {
	return &Repository{
		ID:            strconv.Itoa(gRepo.ID),
		Name:          gRepo.Name,
		FullName:      gRepo.FullName,
		CloneURL:      gRepo.CloneURL,
		SSHURL:        gRepo.SSHURL,
		DefaultBranch: gRepo.DefaultBranch,
		Description:   gRepo.Description,
		Private:       gRepo.Private,
		Archived:      gRepo.Archived,
		LastUpdated:   gRepo.UpdatedAt,
		Topics:        gRepo.Topics,
		Language:      gRepo.Language,
		Metadata: map[string]string{
			"github_id":  strconv.Itoa(gRepo.ID),
			"owner":      gRepo.Owner.Login,
			"owner_type": gRepo.Owner.Type,
		},
	}
}

func (c *GitHubClient) splitFullName(fullName string) (owner, repo string) {
	parts := strings.SplitN(fullName, "/", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "", fullName
}
