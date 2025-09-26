package forge

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"
)

// ForgejoClient implements ForgeClient for Forgejo (Gitea-compatible API)
type ForgejoClient struct {
	config     *ForgeConfig
	httpClient *http.Client
	baseURL    string
	apiURL     string
	token      string
}

// NewForgejoClient creates a new Forgejo client
func NewForgejoClient(config *ForgeConfig) (*ForgejoClient, error) {
	if config.Type != string(ForgeTypeForgejo) {
		return nil, fmt.Errorf("invalid forge type for Forgejo client: %s", config.Type)
	}

	client := &ForgejoClient{
		config:     config,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		apiURL:     config.APIURL,
		baseURL:    config.BaseURL,
	}

	// Extract token from auth config
	if config.Auth != nil && config.Auth.Type == "token" {
		client.token = config.Auth.Token
	} else {
		return nil, fmt.Errorf("Forgejo client requires token authentication")
	}

	return client, nil
}

// GetType returns the forge type
func (c *ForgejoClient) GetType() ForgeType {
	return ForgeTypeForgejo
}

// GetName returns the configured name
func (c *ForgejoClient) GetName() string {
	return c.config.Name
}

// forgejoOrg represents a Forgejo organization
type forgejoOrg struct {
	ID          int    `json:"id"`
	Username    string `json:"username"`
	FullName    string `json:"full_name"`
	Description string `json:"description"`
	Website     string `json:"website"`
}

// forgejoRepo represents a Forgejo repository
type forgejoRepo struct {
	ID            int         `json:"id"`
	Name          string      `json:"name"`
	FullName      string      `json:"full_name"`
	Description   string      `json:"description"`
	Private       bool        `json:"private"`
	Fork          bool        `json:"fork"`
	CloneURL      string      `json:"clone_url"`
	SSHURL        string      `json:"ssh_url"`
	DefaultBranch string      `json:"default_branch"`
	Language      string      `json:"language"`
	Archived      bool        `json:"archived"`
	UpdatedAt     time.Time   `json:"updated_at"`
	Topics        []string    `json:"topics"`
	Owner         forgejoUser `json:"owner"`
}

// forgejoUser represents a Forgejo user
type forgejoUser struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	FullName string `json:"full_name"`
	Email    string `json:"email"`
}

// ListOrganizations returns accessible organizations
func (c *ForgejoClient) ListOrganizations(ctx context.Context) ([]*Organization, error) {
	var orgs []*Organization
	page := 1
	limit := 50

	for {
		endpoint := fmt.Sprintf("/user/orgs?page=%d&limit=%d", page, limit)
		req, err := c.newRequest(ctx, "GET", endpoint, nil)
		if err != nil {
			return nil, err
		}

		var forgejoOrgs []forgejoOrg
		if err := c.doRequest(req, &forgejoOrgs); err != nil {
			return nil, err
		}

		if len(forgejoOrgs) == 0 {
			break
		}

		for _, fOrg := range forgejoOrgs {
			org := &Organization{
				ID:          strconv.Itoa(fOrg.ID),
				Name:        fOrg.Username,
				DisplayName: fOrg.FullName,
				Description: fOrg.Description,
				Type:        "organization",
				Metadata: map[string]string{
					"forgejo_id": strconv.Itoa(fOrg.ID),
					"website":    fOrg.Website,
				},
			}
			orgs = append(orgs, org)
		}

		if len(forgejoOrgs) < limit {
			break
		}
		page++
	}

	return orgs, nil
}

// ListRepositories returns repositories for specified organizations
func (c *ForgejoClient) ListRepositories(ctx context.Context, organizations []string) ([]*Repository, error) {
	repoMap := make(map[string]*Repository)

	userRepos, err := c.listUserRepositories(ctx)
	if err != nil {
		slog.Warn("Forgejo: failed to list user repositories", "forge", c.GetName(), "error", err)
	} else {
		for _, r := range userRepos {
			repoMap[r.FullName] = r
		}
	}

	for _, org := range organizations {
		repos, oerr := c.getOrgRepositories(ctx, org)
		if oerr != nil {
			slog.Warn("Forgejo: skipping organization due to error", "forge", c.GetName(), "organization", org, "error", oerr)
			continue
		}
		for _, r := range repos {
			repoMap[r.FullName] = r
		}
	}

	var allRepos []*Repository
	for _, r := range repoMap {
		allRepos = append(allRepos, r)
	}
	return allRepos, nil
}

func (c *ForgejoClient) listUserRepositories(ctx context.Context) ([]*Repository, error) {
	var all []*Repository
	page := 1
	limit := 50
	for {
		endpoint := fmt.Sprintf("/user/repos?page=%d&limit=%d", page, limit)
		req, err := c.newRequest(ctx, "GET", endpoint, nil)
		if err != nil {
			return nil, err
		}
		var forgejoRepos []forgejoRepo
		if err := c.doRequest(req, &forgejoRepos); err != nil {
			return nil, err
		}
		if len(forgejoRepos) == 0 {
			break
		}
		for _, fRepo := range forgejoRepos {
			all = append(all, c.convertForgejoRepo(&fRepo))
		}
		if len(forgejoRepos) < limit {
			break
		}
		page++
	}
	return all, nil
}

