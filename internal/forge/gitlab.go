package forge

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"
	cfg "git.home.luguber.info/inful/docbuilder/internal/config"
)

// GitLabClient implements ForgeClient for GitLab
type GitLabClient struct {
	config     *ForgeConfig
	httpClient *http.Client
	baseURL    string
	apiURL     string
	token      string
}

// NewGitLabClient creates a new GitLab client
func NewGitLabClient(fg *ForgeConfig) (*GitLabClient, error) {
	if fg.Type != string(ForgeTypeGitLab) {
		return nil, fmt.Errorf("invalid forge type for GitLab client: %s", fg.Type)
	}

	client := &GitLabClient{
		config:     fg,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		apiURL:     fg.APIURL,
		baseURL:    fg.BaseURL,
	}

	// Set default URLs if not provided
	if client.apiURL == "" {
		client.apiURL = "https://gitlab.com/api/v4"
	}
	if client.baseURL == "" {
		client.baseURL = "https://gitlab.com"
	}

	// Extract token from auth config
	if fg.Auth != nil && fg.Auth.Type == cfg.AuthTypeToken {
		client.token = fg.Auth.Token
	} else {
		return nil, fmt.Errorf("GitLab client requires token authentication")
	}

	return client, nil
}

// GetType returns the forge type
func (c *GitLabClient) GetType() ForgeType {
	return ForgeTypeGitLab
}

// GetName returns the configured name
func (c *GitLabClient) GetName() string {
	return c.config.Name
}

// gitlabGroup represents a GitLab group
type gitlabGroup struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Path        string `json:"path"`
	Description string `json:"description"`
	FullName    string `json:"full_name"`
	FullPath    string `json:"full_path"`
	Kind        string `json:"kind"`
}

// gitlabProject represents a GitLab project (repository)
type gitlabProject struct {
	ID                int                `json:"id"`
	Name              string             `json:"name"`
	Path              string             `json:"path"`
	NameWithNamespace string             `json:"name_with_namespace"`
	PathWithNamespace string             `json:"path_with_namespace"`
	Description       string             `json:"description"`
	DefaultBranch     string             `json:"default_branch"`
	HTTPURLToRepo     string             `json:"http_url_to_repo"`
	SSHURLToRepo      string             `json:"ssh_url_to_repo"`
	Visibility        string             `json:"visibility"`
	Archived          bool               `json:"archived"`
	LastActivityAt    time.Time          `json:"last_activity_at"`
	Topics            []string           `json:"topics"`
	Languages         map[string]float64 `json:"languages,omitempty"`
	Namespace         gitlabNamespace    `json:"namespace"`
}

// gitlabNamespace represents a GitLab namespace
type gitlabNamespace struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Path     string `json:"path"`
	Kind     string `json:"kind"`
	FullPath string `json:"full_path"`
}

// ListOrganizations returns accessible groups
func (c *GitLabClient) ListOrganizations(ctx context.Context) ([]*Organization, error) {
	var orgs []*Organization
	page := 1
	perPage := 100

	for {
		endpoint := fmt.Sprintf("/groups?per_page=%d&page=%d&order_by=name", perPage, page)
		req, err := c.newRequest(ctx, "GET", endpoint, nil)
		if err != nil {
			return nil, err
		}

		var gitlabGroups []gitlabGroup
		if err := c.doRequest(req, &gitlabGroups); err != nil {
			return nil, err
		}

		if len(gitlabGroups) == 0 {
			break
		}

		for _, gGroup := range gitlabGroups {
			org := &Organization{
				ID:          strconv.Itoa(gGroup.ID),
				Name:        gGroup.Path,
				DisplayName: gGroup.Name,
				Description: gGroup.Description,
				Type:        gGroup.Kind,
				Metadata: map[string]string{
					"gitlab_id": strconv.Itoa(gGroup.ID),
					"full_path": gGroup.FullPath,
					"full_name": gGroup.FullName,
					"kind":      gGroup.Kind,
				},
			}
			orgs = append(orgs, org)
		}

		if len(gitlabGroups) < perPage {
			break
		}
		page++
	}

	return orgs, nil
}

// ListRepositories returns repositories for specified groups
func (c *GitLabClient) ListRepositories(ctx context.Context, groups []string) ([]*Repository, error) {
	var allRepos []*Repository

	for _, group := range groups {
		repos, err := c.getGroupProjects(ctx, group)
		if err != nil {
			return nil, fmt.Errorf("failed to get projects for group %s: %w", group, err)
		}
		allRepos = append(allRepos, repos...)
	}

	return allRepos, nil
}

