// Package git provides functions for updating, synchronizing, and managing Git repositories in DocBuilder.
package git

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"

	appcfg "git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/logfields"
	"github.com/go-git/go-git/v5"
	ggitcfg "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
)

func (c *Client) updateExistingRepo(repoPath string, repo appcfg.Repository) (string, error) {
	repository, err := git.PlainOpen(repoPath)
	if err != nil {
		return "", fmt.Errorf("open repo: %w", err)
	}
	slog.Info("Updating repository", logfields.Name(repo.Name), slog.String("path", repoPath))
	wt, err := repository.Worktree()
	if err != nil {
		return "", fmt.Errorf("worktree: %w", err)
	}

	// 1. Fetch remote refs
	if err := c.fetchOrigin(repository, repo); err != nil {
		return "", classifyFetchError(repo.URL, err)
	}

	// 2. Resolve target branch
	branch, err := resolveTargetBranch(repository, repo)
	if err != nil {
		return "", err
	}

	// 3. Checkout/create local branch & obtain refs
	localRef, remoteRef, err := checkoutAndGetRefs(repository, wt, branch)
	if err != nil {
		return "", err
	}

	// 4. Fast-forward or handle divergence
	if err := c.syncWithRemote(repository, wt, repo, branch, localRef, remoteRef); err != nil {
		// Divergence without hard reset is treated as permanent (REMOTE_DIVERGED)
		if strings.Contains(strings.ToLower(err.Error()), "diverged") {
			return "", &RemoteDivergedError{Op: "update", URL: repo.URL, Branch: branch, Err: err}
		}
		return "", err
	}

	// 5. Post-update hygiene (clean/prune)
	c.postUpdateCleanup(wt, repoPath, repo)

	// 6. Logging
	logRepositoryUpdated(repository, repo, branch)
	return repoPath, nil
}

// fetchOrigin performs a fetch of the origin remote with appropriate depth, refspec, and authentication.
func (c *Client) fetchOrigin(repository *git.Repository, repo appcfg.Repository) error {
	depth := 0
	if c.buildCfg != nil && c.buildCfg.ShallowDepth > 0 {
		depth = c.buildCfg.ShallowDepth
	}
	fetchOpts := &git.FetchOptions{RemoteName: "origin", Tags: git.NoTags, RefSpecs: []ggitcfg.RefSpec{"+refs/heads/*:refs/remotes/origin/*"}}
	if depth > 0 {
		fetchOpts.Depth = depth
	}
	if repo.Auth != nil {
		auth, err := c.getAuthentication(repo.Auth)
		if err != nil {
			return err
		}
		fetchOpts.Auth = auth
	}
	if err := repository.Fetch(fetchOpts); err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
		return fmt.Errorf("fetch: %w", err)
	}
	return nil
}

// resolveTargetBranch determines the branch to update or checkout, following precedence rules:
// 1. Explicit branch in config, 2. Current HEAD branch, 3. Remote default branch, 4. "main" fallback.
func resolveTargetBranch(repository *git.Repository, repo appcfg.Repository) (string, error) {
	if repo.Branch != "" {
		return repo.Branch, nil
	}
	if headRef, err := repository.Head(); err == nil && headRef.Name().IsBranch() {
		return headRef.Name().Short(), nil
	}
	if def, err := resolveRemoteDefaultBranch(repository); err == nil && def != "" {
		return def, nil
	}
	return "main", nil
}

// checkoutAndGetRefs ensures the local branch exists and is checked out, returning both local and remote references.
func checkoutAndGetRefs(repository *git.Repository, wt *git.Worktree, branch string) (localRef, remoteRef *plumbing.Reference, err error) {
	localBranchRef := plumbing.NewBranchReferenceName(branch)
	remoteBranchRef := plumbing.NewRemoteReferenceName("origin", branch)
	remoteRef, err = repository.Reference(remoteBranchRef, true)
	if err != nil {
		return nil, nil, fmt.Errorf("remote ref: %w", err)
	}
	localRef, lerr := repository.Reference(localBranchRef, true)
	if lerr != nil { // create local branch
		if err = wt.Checkout(&git.CheckoutOptions{Branch: localBranchRef, Create: true, Force: true}); err != nil {
			return nil, nil, fmt.Errorf("checkout new branch: %w", err)
		}
		localRef, _ = repository.Reference(localBranchRef, true)
	} else {
		if err = wt.Checkout(&git.CheckoutOptions{Branch: localBranchRef, Force: true}); err != nil {
			return nil, nil, fmt.Errorf("checkout existing branch: %w", err)
		}
	}
	return localRef, remoteRef, nil
}

// syncWithRemote fast-forwards or hard-resets the local branch depending on divergence and build config.
func (c *Client) syncWithRemote(repository *git.Repository, wt *git.Worktree, repo appcfg.Repository, branch string, localRef, remoteRef *plumbing.Reference) error {
	fastForwardPossible, ffErr := isAncestor(repository, localRef.Hash(), remoteRef.Hash())
	if ffErr != nil {
		slog.Warn("ancestor check failed", slog.String("error", ffErr.Error()))
	}
	if fastForwardPossible {
		currentHead, _ := repository.Head()
		if err := wt.Reset(&git.ResetOptions{Commit: remoteRef.Hash(), Mode: git.HardReset}); err != nil {
			return fmt.Errorf("fast-forward reset: %w", err)
		}
		if currentHead != nil && currentHead.Hash() == remoteRef.Hash() {
			slog.Info("Repository already up-to-date", logfields.Name(repo.Name), slog.String("branch", branch), slog.String("commit", remoteRef.Hash().String()[:8]))
		} else {
			slog.Info("Fast-forwarded repository", logfields.Name(repo.Name), slog.String("branch", branch), slog.String("from", currentHead.Hash().String()[:8]), slog.String("to", remoteRef.Hash().String()[:8]))
		}
		return nil
	}
	hardReset := c.buildCfg != nil && c.buildCfg.HardResetOnDiverge
	if hardReset {
		slog.Warn("diverged branch, hard resetting", logfields.Name(repo.Name), slog.String("branch", branch))
		if err := wt.Reset(&git.ResetOptions{Commit: remoteRef.Hash(), Mode: git.HardReset}); err != nil {
			return fmt.Errorf("hard reset: %w", err)
		}
		return nil
	}
	return fmt.Errorf("local branch diverged from remote (enable hard_reset_on_diverge to override)")
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

// logRepositoryUpdated logs a repository update summary, including the short commit hash if available.
func logRepositoryUpdated(repository *git.Repository, repo appcfg.Repository, branch string) {
	if headRef, err := repository.Head(); err == nil {
		slog.Info("Repository updated", logfields.Name(repo.Name), slog.String("branch", branch), slog.String("commit", headRef.Hash().String()[:8]))
	} else {
		slog.Info("Repository updated", logfields.Name(repo.Name), slog.String("branch", branch))
	}
}

func resolveRemoteDefaultBranch(repo *git.Repository) (string, error) {
	ref, err := repo.Reference(plumbing.ReferenceName("refs/remotes/origin/HEAD"), true)
	if err != nil {
		return "", err
	}
	target := ref.Target()
	if target == "" {
		return "", fmt.Errorf("origin/HEAD target empty")
	}
	return plumbing.ReferenceName(target).Short(), nil
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