// getOrgRepositories gets all repositories for an organization
func (c *ForgejoClient) getOrgRepositories(ctx context.Context, org string) ([]*Repository, error) {
	var allRepos []*Repository
	page := 1
	limit := 50

	for {
		endpoint := fmt.Sprintf("/orgs/%s/repos?page=%d&limit=%d", org, page, limit)
		req, err := c.newRequest(ctx, "GET", endpoint, nil)
		if err != nil {
			return nil, err
		}

		var forgejoRepos []forgejoRepo
		if err := c.doRequest(req, &forgejoRepos); err != nil {
			return nil, err
		}

		if len(forgejoRepos) == 0 {
			break
		}

		for _, fRepo := range forgejoRepos {
			repo := c.convertForgejoRepo(&fRepo)
			allRepos = append(allRepos, repo)
		}

		if len(forgejoRepos) < limit {
			break
		}
		page++
	}

	return allRepos, nil
}

// GetRepository gets detailed information about a specific repository
func (c *ForgejoClient) GetRepository(ctx context.Context, owner, repo string) (*Repository, error) {
	endpoint := fmt.Sprintf("/repos/%s/%s", owner, repo)
	req, err := c.newRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	var forgejoRepo forgejoRepo
	if err := c.doRequest(req, &forgejoRepo); err != nil {
		return nil, err
	}

	return c.convertForgejoRepo(&forgejoRepo), nil
}

