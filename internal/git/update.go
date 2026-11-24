package git

import (
	"context"
	stdErrors "errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/go-git/go-git/v5"
	ggitcfg "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"

	appcfg "git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/logfields"
)

func (c *Client) updateExistingRepo(ctx context.Context, repoPath string, repo appcfg.Repository) (string, error) {
	repository, err := git.PlainOpen(repoPath)
	if err != nil {
		return "", GitError("failed to open repository").
			WithCause(err).
			WithContext("path", repoPath).
			Build()
	}
	slog.Info("Updating repository", logfields.Name(repo.Name), slog.String("path", repoPath))
	wt, err := repository.Worktree()
	if err != nil {
		return "", GitError("failed to get worktree").
			WithCause(err).
			WithContext("path", repoPath).
			Build()
	}

	// Resolve target branch early
	branch := resolveTargetBranch(repository, repo)

	// Check if remote HEAD has changed using cache (skip fetch if unchanged)
	fetchNeeded := true
	var remoteSHA string
	if c.remoteHeadCache != nil {
		changed, sha, checkErr := c.CheckRemoteChanged(ctx, c.remoteHeadCache, repo, branch)
		if checkErr == nil {
			fetchNeeded = changed
			remoteSHA = sha
		}
	}

	// 1. Fetch remote refs (only if needed)
	if fetchNeeded {
		if fetchErr := c.performFetch(ctx, repository, repo, branch, remoteSHA); fetchErr != nil {
			// The clone is not at the head of the remote: the entry
			// of the cache would say that it is, and the next
			// update would skip the fetch.
			if c.remoteHeadCache != nil {
				c.remoteHeadCache.Delete(repo.URL, branch)
			}
			return "", fetchErr
		}
	} else {
		logSkippedFetch(repo.Name, branch, remoteSHA)
	}

	// 2. Checkout/create local branch & obtain refs
	localRef, remoteRef, err := checkoutAndGetRefs(repository, wt, branch)
	if err != nil {
		return "", err
	}

	// 3. Fast-forward or handle divergence
	if err := c.syncWithRemote(ctx, repository, wt, repo, branch, localRef, remoteRef); err != nil {
		// Divergence without hard reset is treated as permanent (REMOTE_DIVERGED)
		if strings.Contains(strings.ToLower(err.Error()), "diverged") {
			return "", ClassifyGitError(err, "update", repo.URL)
		}
		return "", err
	}

	// 4. Post-update hygiene (clean/prune)
	c.postUpdateCleanup(wt, repoPath, repo)

	return repoPath, nil
}

// fetchOrigin performs a fetch of the origin remote with appropriate depth, refspec, and authentication.
//
// Performance note: fetching "+refs/heads/*" can be very expensive for repositories with many branches.
// DocBuilder generally only needs a single target branch, so we fetch only that branch when provided.
func (c *Client) fetchOrigin(ctx context.Context, repository *git.Repository, repo appcfg.Repository, branch string) error {
	depth := 0
	if c.buildCfg != nil && c.buildCfg.ShallowDepth > 0 {
		depth = c.buildCfg.ShallowDepth
	}
	return c.fetchOriginWithDepth(ctx, repository, repo, branch, depth)
}

// fetchOriginWithDepth fetches the origin remote with an explicit history depth.
// A depth of 0 asks for the full history.
func (c *Client) fetchOriginWithDepth(ctx context.Context, repository *git.Repository, repo appcfg.Repository, branch string, depth int) error {
	refSpecs := []ggitcfg.RefSpec{"+refs/heads/*:refs/remotes/origin/*"}
	if branch != "" {
		// Fetch only the required branch.
		refSpecs = []ggitcfg.RefSpec{ggitcfg.RefSpec(
			fmt.Sprintf("+refs/heads/%s:refs/remotes/origin/%s", branch, branch),
		)}
	}

	fetchOpts := &git.FetchOptions{RemoteName: "origin", Tags: git.NoTags, RefSpecs: refSpecs}
	if depth > 0 {
		fetchOpts.Depth = depth
	}
	if repo.Auth != nil {
		auth, err := c.getAuth(repo.Auth)
		if err != nil {
			return err
		}
		fetchOpts.Auth = auth
	}
	if err := repository.FetchContext(ctx, fetchOpts); err != nil && !stdErrors.Is(err, git.NoErrAlreadyUpToDate) {
		return ClassifyGitError(err, "fetch", repo.URL)
	}
	return nil
}