// getGroupProjects gets all projects for a group
func (c *GitLabClient) getGroupProjects(ctx context.Context, group string) ([]*Repository, error) {
	var allRepos []*Repository
	page := 1
	perPage := 100

	for {
		endpoint := fmt.Sprintf("/groups/%s/projects?per_page=%d&page=%d&order_by=last_activity_at&include_subgroups=true",
			url.PathEscape(group), perPage, page)
		req, err := c.newRequest(ctx, "GET", endpoint, nil)
		if err != nil {
			return nil, err
		}

		var gitlabProjects []gitlabProject
		if err := c.doRequest(req, &gitlabProjects); err != nil {
			return nil, err
		}

		if len(gitlabProjects) == 0 {
			break
		}

		for _, gProject := range gitlabProjects {
			repo := c.convertGitLabProject(&gProject)
			allRepos = append(allRepos, repo)
		}

		if len(gitlabProjects) < perPage {
			break
		}
		page++
	}

	return allRepos, nil
}

// GetRepository gets detailed information about a specific repository
func (c *GitLabClient) GetRepository(ctx context.Context, owner, repo string) (*Repository, error) {
	projectPath := fmt.Sprintf("%s/%s", owner, repo)
	endpoint := fmt.Sprintf("/projects/%s", url.PathEscape(projectPath))
	req, err := c.newRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	var gitlabProject gitlabProject
	if err := c.doRequest(req, &gitlabProject); err != nil {
		return nil, err
	}

	return c.convertGitLabProject(&gitlabProject), nil
}

// CheckDocumentation checks if repository has docs folder and .docignore
func (c *GitLabClient) CheckDocumentation(ctx context.Context, repo *Repository) error {
	projectPath := repo.FullName

	// Check for docs folder
	hasDocs, err := c.checkPathExists(ctx, projectPath, "docs", repo.DefaultBranch)
	if err != nil {
		return fmt.Errorf("failed to check docs folder: %w", err)
	}
	repo.HasDocs = hasDocs

	// Check for .docignore file
	hasDocIgnore, err := c.checkPathExists(ctx, projectPath, ".docignore", repo.DefaultBranch)
	if err != nil {
		return fmt.Errorf("failed to check .docignore file: %w", err)
	}
	repo.HasDocIgnore = hasDocIgnore

	return nil
}

// checkPathExists checks if a path exists in the project
func (c *GitLabClient) checkPathExists(ctx context.Context, projectPath, filePath, branch string) (bool, error) {
	endpoint := fmt.Sprintf("/projects/%s/repository/files/%s?ref=%s",
		url.PathEscape(projectPath),
		url.PathEscape(filePath),
		url.PathEscape(branch))

	req, err := c.newRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return false, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return true, nil
}

// ValidateWebhook validates GitLab webhook signature
func (c *GitLabClient) ValidateWebhook(payload []byte, signature string, secret string) bool {
	// GitLab sends X-Gitlab-Token header with the secret
	return signature == secret
}

// ParseWebhookEvent parses GitLab webhook payload
func (c *GitLabClient) ParseWebhookEvent(payload []byte, eventType string) (*WebhookEvent, error) {
	switch eventType {
	case "push", "Push Hook":
		return c.parsePushEvent(payload)
	case "tag_push", "Tag Push Hook":
		return c.parseTagPushEvent(payload)
	case "repository", "Repository Update Hook":
		return c.parseRepositoryEvent(payload)
	default:
		return nil, fmt.Errorf("unsupported event type: %s", eventType)
	}
}

// gitlabPushEvent represents a GitLab push event
type gitlabPushEvent struct {
	Ref        string           `json:"ref"`
	Project    gitlabProject    `json:"project"`
	Commits    []gitlabCommit   `json:"commits"`
	Repository gitlabRepository `json:"repository"`
}

// gitlabCommit represents a GitLab commit
type gitlabCommit struct {
	ID        string       `json:"id"`
	Message   string       `json:"message"`
	Timestamp time.Time    `json:"timestamp"`
	Author    gitlabAuthor `json:"author"`
	Added     []string     `json:"added"`
	Modified  []string     `json:"modified"`
	Removed   []string     `json:"removed"`
}

// gitlabAuthor represents a GitLab commit author
type gitlabAuthor struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// gitlabRepository represents a GitLab repository in webhook
type gitlabRepository struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Homepage    string `json:"homepage"`
	GitHTTPURL  string `json:"git_http_url"`
	GitSSHURL   string `json:"git_ssh_url"`
	Visibility  string `json:"visibility_level"`
}

// parsePushEvent parses a GitLab push event
func (c *GitLabClient) parsePushEvent(payload []byte) (*WebhookEvent, error) {
	var pushEvent gitlabPushEvent
	if err := json.Unmarshal(payload, &pushEvent); err != nil {
		return nil, err
	}
	if pushEvent.Project.ID == 0 { // zero value detection via ID
		return nil, fmt.Errorf("missing project in push event")
	}
	branch := strings.TrimPrefix(pushEvent.Ref, "refs/heads/")
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
		Repository: c.convertGitLabProject(&pushEvent.Project),
		Branch:     branch,
		Commits:    commits,
		Timestamp:  time.Now(),
		Metadata:   map[string]string{"ref": pushEvent.Ref},
	}, nil
}

