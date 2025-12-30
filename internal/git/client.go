// Package git provides a client for performing Git operations such as clone, update, and authentication handling
// for DocBuilder's documentation pipeline.
package git

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"

	appcfg "git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/logfields"
)

// Client handles Git operations for DocBuilder, including clone, update, and authentication.
type Client struct {
	workspaceDir    string
	buildCfg        *appcfg.BuildConfig // optional build config for strategy flags
	inRetry         bool                // internal guard to avoid nested retry wrapping
	remoteHeadCache *RemoteHeadCache    // cache for remote HEAD refs to skip unnecessary fetches
}

// CloneResult contains the result of a clone or update operation.
type CloneResult struct {
	Path       string    // local filesystem path
	CommitSHA  string    // HEAD commit SHA
	CommitDate time.Time // HEAD commit date
}

// NewClient creates a new Git client with the specified workspace directory.
func NewClient(workspaceDir string) *Client { return &Client{workspaceDir: workspaceDir} }

// WithBuildConfig attaches a build configuration to the client for strategy flags (fluent helper).
func (c *Client) WithBuildConfig(cfg *appcfg.BuildConfig) *Client { c.buildCfg = cfg; return c }

// WithRemoteHeadCache attaches a remote HEAD cache to skip fetches for unchanged repositories.
func (c *Client) WithRemoteHeadCache(cache *RemoteHeadCache) *Client {
	c.remoteHeadCache = cache
	return c
}

// CloneRepo clones a repository to the workspace directory.
// If retry is enabled, it wraps the operation with retry logic.
// Returns the local filesystem path and any error.
//
// Deprecated: Use CloneRepoWithMetadata for commit metadata.
func (c *Client) CloneRepo(repo appcfg.Repository) (string, error) {
	result, err := c.CloneRepoWithMetadata(repo)
	if err != nil {
		return "", err
	}
	return result.Path, nil
}

// CloneRepoWithMetadata clones a repository and returns metadata including commit date.
// If retry is enabled, it wraps the operation with retry logic.
func (c *Client) CloneRepoWithMetadata(repo appcfg.Repository) (CloneResult, error) {
	if c.inRetry {
		return c.cloneOnceWithMetadata(repo)
	}
	return c.withRetryMetadata("clone", repo.Name, func() (CloneResult, error) {
		return c.cloneOnceWithMetadata(repo)
	})
}

func (c *Client) cloneOnce(repo appcfg.Repository) (string, error) {
	repoPath := filepath.Join(c.workspaceDir, repo.Name)
	slog.Debug("Cloning repository", logfields.URL(repo.URL), logfields.Name(repo.Name), slog.String("branch", repo.Branch), logfields.Path(repoPath))
	if err := os.RemoveAll(repoPath); err != nil {
		return "", fmt.Errorf("failed to remove existing directory: %w", err)
	}

	cloneOptions := &git.CloneOptions{URL: repo.URL}
	if repo.Branch != "" {
		if repo.IsTag {
			cloneOptions.ReferenceName = plumbing.ReferenceName("refs/tags/" + repo.Branch)
			slog.Debug("Cloning tag reference", logfields.Name(repo.Name), slog.String("tag", repo.Branch), slog.String("ref", string(cloneOptions.ReferenceName)))
		} else {
			cloneOptions.ReferenceName = plumbing.ReferenceName("refs/heads/" + repo.Branch)
			slog.Debug("Cloning branch reference", logfields.Name(repo.Name), slog.String("branch", repo.Branch), slog.String("ref", string(cloneOptions.ReferenceName)))
		}
		cloneOptions.SingleBranch = true
	}
	if c.buildCfg != nil && c.buildCfg.ShallowDepth > 0 {
		cloneOptions.Depth = c.buildCfg.ShallowDepth
	}
	if repo.Auth != nil {
		auth, err := c.getAuth(repo.Auth)
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

func (c *Client) cloneOnceWithMetadata(repo appcfg.Repository) (CloneResult, error) {
	repoPath := filepath.Join(c.workspaceDir, repo.Name)
	slog.Debug("Cloning repository", logfields.URL(repo.URL), logfields.Name(repo.Name), slog.String("branch", repo.Branch), logfields.Path(repoPath))
	if err := os.RemoveAll(repoPath); err != nil {
		return CloneResult{}, fmt.Errorf("failed to remove existing directory: %w", err)
	}

	cloneOptions := &git.CloneOptions{URL: repo.URL}
	if repo.Branch != "" {
		if repo.IsTag {
			cloneOptions.ReferenceName = plumbing.ReferenceName("refs/tags/" + repo.Branch)
			slog.Debug("Cloning tag reference", logfields.Name(repo.Name), slog.String("tag", repo.Branch), slog.String("ref", string(cloneOptions.ReferenceName)))
		} else {
			cloneOptions.ReferenceName = plumbing.ReferenceName("refs/heads/" + repo.Branch)
			slog.Debug("Cloning branch reference", logfields.Name(repo.Name), slog.String("branch", repo.Branch), slog.String("ref", string(cloneOptions.ReferenceName)))
		}
		cloneOptions.SingleBranch = true
	}
	if c.buildCfg != nil && c.buildCfg.ShallowDepth > 0 {
		cloneOptions.Depth = c.buildCfg.ShallowDepth
	}
	if repo.Auth != nil {
		auth, err := c.getAuth(repo.Auth)
		if err != nil {
			return CloneResult{}, fmt.Errorf("failed to setup authentication: %w", err)
		}
		cloneOptions.Auth = auth
	}
	repository, err := git.PlainClone(repoPath, false, cloneOptions)
	if err != nil {
		return CloneResult{}, classifyCloneError(repo.URL, err)
	}

	// Get commit metadata
	result := CloneResult{Path: repoPath}
	if ref, herr := repository.Head(); herr == nil {
		result.CommitSHA = ref.Hash().String()

		// Get commit object to extract date
		if commit, cerr := repository.CommitObject(ref.Hash()); cerr == nil {
			result.CommitDate = commit.Author.When
			slog.Info("Repository cloned successfully",
				logfields.Name(repo.Name),
				logfields.URL(repo.URL),
				slog.String("commit", result.CommitSHA[:8]),
				slog.Time("commit_date", result.CommitDate),
				logfields.Path(repoPath))
		} else {
			slog.Info("Repository cloned successfully (commit metadata unavailable)",
				logfields.Name(repo.Name),
				logfields.URL(repo.URL),
				slog.String("commit", result.CommitSHA[:8]),
				logfields.Path(repoPath))
		}
	} else {
		slog.Info("Repository cloned successfully", logfields.Name(repo.Name), logfields.URL(repo.URL), logfields.Path(repoPath))
	}

	if c.buildCfg != nil && c.buildCfg.PruneNonDocPaths {
		if err := c.pruneNonDocTopLevel(repoPath, repo); err != nil {
			slog.Warn("prune non-doc paths failed", logfields.Name(repo.Name), slog.String("error", err.Error()))
		}
	}
	return result, nil
}

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

// UpdateRepo updates an existing repository or clones it if missing.
// If retry is enabled, it wraps the operation with retry logic.
func (c *Client) UpdateRepo(repo appcfg.Repository) (string, error) {
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
