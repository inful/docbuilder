package git

import (
	"errors"
	"fmt"
	"log/slog"

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
	wt, err := repository.Worktree()
	if err != nil {
		return "", fmt.Errorf("worktree: %w", err)
	}

	depth := 0
	if c.buildCfg != nil && c.buildCfg.ShallowDepth > 0 {
		depth = c.buildCfg.ShallowDepth
	}
	fetchOpts := &git.FetchOptions{RemoteName: "origin", Tags: git.NoTags}
	if depth > 0 {
		fetchOpts.Depth = depth
	}
	fetchOpts.RefSpecs = []ggitcfg.RefSpec{"+refs/heads/*:refs/remotes/origin/*"}
	if repo.Auth != nil {
		auth, aerr := c.getAuthentication(repo.Auth)
		if aerr != nil {
			return "", aerr
		}
		fetchOpts.Auth = auth
	}
	if err := repository.Fetch(fetchOpts); err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
		return "", fmt.Errorf("fetch: %w", err)
	}

	branch := repo.Branch
	if branch == "" {
		if headRef, herr := repository.Head(); herr == nil && headRef.Name().IsBranch() {
			branch = headRef.Name().Short()
		} else {
			if def, derr := resolveRemoteDefaultBranch(repository); derr == nil {
				branch = def
			} else {
				branch = "main"
			}
		}
	}
	localBranchRef := plumbing.NewBranchReferenceName(branch)
	remoteBranchRef := plumbing.NewRemoteReferenceName("origin", branch)
	remoteRef, err := repository.Reference(remoteBranchRef, true)
	if err != nil {
		return "", fmt.Errorf("remote ref: %w", err)
	}
	localRef, lerr := repository.Reference(localBranchRef, true)
	if lerr != nil { // create local branch
		if err := wt.Checkout(&git.CheckoutOptions{Branch: localBranchRef, Create: true, Force: true}); err != nil {
			return "", fmt.Errorf("checkout new branch: %w", err)
		}
		localRef, _ = repository.Reference(localBranchRef, true)
	} else {
		if err := wt.Checkout(&git.CheckoutOptions{Branch: localBranchRef, Force: true}); err != nil {
			return "", fmt.Errorf("checkout existing branch: %w", err)
		}
	}
	fastForwardPossible, ffErr := isAncestor(repository, localRef.Hash(), remoteRef.Hash())
	if ffErr != nil {
		slog.Warn("ancestor check failed", slog.String("error", ffErr.Error()))
	}
	if fastForwardPossible {
		if err := wt.Reset(&git.ResetOptions{Commit: remoteRef.Hash(), Mode: git.HardReset}); err != nil {
			return "", fmt.Errorf("fast-forward reset: %w", err)
		}
	} else {
		hardReset := c.buildCfg != nil && c.buildCfg.HardResetOnDiverge
		if hardReset {
			slog.Warn("diverged branch, hard resetting", logfields.Name(repo.Name), slog.String("branch", branch))
			if err := wt.Reset(&git.ResetOptions{Commit: remoteRef.Hash(), Mode: git.HardReset}); err != nil {
				return "", fmt.Errorf("hard reset: %w", err)
			}
		} else {
			return "", fmt.Errorf("local branch diverged from remote (enable hard_reset_on_diverge to override)")
		}
	}
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
	if headRef, err := repository.Head(); err == nil {
		slog.Info("Repository updated", logfields.Name(repo.Name), slog.String("branch", branch), slog.String("commit", headRef.Hash().String()[:8]))
	} else {
		slog.Info("Repository updated", logfields.Name(repo.Name), slog.String("branch", branch))
	}
	return repoPath, nil
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
		for _, p := range commit.ParentHashes {
			queue = append(queue, p)
		}
	}
	return false, nil
}
