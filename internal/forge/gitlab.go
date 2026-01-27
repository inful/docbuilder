package forge

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	cfg "git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/foundation/errors"
)

const defaultMainBranch = "main"

// GitLabClient implements ForgeClient for GitLab.
type GitLabClient struct {
	*BaseForge
	config  *Config
	baseURL string
}

// NewGitLabClient creates a new GitLab client.
func NewGitLabClient(fg *Config) (*GitLabClient, error) {
	if fg.Type != cfg.ForgeGitLab {
		return nil, errors.ForgeError("invalid forge type for GitLab client").
			WithContext("type", fg.Type).
			Fatal().
			Build()
	}

	// Set default URLs if not provided
	apiURL, baseURL := withDefaults(fg.APIURL, fg.BaseURL, "https://gitlab.com/api/v4", "https://gitlab.com")

	// Extract token from auth config
	tok, err := tokenFromConfig(fg, "GitLab")
	if err != nil {
		return nil, err
	}

	// Create BaseForge with common HTTP operations
	baseForge := NewBaseForge(newHTTPClient30s(), apiURL, tok)
	// GitLab uses Bearer auth (default), no custom headers needed

	return &GitLabClient{
		BaseForge: baseForge,
		config:    fg,
		baseURL:   baseURL,
	}, nil
}

func (c *GitLabClient) GetType() cfg.ForgeType { return cfg.ForgeGitLab }

// GetName returns the configured name.
func (c *GitLabClient) GetName() string {
	return c.config.Name
}

type gitlabGroup struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Path        string `json:"path"`
	Description string `json:"description"`
	FullName    string `json:"full_name"`
	FullPath    string `json:"full_path"`
	Kind        string `json:"kind"`
}

// gitlabProject represents a GitLab project (repository).
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

// gitlabNamespace represents a GitLab namespace.
type gitlabNamespace struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Path     string `json:"path"`
	Kind     string `json:"kind"`
	FullPath string `json:"full_path"`
}

func (n *gitlabNamespace) UnmarshalJSON(data []byte) error {
	// GitLab webhooks are inconsistent across event types and versions.
	// In particular, System Hooks may encode project.namespace as a string (e.g. "group")
	// while other payloads encode it as an object.
	if len(data) == 0 || string(data) == "null" {
		*n = gitlabNamespace{}
		return nil
	}

	switch data[0] {
	case '"':
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		*n = gitlabNamespace{Name: s, Path: s, FullPath: s}
		return nil
	case '{':
		type alias gitlabNamespace
		var a alias
		if err := json.Unmarshal(data, &a); err != nil {
			return err
		}
		*n = gitlabNamespace(a)
		return nil
	default:
		// Some payloads may provide only a numeric namespace id.
		var id int
		if err := json.Unmarshal(data, &id); err != nil {
			return err
		}
		*n = gitlabNamespace{ID: id}
		return nil
	}
}

