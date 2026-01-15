package forge

import (
	"context"
	"crypto/hmac"
	"crypto/sha1" // #nosec G505 -- SHA-1 needed for legacy Forgejo/Gitea webhook compatibility
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	cfg "git.home.luguber.info/inful/docbuilder/internal/config"
)

// ForgejoClient implements ForgeClient for Forgejo (Gitea-compatible API).
type ForgejoClient struct {
	*BaseForge
	config  *Config
	baseURL string
}

// NewForgejoClient creates a new Forgejo client.
func NewForgejoClient(fg *Config) (*ForgejoClient, error) {
	if fg.Type != cfg.ForgeForgejo {
		return nil, fmt.Errorf("invalid forge type for Forgejo client: %s", fg.Type)
	}

	// Extract token from auth config
	tok, err := tokenFromConfig(fg, "Forgejo")
	if err != nil {
		return nil, err
	}

	// Create BaseForge with common HTTP operations
	baseForge := NewBaseForge(newHTTPClient30s(), fg.APIURL, tok)

	// Forgejo uses "token " auth prefix instead of "Bearer "
	baseForge.SetAuthHeaderPrefix("token ")

	return &ForgejoClient{
		BaseForge: baseForge,
		config:    fg,
		baseURL:   fg.BaseURL,
	}, nil
}

// GetType returns the forge type.
func (c *ForgejoClient) GetType() cfg.ForgeType { return cfg.ForgeForgejo }

// GetName returns the configured name.
func (c *ForgejoClient) GetName() string {
	if c == nil || c.config == nil {
		return ""
	}
	return c.config.Name
}

// forgejoOrg represents a Forgejo organization.
type forgejoOrg struct {
	ID          int    `json:"id"`
	Username    string `json:"username"`
	FullName    string `json:"full_name"`
	Description string `json:"description"`
	Website     string `json:"website"`
}

// forgejoRepo represents a Forgejo repository.
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

// forgejoUser represents a Forgejo user.
type forgejoUser struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	FullName string `json:"full_name"`
	Email    string `json:"email"`
}

// ListOrganizations returns accessible organizations.
func (c *ForgejoClient) ListOrganizations(ctx context.Context) ([]*Organization, error) {
	var orgs []*Organization
	page := 1
	limit := 50

	for {
		endpoint := fmt.Sprintf("/user/orgs?page=%d&limit=%d", page, limit)
		req, err := c.NewRequest(ctx, "GET", endpoint, nil)
		if err != nil {
			return nil, err
		}

		var forgejoOrgs []forgejoOrg
		if err := c.DoRequest(req, &forgejoOrgs); err != nil {
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

// ListRepositories returns repositories for specified organizations.
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

	// Organization listing is often the slowest part. Run those in parallel,
	// but preserve existing behavior: org failures are logged and skipped.
	results := runOrdered(organizations, 4, func(org string) ([]*Repository, error) {
		return c.getOrgRepositories(ctx, org)
	})

	for i, org := range organizations {
		res := results[i]
		if res.Err != nil {
			slog.Warn("Forgejo: skipping organization due to error", "forge", c.GetName(), "organization", org, "error", res.Err)
			continue
		}
		for _, r := range res.Value {
			repoMap[r.FullName] = r
		}
	}

	allRepos := make([]*Repository, 0, len(repoMap))
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
		req, err := c.NewRequest(ctx, "GET", endpoint, nil)
		if err != nil {
			return nil, err
		}
		var forgejoRepos []forgejoRepo
		if err := c.DoRequest(req, &forgejoRepos); err != nil {
			return nil, err
		}
		if len(forgejoRepos) == 0 {
			break
		}
		for i := range forgejoRepos {
			all = append(all, c.convertForgejoRepo(&forgejoRepos[i]))
		}
		if len(forgejoRepos) < limit {
			break
		}
		page++
	}
	return all, nil
}

// getOrgRepositories gets all repositories for an organization.
func (c *ForgejoClient) getOrgRepositories(ctx context.Context, org string) ([]*Repository, error) {
	baseEndpoint := fmt.Sprintf("/orgs/%s/repos", org)
	return c.fetchAndConvertRepos(ctx, baseEndpoint, 50)
}

// fetchAndConvertRepos is a helper that fetches paginated repositories and converts them.
func (c *ForgejoClient) fetchAndConvertRepos(ctx context.Context, endpoint string, pageSize int) ([]*Repository, error) {
	return fetchAndConvertReposGeneric(
		c.BaseForge,
		ctx,
		endpoint,
		"page",
		"limit",
		pageSize,
		c.convertForgejoRepo,
	)
}

// GetRepository gets detailed information about a specific repository.
func (c *ForgejoClient) GetRepository(ctx context.Context, owner, repo string) (*Repository, error) {
	endpoint := fmt.Sprintf("/repos/%s/%s", owner, repo)
	req, err := c.NewRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	var forgejoRepository forgejoRepo
	if err = c.DoRequest(req, &forgejoRepository); err != nil {
		return nil, err
	}

	return c.convertForgejoRepo(&forgejoRepository), nil
}

// CheckDocumentation checks if repository has docs folder and .docignore.
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

// checkPathExists checks if a path exists in the repository.
func (c *ForgejoClient) checkPathExists(ctx context.Context, owner, repo, path, branch string) (bool, error) {
	endpoint := fmt.Sprintf("/repos/%s/%s/contents/%s?ref=%s", owner, repo, path, branch)
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
		return false, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return true, nil
}

// ValidateWebhook validates Forgejo webhook signature (Gitea-style HMAC-SHA1).
func (c *ForgejoClient) ValidateWebhook(payload []byte, signature string, secret string) bool {
	if signature == "" || secret == "" {
		return false
	}
	// Preferred GitHub-compatible sha256=<hash>
	if strings.HasPrefix(signature, "sha256=") {
		expected := signature[len("sha256="):]
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(payload)
		calc := hex.EncodeToString(mac.Sum(nil))
		return hmac.Equal([]byte(expected), []byte(calc))
	}
	// Legacy raw SHA1 (some older Forgejo/Gitea setups)
	if strings.HasPrefix(signature, "sha1=") {
		expected := signature[len("sha1="):]
		mac := hmac.New(sha1.New, []byte(secret))
		mac.Write(payload)
		calc := hex.EncodeToString(mac.Sum(nil))
		return hmac.Equal([]byte(expected), []byte(calc))
	}
	// Bare SHA1 hash fallback (no prefix)
	mac := hmac.New(sha1.New, []byte(secret))
	mac.Write(payload)
	calc := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(signature), []byte(calc))
}