// resolveTargetBranch determines the branch to update or checkout, following precedence rules:
// 1. Explicit branch in config, 2. Current HEAD branch, 3. Remote default branch, 4. "main" fallback.
func resolveTargetBranch(repository *git.Repository, repo appcfg.Repository) string {
	if repo.Branch != "" {
		return repo.Branch
	}
	if headRef, err := repository.Head(); err == nil && headRef.Name().IsBranch() {
		return headRef.Name().Short()
	}
	if def, err := resolveRemoteDefaultBranch(repository); err == nil && def != "" {
		return def
	}
	return "main"
}

// checkoutAndGetRefs ensures the local branch exists and is checked out, returning both local and remote references.
func checkoutAndGetRefs(repository *git.Repository, wt *git.Worktree, branch string) (localRef, remoteRef *plumbing.Reference, err error) {
	localBranchRef := plumbing.NewBranchReferenceName(branch)
	remoteBranchRef := plumbing.NewRemoteReferenceName("origin", branch)
	remoteRef, err = repository.Reference(remoteBranchRef, true)
	if err != nil {
		return nil, nil, GitError("failed to get remote reference").
			WithCause(err).
			WithContext("ref", remoteBranchRef.String()).
			Build()
	}
	localRef, lerr := repository.Reference(localBranchRef, true)
	if lerr != nil { // create local branch
		if err = wt.Checkout(&git.CheckoutOptions{Branch: localBranchRef, Create: true, Force: true}); err != nil {
			return nil, nil, GitError("failed to checkout new branch").
				WithCause(err).
				WithContext("branch", branch).
				Build()
		}
		var refErr error
		localRef, refErr = repository.Reference(localBranchRef, true)
		if refErr != nil {
			return nil, nil, GitError("failed to read the branch that was checked out").
				WithCause(refErr).
				WithContext("branch", branch).
				Build()
		}
	} else {
		if err = wt.Checkout(&git.CheckoutOptions{Branch: localBranchRef, Force: true}); err != nil {
			return nil, nil, GitError("failed to checkout existing branch").
				WithCause(err).
				WithContext("branch", branch).
				Build()
		}
	}
	return localRef, remoteRef, nil
}

// deepenDepths are the history depths tried when the local clone is too
// shallow to answer the ancestry question. The list starts small and grows:
// the missing history is normally only a few commits, and a deeper fetch
// re-uses the objects already on disk.
var deepenDepths = []int{16, 128, 1024}

// syncWithRemote fast-forwards or hard-resets the local branch depending on divergence and build config.
func (c *Client) syncWithRemote(ctx context.Context, repository *git.Repository, wt *git.Worktree, repo appcfg.Repository, branch string, localRef, remoteRef *plumbing.Reference) error {
	fastForwardPossible, ancestryErr := c.checkAncestry(ctx, repository, repo, branch, localRef.Hash(), remoteRef.Hash())
	hardReset := c.buildCfg != nil && c.buildCfg.HardResetOnDiverge
	if ancestryErr != nil {
		// The history could not be read, so we cannot claim the branch
		// diverged. Say what actually happened instead of guessing.
		slog.Warn("could not determine ancestry", logfields.Name(repo.Name), slog.String("branch", branch), slog.String("error", ancestryErr.Error()))
		if !hardReset {
			return GitError("could not read the local history to compare it against the remote").
				WithCause(ancestryErr).
				WithContext("hint", "increase shallow_depth or enable hard_reset_on_diverge").
				Build()
		}
		// The operator asked for the local branch to be aligned to the
		// remote (hard_reset_on_diverge, on by default). A history that
		// cannot be read does not stop that: the branch is aligned to
		// the remote, which is what the setting says.
		slog.Warn("the history could not be read, hard resetting", logfields.Name(repo.Name), slog.String("branch", branch))
		if err := wt.Reset(&git.ResetOptions{Commit: remoteRef.Hash(), Mode: git.HardReset}); err != nil {
			return GitError("failed to reset for hard-reset").
				WithCause(err).
				Build()
		}
		return nil
	}
	if fastForwardPossible {
		currentHead, _ := repository.Head()
		if err := wt.Reset(&git.ResetOptions{Commit: remoteRef.Hash(), Mode: git.HardReset}); err != nil {
			return GitError("failed to reset for fast-forward").
				WithCause(err).
				Build()
		}
		from := "unknown"
		if currentHead != nil {
			from = currentHead.Hash().String()[:8]
		}
		if currentHead != nil && currentHead.Hash() == remoteRef.Hash() {
			slog.Info("Repository already up-to-date", logfields.Name(repo.Name), slog.String("branch", branch), slog.String("commit", remoteRef.Hash().String()[:8]))
		} else {
			slog.Info("Fast-forwarded repository", logfields.Name(repo.Name), slog.String("branch", branch), slog.String("from", from), slog.String("to", remoteRef.Hash().String()[:8]))
		}
		return nil
	}
	if hardReset {
		slog.Warn("diverged branch, hard resetting", logfields.Name(repo.Name), slog.String("branch", branch))
		if err := wt.Reset(&git.ResetOptions{Commit: remoteRef.Hash(), Mode: git.HardReset}); err != nil {
			return GitError("failed to reset for hard-reset").
				WithCause(err).
				Build()
		}
		return nil
	}
	return GitError("local branch diverged from remote").
		WithContext("hint", "enable hard_reset_on_diverge to override").
		Build()
}