// CheckDocumentation checks if repository has docs folder and .docignore
func (c *ForgejoClient) CheckDocumentation(ctx context.Context, repo *Repository) error {
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

// checkPathExists checks if a path exists in the repository
func (c *ForgejoClient) checkPathExists(ctx context.Context, owner, repo, path, branch string) (bool, error) {
	endpoint := fmt.Sprintf("/repos/%s/%s/contents/%s?ref=%s", owner, repo, path, branch)
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

// ValidateWebhook validates Forgejo webhook signature (Gitea-style HMAC-SHA1)
func (c *ForgejoClient) ValidateWebhook(payload []byte, signature string, secret string) bool {
	mac := hmac.New(sha1.New, []byte(secret))
	mac.Write(payload)
	expectedMAC := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(signature), []byte(expectedMAC))
}

// ParseWebhookEvent parses Forgejo webhook payload
func (c *ForgejoClient) ParseWebhookEvent(payload []byte, eventType string) (*WebhookEvent, error) {
	switch eventType {
	case "push":
		return c.parsePushEvent(payload)
	case "repository":
		return c.parseRepositoryEvent(payload)
	default:
		return nil, fmt.Errorf("unsupported event type: %s", eventType)
	}
}

// forgejoPushEvent represents a Forgejo push event
type forgejoPushEvent struct {
	Ref        string          `json:"ref"`
	Repository forgejoRepo     `json:"repository"`
	Commits    []forgejoCommit `json:"commits"`
	HeadCommit forgejoCommit   `json:"head_commit"`
	Pusher     forgejoUser     `json:"pusher"`
}

// forgejoCommit represents a Forgejo commit
type forgejoCommit struct {
	ID        string    `json:"id"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
	Author    struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	} `json:"author"`
	Committer struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	} `json:"committer"`
	Added    []string `json:"added"`
	Removed  []string `json:"removed"`
	Modified []string `json:"modified"`
}

// parsePushEvent parses a Forgejo push event
func (c *ForgejoClient) parsePushEvent(payload []byte) (*WebhookEvent, error) {
	var pushEvent forgejoPushEvent
	if err := json.Unmarshal(payload, &pushEvent); err != nil {
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
		Repository: c.convertForgejoRepo(&pushEvent.Repository),
		Branch:     branch,
		Commits:    commits,
		Timestamp:  time.Now(),
		Metadata: map[string]string{
			"ref":         pushEvent.Ref,
			"head_commit": pushEvent.HeadCommit.ID,
			"pusher":      pushEvent.Pusher.Username,
		},
	}, nil
}

// parseRepositoryEvent parses a Forgejo repository event
func (c *ForgejoClient) parseRepositoryEvent(payload []byte) (*WebhookEvent, error) {
	var repoEvent map[string]interface{}
	if err := json.Unmarshal(payload, &repoEvent); err != nil {
		return nil, err
	}

	event := &WebhookEvent{
		Type:      WebhookEventRepository,
		Timestamp: time.Now(),
		Changes:   make(map[string]string),
		Metadata:  make(map[string]string),
	}

	// Extract repository information
	if repository, ok := repoEvent["repository"].(map[string]interface{}); ok {
		if repoData, err := json.Marshal(repository); err == nil {
			var forgejoRepo forgejoRepo
			if err := json.Unmarshal(repoData, &forgejoRepo); err == nil {
				event.Repository = c.convertForgejoRepo(&forgejoRepo)
			}
		}
	}

	// Extract action if available
	if action, ok := repoEvent["action"].(string); ok {
		event.Action = action
		event.Metadata["action"] = action
	}

	return event, nil
}

// RegisterWebhook registers a webhook for a repository
func (c *ForgejoClient) RegisterWebhook(ctx context.Context, repo *Repository, webhookURL string) error {
	if c.config.Webhook == nil {
		return fmt.Errorf("webhook not configured for forge %s", c.config.Name)
	}

	owner, repoName := c.splitFullName(repo.FullName)
	endpoint := fmt.Sprintf("/repos/%s/%s/hooks", owner, repoName)

	config := map[string]interface{}{
		"url":          webhookURL,
		"content_type": "json",
		"secret":       c.config.Webhook.Secret,
	}

	events := c.config.Webhook.Events
	if len(events) == 0 {
		events = []string{"push", "repository"}
	}

	payload := map[string]interface{}{
		"type":   "forgejo",
		"config": config,
		"events": events,
		"active": true,
	}

	req, err := c.newRequest(ctx, "POST", endpoint, payload)
	if err != nil {
		return err
	}

	var result map[string]interface{}
	return c.doRequest(req, &result)
}

// GetEditURL returns the Forgejo edit URL for a file
func (c *ForgejoClient) GetEditURL(repo *Repository, filePath string, branch string) string {
	return fmt.Sprintf("%s/%s/_edit/%s/%s", c.baseURL, repo.FullName, branch, filePath)
}

// Helper methods

func (c *ForgejoClient) convertForgejoRepo(fRepo *forgejoRepo) *Repository {
	return &Repository{
		ID:            strconv.Itoa(fRepo.ID),
		Name:          fRepo.Name,
		FullName:      fRepo.FullName,
		CloneURL:      fRepo.CloneURL,
		SSHURL:        fRepo.SSHURL,
		DefaultBranch: fRepo.DefaultBranch,
		Description:   fRepo.Description,
		Private:       fRepo.Private,
		Archived:      fRepo.Archived,
		LastUpdated:   fRepo.UpdatedAt,
		Topics:        fRepo.Topics,
		Language:      fRepo.Language,
		Metadata: map[string]string{
			"forgejo_id": strconv.Itoa(fRepo.ID),
			"owner":      fRepo.Owner.Username,
			"owner_name": fRepo.Owner.FullName,
			"fork":       strconv.FormatBool(fRepo.Fork),
			"forge_name": c.GetName(),
		},
	}
}

func (c *ForgejoClient) splitFullName(fullName string) (owner, repo string) {
	parts := strings.SplitN(fullName, "/", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "", fullName
}

func (c *ForgejoClient) newRequest(ctx context.Context, method, endpoint string, body interface{}) (*http.Request, error) {
	// Normalize endpoint so we don't accidentally discard the /api/v1 prefix when using path.Join.
	// path.Join("/api/v1", "/user/orgs") -> "/user/orgs" (BUG) so we must trim the leading slash.
	cleanEndpoint := strings.TrimPrefix(endpoint, "/")

	// Split query string from path (we were previously passing query params inside endpoint and they were
	// treated as part of the path, producing %3F in final URL and 404s from Forgejo). Keep raw query separate.
	var rawQuery string
	if idx := strings.Index(cleanEndpoint, "?"); idx != -1 {
		rawQuery = cleanEndpoint[idx+1:]
		cleanEndpoint = cleanEndpoint[:idx]
	}

	u, err := url.Parse(c.apiURL)
	if err != nil {
		return nil, err
	}

	// Ensure apiURL path and endpoint are joined correctly preserving existing base path
	basePath := strings.TrimSuffix(u.Path, "/")
	u.Path = path.Join(basePath, cleanEndpoint)
	if rawQuery != "" {
		// Preserve original query ordering/content (no re-encoding beyond necessary)
		u.RawQuery = rawQuery
	}

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

	req.Header.Set("Authorization", "token "+c.token)
	req.Header.Set("User-Agent", "DocBuilder/1.0")

	return req, nil
}

func (c *ForgejoClient) doRequest(req *http.Request, result interface{}) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		// Read and truncate body for diagnostics (avoid large/error HTML pages flooding logs)
		limitedBody, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		// Best effort to make body single-line
		bodyStr := strings.ReplaceAll(string(limitedBody), "\n", " ")
		return fmt.Errorf("Forgejo API error: %s url=%s body=%q", resp.Status, req.URL.String(), bodyStr)
	}

	if result != nil {
		return json.NewDecoder(resp.Body).Decode(result)
	}

	return nil
}