// ListOrganizations returns accessible groups.
func (c *GitLabClient) ListOrganizations(ctx context.Context) ([]*Organization, error) {
	var orgs []*Organization
	page := 1
	perPage := 100

	for {
		endpoint := fmt.Sprintf("/groups?per_page=%d&page=%d&order_by=name", perPage, page)
		req, err := c.NewRequest(ctx, "GET", endpoint, nil)
		if err != nil {
			return nil, err
		}

		var gitlabGroups []gitlabGroup
		if err := c.DoRequest(req, &gitlabGroups); err != nil {
			return nil, err
		}

		if len(gitlabGroups) == 0 {
			break
		}

		for _, gGroup := range gitlabGroups {
			org := &Organization{
				ID: strconv.Itoa(gGroup.ID),
				// CRITICAL: GitLab API only accepts numeric IDs for group parameters.
				// Do NOT use gGroup.Path, gGroup.FullPath, or gGroup.Name here.
				// Using names/paths will cause API errors when ListRepositories calls getGroupProjects.
				Name:        strconv.Itoa(gGroup.ID),
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

// ListRepositories returns repositories for specified groups.
func (c *GitLabClient) ListRepositories(ctx context.Context, groups []string) ([]*Repository, error) {
	if len(groups) == 0 {
		return nil, nil
	}

	results := runOrdered(groups, 4, func(group string) ([]*Repository, error) {
		return c.getGroupProjects(ctx, group)
	})

	var allRepos []*Repository
	for i, group := range groups {
		res := results[i]
		if res.Err != nil {
			return nil, errors.ForgeError("failed to get projects for GitLab group").
				WithCause(res.Err).
				WithContext("group", group).
				Build()
		}
		allRepos = append(allRepos, res.Value...)
	}

	return allRepos, nil
}

// getGroupProjects gets all projects for a group.
// CRITICAL: The group parameter MUST be a numeric group ID (e.g., "123").
// GitLab API does NOT accept group names, paths, or full paths - only numeric IDs work.
// This has been verified through testing - using names/paths causes 404 errors.
func (c *GitLabClient) getGroupProjects(ctx context.Context, group string) ([]*Repository, error) {
	var allRepos []*Repository
	page := 1
	perPage := 100

	for {
		// GitLab API: /groups/:id/projects where :id MUST be a numeric group ID.
		// URL-encoding is applied defensively but group parameter must already be numeric.
		endpoint := fmt.Sprintf("/groups/%s/projects?per_page=%d&page=%d&order_by=last_activity_at&include_subgroups=true",
			url.PathEscape(group), perPage, page)
		req, err := c.NewRequest(ctx, "GET", endpoint, nil)
		if err != nil {
			return nil, err
		}

		var gitlabProjects []gitlabProject
		if err := c.DoRequest(req, &gitlabProjects); err != nil {
			return nil, err
		}

		if len(gitlabProjects) == 0 {
			break
		}

		for i := range gitlabProjects {
			repo := c.convertGitLabProject(&gitlabProjects[i])
			allRepos = append(allRepos, repo)
		}

		if len(gitlabProjects) < perPage {
			break
		}
		page++
	}

	return allRepos, nil
}

// GetRepository gets detailed information about a specific repository.
func (c *GitLabClient) GetRepository(ctx context.Context, owner, repo string) (*Repository, error) {
	projectPath := fmt.Sprintf("%s/%s", owner, repo)
	endpoint := fmt.Sprintf("/projects/%s", url.PathEscape(projectPath))
	req, err := c.NewRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	var gitlabProj gitlabProject
	if err = c.DoRequest(req, &gitlabProj); err != nil {
		return nil, err
	}

	return c.convertGitLabProject(&gitlabProj), nil
}

// CheckDocumentation checks if repository has docs folder and .docignore.
func (c *GitLabClient) CheckDocumentation(ctx context.Context, repo *Repository) error {
	// Use project ID instead of path for GitLab API
	projectID := repo.ID

	// Use default branch, or fall back to common defaults if not set
	branch := repo.DefaultBranch
	if branch == "" {
		// Try common default branch names
		branch = defaultMainBranch
	}

	// Check for docs folder
	hasDocs, err := c.checkPathExists(ctx, projectID, "docs", branch)
	if err != nil {
		// If main doesn't work, try master as fallback
		if branch == defaultMainBranch && repo.DefaultBranch == "" {
			hasDocs, err = c.checkPathExists(ctx, projectID, "docs", "master")
			if err != nil {
				return errors.ForgeError("failed to check docs folder existence on GitLab").
					WithCause(err).
					WithContext("repo", repo.FullName).
					Build()
			}
		} else {
			return errors.ForgeError("failed to check docs folder existence on GitLab").
				WithCause(err).
				WithContext("repo", repo.FullName).
				Build()
		}
	}
	repo.HasDocs = hasDocs

	// Check for .docignore file
	// Only check if docs folder exists, otherwise skip
	if hasDocs {
		hasDocIgnore, err := c.checkPathExists(ctx, projectID, ".docignore", branch)
		if err != nil {
			return errors.ForgeError("failed to check .docignore existence on GitLab").
				WithCause(err).
				WithContext("repo", repo.FullName).
				Build()
		}
		repo.HasDocIgnore = hasDocIgnore
	} else {
		repo.HasDocIgnore = false
	}

	return nil
}

// checkPathExists checks if a path exists in the project
// projectID should be the numeric project ID (not the path).
func (c *GitLabClient) checkPathExists(ctx context.Context, projectID, filePath, branch string) (bool, error) {
	// For directories (like "docs"), we need to use the tree API to check if the directory exists
	// GitLab API: /projects/:id/repository/tree?path=:path&ref=:ref
	endpoint := fmt.Sprintf("/projects/%s/repository/tree?path=%s&ref=%s&per_page=1",
		url.PathEscape(projectID),
		url.PathEscape(filePath),
		url.PathEscape(branch))

	req, err := c.NewRequest(ctx, "GET", endpoint, nil)
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
		// Log the full error for debugging
		body, _ := io.ReadAll(resp.Body)
		return false, errors.ForgeError("unexpected status code from GitLab").
			WithContext("status", resp.Status).
			WithContext("code", resp.StatusCode).
			WithContext("response", string(body)).
			WithContext("endpoint", endpoint).
			Build()
	}

	// Check if we got any results (directory exists and has content)
	// An empty array means the directory exists but is empty
	// We'll accept both cases as "exists"
	return true, nil
}

// ValidateWebhook validates GitLab webhook signature.
func (c *GitLabClient) ValidateWebhook(_ []byte, signature string, secret string) bool {
	// GitLab sends X-Gitlab-Token header with the secret
	return signature == secret
}

// ParseWebhookEvent parses GitLab webhook payload.
func (c *GitLabClient) ParseWebhookEvent(payload []byte, eventType string) (*WebhookEvent, error) {
	eventType = strings.TrimSpace(eventType)

	if eventType == "System Hook" {
		return c.parseSystemHookEvent(payload, eventType)
	}

	switch eventType {
	case string(WebhookEventPush), "Push Hook":
		return c.parsePushEvent(payload)
	case "tag_push", "Tag Push Hook":
		return c.parseTagPushEvent(payload)
	case string(WebhookEventRepository), "Repository Update Hook":
		return c.parseRepositoryEvent(payload)
	default:
		// Some GitLab setups (notably System Hooks) send event information primarily in the JSON body.
		// As a safe fallback, try dispatching based on payload kind when the header type is not recognized.
		event, err := c.parseSystemHookEvent(payload, eventType)
		if err == nil {
			return event, nil
		}
		return nil, errors.ForgeError("unsupported event type from GitLab").
			WithContext("type", eventType).
			Build()
	}
}

type gitlabWebhookEnvelope struct {
	ObjectKind string `json:"object_kind"`
	EventName  string `json:"event_name"`
	EventType  string `json:"event_type"`
}

func (c *GitLabClient) parseSystemHookEvent(payload []byte, headerEventType string) (*WebhookEvent, error) {
	var env gitlabWebhookEnvelope
	if err := json.Unmarshal(payload, &env); err != nil {
		return nil, errors.ForgeError("failed to unmarshal GitLab webhook envelope").
			WithCause(err).
			WithContext("type", headerEventType).
			Build()
	}

	kind := strings.TrimSpace(env.ObjectKind)
	if kind == "" {
		kind = strings.TrimSpace(env.EventName)
	}
	if kind == "" {
		kind = strings.TrimSpace(env.EventType)
	}

	switch kind {
	case string(WebhookEventPush):
		return c.parsePushEvent(payload)
	case "tag_push":
		return c.parseTagPushEvent(payload)
	case string(WebhookEventRepository), "repository_update":
		return c.parseRepositoryEvent(payload)
	default:
		return nil, errors.ForgeError("unsupported event type from GitLab").
			WithContext("type", headerEventType).
			WithContext("object_kind", env.ObjectKind).
			WithContext("event_name", env.EventName).
			WithContext("event_type", env.EventType).
			Build()
	}
}

// gitlabPushEvent represents a GitLab push event.
type gitlabPushEvent struct {
	Ref        string           `json:"ref"`
	Project    gitlabProject    `json:"project"`
	Commits    []gitlabCommit   `json:"commits"`
	Repository gitlabRepository `json:"repository"`
}

// gitlabCommit represents a GitLab commit.
type gitlabCommit struct {
	ID        string       `json:"id"`
	Message   string       `json:"message"`
	Timestamp time.Time    `json:"timestamp"`
	Author    gitlabAuthor `json:"author"`
	Added     []string     `json:"added"`
	Modified  []string     `json:"modified"`
	Removed   []string     `json:"removed"`
}

// gitlabAuthor represents a GitLab commit author.
type gitlabAuthor struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// gitlabRepository represents a GitLab repository in webhook.
type gitlabRepository struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Homepage    string `json:"homepage"`
	GitHTTPURL  string `json:"git_http_url"`
	GitSSHURL   string `json:"git_ssh_url"`
	Visibility  string `json:"visibility_level"`
}

// parsePushEvent parses a GitLab push event.
func (c *GitLabClient) parsePushEvent(payload []byte) (*WebhookEvent, error) {
	var pushEvent gitlabPushEvent
	if err := json.Unmarshal(payload, &pushEvent); err != nil {
		return nil, errors.ForgeError("failed to unmarshal GitLab push event").
			WithCause(err).
			Build()
	}
	if pushEvent.Project.ID == 0 { // zero value detection via ID
		return nil, errors.ForgeError("missing project in GitLab push event").Build()
	}
	branch := strings.TrimPrefix(pushEvent.Ref, "refs/heads/")
	commits := make([]WebhookCommit, 0, len(pushEvent.Commits))
	for i := range pushEvent.Commits {
		commit := &pushEvent.Commits[i]
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

// parseTagPushEvent parses a GitLab tag push event.
func (c *GitLabClient) parseTagPushEvent(payload []byte) (*WebhookEvent, error) {
	var pushEvent gitlabPushEvent
	if err := json.Unmarshal(payload, &pushEvent); err != nil {
		return nil, errors.ForgeError("failed to unmarshal GitLab tag push event").
			WithCause(err).
			Build()
	}
	if pushEvent.Project.ID == 0 {
		return nil, errors.ForgeError("missing project in GitLab tag push event").Build()
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

// parseRepositoryEvent parses a GitLab repository event.
func (c *GitLabClient) parseRepositoryEvent(payload []byte) (*WebhookEvent, error) {
	var repoEvent map[string]any
	if err := json.Unmarshal(payload, &repoEvent); err != nil {
		return nil, errors.ForgeError("failed to unmarshal GitLab repository event").
			WithCause(err).
			Build()
	}

	event := &WebhookEvent{Type: WebhookEventRepository, Timestamp: time.Now(), Changes: make(map[string]string), Metadata: make(map[string]string)}
	if project, ok := repoEvent["project"].(map[string]any); ok {
		if projectData, err := json.Marshal(project); err == nil {
			var gProj gitlabProject
			if err := json.Unmarshal(projectData, &gProj); err == nil {
				event.Repository = c.convertGitLabProject(&gProj)
			}
		}
	} else {
		return nil, errors.ForgeError("missing project in GitLab repository event").Build()
	}
	return event, nil
}

// RegisterWebhook registers a webhook for a project.
func (c *GitLabClient) RegisterWebhook(ctx context.Context, repo *Repository, webhookURL string) error {
	if c.config.Webhook == nil {
		return errors.ForgeError("webhook not configured for GitLab forge").
			WithContext("name", c.config.Name).
			Build()
	}

	endpoint := fmt.Sprintf("/projects/%s/hooks", url.PathEscape(repo.FullName))

	events := c.config.Webhook.Events
	if len(events) == 0 {
		events = []string{"push_events", "repository_update_events"}
	}

	payload := map[string]any{
		"url":   webhookURL,
		"token": c.config.Webhook.Secret,
	}

	// Set event flags
	for _, event := range events {
		switch event {
		case string(WebhookEventPush), "push_events":
			payload["push_events"] = true
		case string(WebhookEventRepository), "repository_update_events":
			payload["repository_update_events"] = true
		}
	}

	req, err := c.NewRequest(ctx, "POST", endpoint, payload)
	if err != nil {
		return err
	}

	var result map[string]any
	return c.DoRequest(req, &result)
}

// GetEditURL returns the GitLab edit URL for a file.
func (c *GitLabClient) GetEditURL(repo *Repository, filePath string, branch string) string {
	return GenerateEditURL(TypeGitLab, c.baseURL, repo.FullName, branch, filePath)
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