// checkAncestry answers whether the local commit is on the remote history.
// A shallow clone does not hold the history the question needs, so the fetch
// is deepened and the question asked again: an unreadable history is
// unknown, not a divergence.
func (c *Client) checkAncestry(ctx context.Context, repository *git.Repository, repo appcfg.Repository, branch string, local, remote plumbing.Hash) (bool, error) {
	ok, err := isAncestor(repository, local, remote)
	if err == nil {
		return ok, nil
	}
	slog.Warn("ancestor check failed, deepening history", logfields.Name(repo.Name), slog.String("error", err.Error()))
	for _, depth := range deepenDepths {
		if fetchErr := c.fetchOriginWithDepth(ctx, repository, repo, branch, depth); fetchErr != nil {
			return false, stdErrors.Join(err, fetchErr)
		}
		ok, err = isAncestor(repository, local, remote)
		if err == nil {
			return ok, nil
		}
	}
	return false, err
}

// postUpdateCleanup applies optional workspace hygiene, such as cleaning untracked files and pruning non-doc paths.
func (c *Client) postUpdateCleanup(wt *git.Worktree, repoPath string, repo appcfg.Repository) {
	if c.buildCfg != nil && c.buildCfg.CleanUntracked {
		if err := wt.Clean(&git.CleanOptions{Dir: true}); err != nil {
			slog.Warn("clean untracked failed", slog.String("error", err.Error()))
		}
	}
	if c.buildCfg != nil && c.buildCfg.PruneNonDocPaths {
		if err := c.pruneNonDocTopLevel(repoPath, repo); err != nil {
			slog.Warn("prune non-doc paths failed", logfields.Name(repo.Name), slog.String("error", err.Error()))
		}
	}
}

func resolveRemoteDefaultBranch(repo *git.Repository) (string, error) {
	ref, err := repo.Reference(plumbing.ReferenceName("refs/remotes/origin/HEAD"), true)
	if err != nil {
		return "", err
	}
	target := ref.Target()
	if target == "" {
		return "", GitError("origin/HEAD target empty").Build()
	}
	return target.Short(), nil
}

func isAncestor(repo *git.Repository, a, b plumbing.Hash) (bool, error) {
	if a == b {
		return true, nil
	}
	seen := map[plumbing.Hash]struct{}{}
	queue := []plumbing.Hash{b}
	for len(queue) > 0 {
		h := queue[0]
		queue = queue[1:]
		if h == a {
			return true, nil
		}
		if _, ok := seen[h]; ok {
			continue
		}
		seen[h] = struct{}{}
		commit, err := repo.CommitObject(h)
		if err != nil {
			return false, err
		}
		queue = append(queue, commit.ParentHashes...)
	}
	return false, nil
}

// performFetch executes fetch operation and updates cache.
func (c *Client) performFetch(ctx context.Context, repository *git.Repository, repo appcfg.Repository, branch, remoteSHA string) error {
	logFetchOperation(repo.Name, branch, remoteSHA)

	if fetchErr := c.fetchOrigin(ctx, repository, repo, branch); fetchErr != nil {
		return ClassifyGitError(fetchErr, "fetch", repo.URL)
	}

	// Update cache with new remote HEAD
	if c.remoteHeadCache != nil && remoteSHA != "" {
		c.remoteHeadCache.Set(repo.URL, branch, remoteSHA)
	}

	return nil
}

// logFetchOperation logs appropriate message based on whether remote SHA is known.
func logFetchOperation(name, branch, remoteSHA string) {
	if remoteSHA != "" {
		slog.Info("Repository changed, fetching updates",
			logfields.Name(name),
			slog.String("branch", branch),
			slog.String("remote_commit", remoteSHA[:8]))
	} else {
		slog.Info("Fetching repository updates",
			logfields.Name(name),
			slog.String("branch", branch))
	}
}

// logSkippedFetch logs when fetch is skipped due to unchanged remote.
func logSkippedFetch(name, branch, remoteSHA string) {
	slog.Info("Repository unchanged, skipping fetch",
		logfields.Name(name),
		slog.String("branch", branch),
		// The hash can be empty: the remote has no branch of that
		// name, and the slice of it stopped the daemon.
		slog.String("commit", shortSHA(remoteSHA)))
}
