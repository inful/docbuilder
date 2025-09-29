package git

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	appcfg "git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/logfields"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

// Client handles Git operations
type Client struct {
	workspaceDir string
	buildCfg     *appcfg.BuildConfig // optional build config for strategy flags
	inRetry      bool                // internal guard to avoid nested retry wrapping
}

// NewClient creates a new Git client with the specified workspace directory
func NewClient(workspaceDir string) *Client { return &Client{workspaceDir: workspaceDir} }

// WithBuildConfig attaches build configuration to the client (fluent helper).
func (c *Client) WithBuildConfig(cfg *appcfg.BuildConfig) *Client { c.buildCfg = cfg; return c }

// CloneRepository clones a repository to the workspace (with retry wrapper if enabled).
func (c *Client) CloneRepository(repo appcfg.Repository) (string, error) {
	if c.inRetry {
		return c.cloneOnce(repo)
	}
	return c.withRetry("clone", repo.Name, func() (string, error) { return c.cloneOnce(repo) })
}

func (c *Client) cloneOnce(repo appcfg.Repository) (string, error) {
	repoPath := filepath.Join(c.workspaceDir, repo.Name)
	slog.Debug("Cloning repository", logfields.URL(repo.URL), logfields.Name(repo.Name), slog.String("branch", repo.Branch), logfields.Path(repoPath))
	if err := os.RemoveAll(repoPath); err != nil {
		return "", fmt.Errorf("failed to remove existing directory: %w", err)
	}

	cloneOptions := &git.CloneOptions{URL: repo.URL, Progress: os.Stdout}
	if repo.Branch != "" {
		cloneOptions.ReferenceName = plumbing.ReferenceName("refs/heads/" + repo.Branch)
		cloneOptions.SingleBranch = true
	}
	if c.buildCfg != nil && c.buildCfg.ShallowDepth > 0 {
		cloneOptions.Depth = c.buildCfg.ShallowDepth
	}
	if repo.Auth != nil {
		auth, err := c.getAuthentication(repo.Auth)
		if err != nil {
			return "", fmt.Errorf("failed to setup authentication: %w", err)
		}
		cloneOptions.Auth = auth
	}
	repository, err := git.PlainClone(repoPath, false, cloneOptions)
	if err != nil {
		return "", classifyCloneError(repo.URL, err)
	}
	if ref, herr := repository.Head(); herr == nil {
		slog.Info("Repository cloned successfully", logfields.Name(repo.Name), logfields.URL(repo.URL), slog.String("commit", ref.Hash().String()[:8]), logfields.Path(repoPath))
	} else {
		slog.Info("Repository cloned successfully", logfields.Name(repo.Name), logfields.URL(repo.URL), logfields.Path(repoPath))
	}
	if c.buildCfg != nil && c.buildCfg.PruneNonDocPaths {
		if err := c.pruneNonDocTopLevel(repoPath, repo); err != nil {
			slog.Warn("prune non-doc paths failed", logfields.Name(repo.Name), slog.String("error", err.Error()))
		}
	}
	return repoPath, nil
}

// classifyCloneError attempts to wrap underlying go-git errors into typed permanent failures.
func classifyCloneError(url string, err error) error {
	l := strings.ToLower(err.Error())
 	// Heuristic mapping (Phase 4 start). These types allow downstream classification without string parsing.
 	if strings.Contains(l, "authentication") || strings.Contains(l, "auth fail") || strings.Contains(l, "invalid username or password") {
 		return &AuthError{Op: "clone", URL: url, Err: err}
 	}
 	if strings.Contains(l, "not found") || strings.Contains(l, "repository does not exist") {
 		return &NotFoundError{Op: "clone", URL: url, Err: err}
 	}
 	if strings.Contains(l, "unsupported protocol") || strings.Contains(l, "protocol not supported") {
 		return &UnsupportedProtocolError{Op: "clone", URL: url, Err: err}
 	}
	if strings.Contains(l, "rate limit") || strings.Contains(l, "too many requests") {
		return &RateLimitError{Op: "clone", URL: url, Err: err}
	}
	if strings.Contains(l, "timeout") || strings.Contains(l, "i/o timeout") {
		return &NetworkTimeoutError{Op: "clone", URL: url, Err: err}
	}
 	return fmt.Errorf("failed to clone repository %s: %w", url, err)
}

// UpdateRepository updates an existing repository or clones if missing.
func (c *Client) UpdateRepository(repo appcfg.Repository) (string, error) {
	if c.inRetry {
		return c.updateOnce(repo)
	}
	return c.withRetry("update", repo.Name, func() (string, error) { return c.updateOnce(repo) })
}

func (c *Client) updateOnce(repo appcfg.Repository) (string, error) {
	repoPath := filepath.Join(c.workspaceDir, repo.Name)
	if _, err := os.Stat(filepath.Join(repoPath, ".git")); err != nil { // missing => clone
		slog.Debug("Repository missing, cloning", logfields.Name(repo.Name))
		return c.cloneOnce(repo)
	}
	return c.updateExistingRepo(repoPath, repo)
}