// ParseWebhookEvent parses Forgejo webhook payload.
func (c *ForgejoClient) ParseWebhookEvent(payload []byte, eventType string) (*WebhookEvent, error) {
	switch eventType {
	case string(WebhookEventPush):
		return c.parsePushEvent(payload)
	case string(WebhookEventRepository):
		return c.parseRepositoryEvent(payload)
	default:
		return nil, fmt.Errorf("unsupported event type: %s", eventType)
	}
}

// forgejoPushEvent represents a Forgejo push event.
type forgejoPushEvent struct {
	Ref        string          `json:"ref"`
	Repository json.RawMessage `json:"repository"`
	Commits    []forgejoCommit `json:"commits"`
	HeadCommit forgejoCommit   `json:"head_commit"`
	Pusher     forgejoUser     `json:"pusher"`
}

// forgejoCommit represents a Forgejo commit.
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

// parsePushEvent parses a Forgejo push event.
func (c *ForgejoClient) parsePushEvent(payload []byte) (*WebhookEvent, error) {
	var pushEvent forgejoPushEvent
	if err := json.Unmarshal(payload, &pushEvent); err != nil {
		return nil, err
	}

	if len(pushEvent.Repository) == 0 {
		return nil, errors.New("missing repository in push event")
	}

	var repoMap map[string]any
	if err := json.Unmarshal(pushEvent.Repository, &repoMap); err != nil {
		return nil, err
	}
	if rawID, ok := repoMap["id"].(string); ok {
		if intID, convErr := strconv.Atoi(rawID); convErr == nil {
			repoMap["id"] = intID
		}
	}
	repoBytes, marshalErr := json.Marshal(repoMap)
	if marshalErr != nil {
		return nil, marshalErr
	}
	var repo forgejoRepo
	if err := json.Unmarshal(repoBytes, &repo); err != nil {
		return nil, err
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
		Repository: c.convertForgejoRepo(&repo),
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

// parseRepositoryEvent parses a Forgejo repository event.
func (c *ForgejoClient) parseRepositoryEvent(payload []byte) (*WebhookEvent, error) {
	var repoEvent map[string]any
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
	if repository, ok := repoEvent["repository"].(map[string]any); ok {
		if repoData, err := json.Marshal(repository); err == nil {
			var forgejoRepository forgejoRepo
			if unmarshalErr := json.Unmarshal(repoData, &forgejoRepository); unmarshalErr == nil {
				event.Repository = c.convertForgejoRepo(&forgejoRepository)
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

// RegisterWebhook registers a webhook for a repository.
func (c *ForgejoClient) RegisterWebhook(ctx context.Context, repo *Repository, webhookURL string) error {
	if c.config.Webhook == nil {
		return fmt.Errorf("webhook not configured for forge %s", c.config.Name)
	}

	owner, repoName := c.splitFullName(repo.FullName)
	endpoint := fmt.Sprintf("/repos/%s/%s/hooks", owner, repoName)

	config := map[string]any{
		"url":          webhookURL,
		"content_type": "json",
		"secret":       c.config.Webhook.Secret,
	}

	events := c.config.Webhook.Events
	if len(events) == 0 {
		events = []string{"push", "repository"}
	}

	payload := map[string]any{
		"type":   "forgejo",
		"config": config,
		"events": events,
		"active": true,
	}

	req, err := c.NewRequest(ctx, "POST", endpoint, payload)
	if err != nil {
		return err
	}

	var result map[string]any
	return c.DoRequest(req, &result)
}

// GetEditURL returns the URL to edit a file in Forgejo.
func (c *ForgejoClient) GetEditURL(repo *Repository, filePath, branch string) string {
	return GenerateEditURL(TypeForgejo, c.baseURL, repo.FullName, branch, filePath)
}

// Helper methods

func (c *ForgejoClient) convertForgejoRepo(fRepo *forgejoRepo) *Repository {
	forgeName := ""
	if c != nil && c.config != nil {
		forgeName = c.GetName()
	}
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
			"forge_name": forgeName,
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