// parseTagPushEvent parses a GitLab tag push event
func (c *GitLabClient) parseTagPushEvent(payload []byte) (*WebhookEvent, error) {
	var pushEvent gitlabPushEvent
	if err := json.Unmarshal(payload, &pushEvent); err != nil {
		return nil, err
	}
	if pushEvent.Project.ID == 0 {
		return nil, fmt.Errorf("missing project in tag push event")
	}
	// Extract tag name from ref (refs/tags/v1.0.0 -> v1.0.0)
	tag := strings.TrimPrefix(pushEvent.Ref, "refs/tags/")
	return &WebhookEvent{
		Type:       WebhookEventTag,
		Repository: c.convertGitLabProject(&pushEvent.Project),
		Branch:     tag, // reuse Branch field to carry the tag reference (could extend struct later)
		Timestamp:  time.Now(),
		Metadata:   map[string]string{"ref": pushEvent.Ref, "tag": tag},
	}, nil
}

// parseRepositoryEvent parses a GitLab repository event
func (c *GitLabClient) parseRepositoryEvent(payload []byte) (*WebhookEvent, error) {
	var repoEvent map[string]interface{}
	if err := json.Unmarshal(payload, &repoEvent); err != nil {
		return nil, err
	}

	event := &WebhookEvent{Type: WebhookEventRepository, Timestamp: time.Now(), Changes: make(map[string]string), Metadata: make(map[string]string)}
	if project, ok := repoEvent["project"].(map[string]interface{}); ok {
		if projectData, err := json.Marshal(project); err == nil {
			var gProj gitlabProject
			if err := json.Unmarshal(projectData, &gProj); err == nil {
				event.Repository = c.convertGitLabProject(&gProj)
			}
		}
	} else {
		return nil, fmt.Errorf("missing project in repository event")
	}
	return event, nil
}

// RegisterWebhook registers a webhook for a project
func (c *GitLabClient) RegisterWebhook(ctx context.Context, repo *Repository, webhookURL string) error {
	if c.config.Webhook == nil {
		return fmt.Errorf("webhook not configured for forge %s", c.config.Name)
	}

	endpoint := fmt.Sprintf("/projects/%s/hooks", url.PathEscape(repo.FullName))

	events := c.config.Webhook.Events
	if len(events) == 0 {
		events = []string{"push_events", "repository_update_events"}
	}

	payload := map[string]interface{}{
		"url":   webhookURL,
		"token": c.config.Webhook.Secret,
	}

	// Set event flags
	for _, event := range events {
		switch event {
		case "push", "push_events":
			payload["push_events"] = true
		case "repository", "repository_update_events":
			payload["repository_update_events"] = true
		}
	}

	req, err := c.newRequest(ctx, "POST", endpoint, payload)
	if err != nil {
		return err
	}

	var result map[string]interface{}
	return c.doRequest(req, &result)
}

// GetEditURL returns the GitLab edit URL for a file
func (c *GitLabClient) GetEditURL(repo *Repository, filePath string, branch string) string {
	return fmt.Sprintf("%s/%s/-/edit/%s/%s", c.baseURL, repo.FullName, branch, filePath)
}

// Helper methods

func (c *GitLabClient) convertGitLabProject(gProject *gitlabProject) *Repository {
	// Determine primary language from languages map
	primaryLanguage := ""
	maxPercentage := 0.0
	for lang, percentage := range gProject.Languages {
		if percentage > maxPercentage {
			maxPercentage = percentage
			primaryLanguage = lang
		}
	}

	return &Repository{
		ID:            strconv.Itoa(gProject.ID),
		Name:          gProject.Path,
		FullName:      gProject.PathWithNamespace,
		CloneURL:      gProject.HTTPURLToRepo,
		SSHURL:        gProject.SSHURLToRepo,
		DefaultBranch: gProject.DefaultBranch,
		Description:   gProject.Description,
		Private:       gProject.Visibility != "public",
		Archived:      gProject.Archived,
		LastUpdated:   gProject.LastActivityAt,
		Topics:        gProject.Topics,
		Language:      primaryLanguage,
		Metadata: map[string]string{
			"gitlab_id":           strconv.Itoa(gProject.ID),
			"visibility":          gProject.Visibility,
			"name_with_namespace": gProject.NameWithNamespace,
			"namespace_kind":      gProject.Namespace.Kind,
			"namespace_path":      gProject.Namespace.Path,
		},
	}
}

func (c *GitLabClient) newRequest(ctx context.Context, method, endpoint string, body interface{}) (*http.Request, error) {
	u, err := url.Parse(c.apiURL)
	if err != nil {
		return nil, err
	}
	u.Path = path.Join(u.Path, endpoint)

	var req *http.Request
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		req, err = http.NewRequestWithContext(ctx, method, u.String(), strings.NewReader(string(jsonBody)))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
	} else {
		var err error
		req, err = http.NewRequestWithContext(ctx, method, u.String(), nil)
		if err != nil {
			return nil, err
		}
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("User-Agent", "DocBuilder/1.0")

	return req, nil
}

func (c *GitLabClient) doRequest(req *http.Request, result interface{}) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("GitLab API error: %s", resp.Status)
	}

	if result != nil {
		return json.NewDecoder(resp.Body).Decode(result)
	}

	return nil
}
