package forge

import (
	"context"
	"os"
	"path/filepath"

	cfg "git.home.luguber.info/inful/docbuilder/internal/config"
)

// LocalClient is a minimal forge client that uses the current working directory
// as a single local repository source. It is useful for development and
// environments where documentation is sourced from the current repo without
// external forge API access.
type LocalClient struct {
	name string
}

func NewLocalClient(config *Config) (Client, error) {
	lc := &LocalClient{name: config.Name}
	return lc, nil
}

func (c *LocalClient) GetType() Type   { return cfg.ForgeLocal }
func (c *LocalClient) GetName() string { return c.name }

func (c *LocalClient) ListOrganizations(ctx context.Context) ([]*Organization, error) {
	// Local client does not have organizations; return a single pseudo-org based on cwd.
	cwd, _ := os.Getwd()
	return []*Organization{{
		ID:          "local",
		Name:        filepath.Base(cwd),
		DisplayName: filepath.Base(cwd),
		Type:        "local",
	}}, nil
}

func (c *LocalClient) ListRepositories(ctx context.Context, _ []string) ([]*Repository, error) {
	cwd, _ := os.Getwd()
	name := filepath.Base(cwd)
	return []*Repository{{
		ID:            "local:" + name,
		Name:          name,
		FullName:      "local/" + name,
		CloneURL:      cwd,
		DefaultBranch: "main",
		Description:   "Local repository (current working directory)",
		Private:       false,
		Archived:      false,
	}}, nil
}

func (c *LocalClient) GetRepository(ctx context.Context, owner, repo string) (*Repository, error) {
	cwd, _ := os.Getwd()
	return &Repository{
		ID:            "local:" + repo,
		Name:          repo,
		FullName:      owner + "/" + repo,
		CloneURL:      cwd,
		DefaultBranch: "main",
	}, nil
}

func (c *LocalClient) CheckDocumentation(ctx context.Context, repo *Repository) error {
	// No-op; discovery will inspect paths during DocBuilder discovery phase.
	return nil
}

func (c *LocalClient) ValidateWebhook(payload []byte, signature string, secret string) bool {
	return true
}

func (c *LocalClient) ParseWebhookEvent(payload []byte, event string) (*WebhookEvent, error) {
	return nil, nil
}

func (c *LocalClient) RegisterWebhook(ctx context.Context, repo *Repository, webhookURL string) error {
	return nil
}
func (c *LocalClient) GetEditURL(repo *Repository, filePath string, branch string) string { return "" }
