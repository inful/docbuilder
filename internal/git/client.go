package git

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
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
func (c *Client) CloneRepo(ctx context.Context, repo appcfg.Repository) (string, error) {
	result, err := c.CloneRepoWithMetadata(ctx, repo)
	if err != nil {
		return "", err
	}
	return result.Path, nil
}

// UpdateRepo updates an existing repository in the workspace.
// If retry is enabled, it wraps the operation with retry logic.
func (c *Client) UpdateRepo(ctx context.Context, repo appcfg.Repository) (string, error) {
	return c.withRetry(ctx, "update", repo.Name, func() (string, error) {
		return c.updateOnce(ctx, repo)
	})
}

// CloneRepoWithMetadata clones a repository and returns metadata including commit date.
// If retry is enabled, it wraps the operation with retry logic.
func (c *Client) CloneRepoWithMetadata(ctx context.Context, repo appcfg.Repository) (CloneResult, error) {
	return c.withRetryMetadata(ctx, "clone", repo.Name, func() (CloneResult, error) {
		return c.cloneOnceWithMetadata(ctx, repo)
	})
}

func (c *Client) cloneOnce(ctx context.Context, repo appcfg.Repository) (string, error) {
	result, err := c.cloneOnceWithMetadata(ctx, repo)
	if err != nil {
		return "", err
	}
	return result.Path, nil
}

func (c *Client) cloneOnceWithMetadata(ctx context.Context, repo appcfg.Repository) (CloneResult, error) {
	if repo.Name == "" {
		// The path of the clone would be the root of the workspace, and
		// the removal of the old clone would delete every other clone.
		return CloneResult{}, GitError("the repository has no name").
			WithContext("url", repo.URL).
			Build()
	}

	repoPath := filepath.Join(c.workspaceDir, repo.Name)
	slog.Debug("Cloning repository", logfields.URL(repo.URL), logfields.Name(repo.Name), slog.String("branch", repo.Branch), logfields.Path(repoPath))
	// The clone goes into a directory of its own: the old copy of the
	// repository is removed only when the clone is there. A clone that
	// fails - the forge is unreachable, the token expired, the disk is
	// full - took the working copy with it, and the pages of the
	// repository were gone until a later clone succeeded.
	tempPath := repoPath + ".clone-in-progress"
	if err := os.RemoveAll(tempPath); err != nil {
		return CloneResult{}, GitError("failed to remove the directory of the clone").
			WithCause(err).
			WithContext("path", tempPath).
			Build()
	}
	defer func() {
		// The directory of the clone is gone when the clone failed.
		_ = os.RemoveAll(tempPath)
	}()

	cloneOptions := &git.CloneOptions{URL: repo.URL, Tags: git.NoTags}
	// Default to cloning only the remote HEAD branch.
	if repo.IsTag {
		if repo.Branch != "" {
			cloneOptions.ReferenceName = plumbing.ReferenceName("refs/tags/" + repo.Branch)
			slog.Debug("Cloning tag reference", logfields.Name(repo.Name), slog.String("tag", repo.Branch), slog.String("ref", string(cloneOptions.ReferenceName)))
		}
		cloneOptions.SingleBranch = true
	} else {
		if repo.Branch != "" {
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
			return CloneResult{}, GitError("failed to setup authentication").
				WithCause(err).
				Build()
		}
		cloneOptions.Auth = auth
	}
	repository, err := git.PlainCloneContext(ctx, tempPath, false, cloneOptions)
	if err != nil {
		return CloneResult{}, ClassifyGitError(err, "clone", repo.URL)
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

	// The clone is there: the old copy can go, and the new one takes
	// its place.
	if err := os.RemoveAll(repoPath); err != nil {
		return CloneResult{}, GitError("failed to remove existing directory").
			WithCause(err).
			WithContext("path", repoPath).
			Build()
	}
	if err := os.Rename(tempPath, repoPath); err != nil {
		return CloneResult{}, GitError("failed to move the clone into place").
			WithCause(err).
			WithContext("path", repoPath).
			Build()
	}

	if c.buildCfg != nil && c.buildCfg.PruneNonDocPaths {
		if err := c.pruneNonDocTopLevel(repoPath, repo); err != nil {
			slog.Warn("prune non-doc paths failed", logfields.Name(repo.Name), slog.String("error", err.Error()))
		}
	}
	return result, nil
}

func (c *Client) updateOnce(ctx context.Context, repo appcfg.Repository) (string, error) {
	repoPath := filepath.Join(c.workspaceDir, repo.Name)
	if _, err := os.Stat(filepath.Join(repoPath, ".git")); err != nil { // missing => clone
		slog.Debug("Repository missing, cloning", logfields.Name(repo.Name))
		return c.cloneOnce(ctx, repo)
	}
	// A tag never moves, so the clone that is there is the one that is
	// wanted. The update fetches a branch, and a tag is not one: the
	// fetch of "refs/heads/v1.2.3" fails for a repository that has only
	// the tag, and the pages of the tag vanish from the site.
	if repo.IsTag {
		return repoPath, nil
	}
	path, err := c.updateExistingRepo(ctx, repoPath, repo)
	if err != nil && isCorruptionError(err) {
		// Self-heal: corrupt on-disk git state (e.g. truncated pack, bad
		// index checksum) cannot be repaired by re-fetching — go-git's
		// pack/idx decoders reject any partial or mismatched object and
		// the local pack won't be replaced just because a new fetch ran.
		// Wipe the workspace and let the next attempt (or the retry
		// wrapper) re-clone from scratch.
		slog.Warn("git repository corruption detected; re-cloning",
			logfields.Name(repo.Name),
			logfields.Path(repoPath),
			slog.String("error", err.Error()))
		if rmErr := os.RemoveAll(repoPath); rmErr != nil {
			slog.Error("failed to remove corrupt repository",
				logfields.Name(repo.Name),
				logfields.Path(repoPath),
				slog.String("error", rmErr.Error()))
		}
		// Drop any cached "remote HEAD unchanged" entry so a follow-up call
		// doesn't short-circuit a fetch that's now actually needed.
		if c.remoteHeadCache != nil {
			c.remoteHeadCache.DeleteByURL(repo.URL)
		}
		return c.cloneOnce(ctx, repo)
	}
	return path, err
}
