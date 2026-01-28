package stages

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	ggit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/git"
)

// RepoFetchResult captures the outcome path and (optional) pre/post head commits for change detection.
type RepoFetchResult struct {
	Name       string
	Path       string
	PreHead    string    // empty if unknown
	PostHead   string    // empty if clone/update failed to resolve
	CommitDate time.Time // commit date of PostHead
	Err        error
	Updated    bool // true if repository contents potentially changed (clone, new commits, or forced reset)
}

// RepoFetcher defines cloning/updating behavior abstracted from stage logic so future
// implementations (e.g., shallow mirror pool, cache service, mock) can be swapped without
// modifying the clone stage. Implementations must be concurrency safe (stateless or internal sync).
type RepoFetcher interface {
	// Fetch ensures a repository is present locally according to strategy semantics.
	// strategy: one of config.CloneStrategy* constants. Returns RepoFetchResult.
	Fetch(ctx context.Context, strategy config.CloneStrategy, repo config.Repository) RepoFetchResult
}

// defaultRepoFetcher wraps the existing git.Client for backwards-compatible behavior.
type defaultRepoFetcher struct {
	workspace string
	buildCfg  *config.BuildConfig
}

// NewDefaultRepoFetcher creates a new default repository fetcher (exported for commands package).
func NewDefaultRepoFetcher(workspace string, buildCfg *config.BuildConfig) RepoFetcher {
	return &defaultRepoFetcher{workspace: workspace, buildCfg: buildCfg}
}

func (f *defaultRepoFetcher) Fetch(_ context.Context, strategy config.CloneStrategy, repo config.Repository) RepoFetchResult {
	res := RepoFetchResult{Name: repo.Name}
	client := git.NewClient(f.workspace)
	if f.buildCfg != nil {
		client = client.WithBuildConfig(f.buildCfg)
	}

	// Snapshot builds: if a specific commit SHA is pinned for this repo, ensure the
	// working copy is checked out at that exact commit.
	if repo.PinnedCommit != "" {
		return f.fetchPinnedCommit(client, strategy, repo)
	}

	attemptUpdate := false
	var preHead string
	switch strategy {
	case config.CloneStrategyUpdate:
		attemptUpdate = true
	case config.CloneStrategyAuto:
		// Determine if repo exists already
		// We replicate minimal logic; detailed head read happens after successful op.
		// Use same path logic as client.
		repoPath := filepath.Join(f.workspace, repo.Name)
		if err := gitStatRepo(repoPath); err == nil {
			attemptUpdate = true
			if h, herr := readRepoHead(repoPath); herr == nil {
				preHead = h
			}
		}
	case config.CloneStrategyFresh:
		// Fresh clone - don't attempt update (attemptUpdate remains false)
	}
	var path string
	var err error
	var commitDate time.Time
	if attemptUpdate {
		path, commitDate, err = f.performUpdate(client, repo)
	} else {
		path, commitDate, err = f.performClone(client, repo, &res)
	}
	res.Path = path
	res.CommitDate = commitDate
	res.PreHead = preHead
	if err != nil {
		res.Err = err
		return res
	}
	// Determine post head (for update path or if clone didn't set it)
	if res.PostHead == "" {
		if h, herr := readRepoHead(path); herr == nil {
			res.PostHead = h
		}
	}
	// Updated determination: if cloning (preHead empty) or heads differ
	res.Updated = preHead == "" || (preHead != "" && res.PostHead != "" && preHead != res.PostHead)
	return res
}

func (f *defaultRepoFetcher) fetchPinnedCommit(client *git.Client, strategy config.CloneStrategy, repo config.Repository) RepoFetchResult {
	res := RepoFetchResult{Name: repo.Name}
	repoPath := filepath.Join(f.workspace, repo.Name)

	preHead, _ := readRepoHead(repoPath)
	res.PreHead = preHead

	// If we already have the desired commit checked out, skip fetch/update entirely.
	if preHead != "" && preHead == repo.PinnedCommit {
		res.Path = repoPath
		res.PostHead = repo.PinnedCommit
		res.CommitDate = getCommitDate(repoPath, repo.PinnedCommit)
		res.Updated = false
		return res
	}

	// Optimization: if the repo already exists locally and already has the pinned
	// commit object available, we can skip any clone/update and just force-checkout
	// the desired commit.
	if err := gitStatRepo(repoPath); err == nil {
		if checkedOutAt, cerr := checkoutExactCommit(repoPath, repo.PinnedCommit); cerr == nil {
			res.Path = repoPath
			res.PostHead = repo.PinnedCommit
			res.CommitDate = checkedOutAt
			res.Updated = preHead == "" || preHead != repo.PinnedCommit
			return res
		}
	}

	// Ensure repo exists locally.
	attemptUpdate := false
	repoExists := gitStatRepo(repoPath) == nil
	switch strategy {
	case config.CloneStrategyUpdate:
		attemptUpdate = true
	case config.CloneStrategyAuto:
		if repoExists {
			attemptUpdate = true
		}
	case config.CloneStrategyFresh:
		// For pinned commits, prefer updating an existing clone instead of recloning.
		// If the commit wasn't locally available, an update can fetch it.
		attemptUpdate = repoExists
	}

	var path string
	var err error
	var commitDate time.Time
	if attemptUpdate {
		path, commitDate, err = f.performUpdate(client, repo)
	} else {
		path, commitDate, err = f.performClone(client, repo, &res)
	}
	res.Path = path
	res.CommitDate = commitDate
	if err != nil {
		res.Err = err
		return res
	}

	// Checkout exact pinned SHA (detached HEAD).
	checkedOutAt, cerr := checkoutExactCommit(path, repo.PinnedCommit)
	if cerr != nil {
		res.Err = cerr
		return res
	}

	res.PostHead = repo.PinnedCommit
	res.CommitDate = checkedOutAt
	res.Updated = preHead == "" || preHead != repo.PinnedCommit
	return res
}

// performUpdate updates an existing repository and returns its path, commit date, and error.
func (f *defaultRepoFetcher) performUpdate(client *git.Client, repo config.Repository) (string, time.Time, error) {
	path, err := client.UpdateRepo(repo)
	var commitDate time.Time

	// For updates, try to get commit date by reading HEAD
	if err == nil {
		if h, herr := readRepoHead(path); herr == nil {
			commitDate = getCommitDate(path, h)
		}
	}

	return path, commitDate, err
}

// performClone performs a fresh clone and returns its path, commit date, and error.
// It also sets the PostHead field in res.
func (f *defaultRepoFetcher) performClone(client *git.Client, repo config.Repository, res *RepoFetchResult) (string, time.Time, error) {
	result, err := client.CloneRepoWithMetadata(repo)
	var path string
	var commitDate time.Time

	if err == nil {
		path = result.Path
		commitDate = result.CommitDate
		res.PostHead = result.CommitSHA
	}

	return path, commitDate, err
}

// gitStatRepo isolates os.Stat dependency (simple indirection aids test stubbing later).
func gitStatRepo(path string) error {
	// minimal existence check reused from stage logic previously
	if fi, err := os.Stat(path); err != nil || !fi.IsDir() { // missing or not dir
		return err
	}
	if _, err := os.Stat(path + "/.git"); err != nil { // missing .git
		return fmt.Errorf("no git dir: %w", err)
	}
	return nil
}

// getCommitDate retrieves the commit date for a given commit hash in a repository.
// Returns zero time if the commit date cannot be determined.
func getCommitDate(repoPath, commitSHA string) time.Time {
	repo, err := ggit.PlainOpen(repoPath)
	if err != nil {
		return time.Time{}
	}
	hash := plumbing.NewHash(commitSHA)
	commit, err := repo.CommitObject(hash)
	if err != nil {
		return time.Time{}
	}
	return commit.Author.When
}

func checkoutExactCommit(repoPath, commitSHA string) (time.Time, error) {
	repo, err := ggit.PlainOpen(repoPath)
	if err != nil {
		return time.Time{}, fmt.Errorf("open repo for checkout: %w", err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		return time.Time{}, fmt.Errorf("get worktree for checkout: %w", err)
	}
	h := plumbing.NewHash(commitSHA)
	if checkoutErr := wt.Checkout(&ggit.CheckoutOptions{Hash: h, Force: true}); checkoutErr != nil {
		return time.Time{}, fmt.Errorf("checkout commit %s: %w", commitSHA, checkoutErr)
	}
	commit, _ := repo.CommitObject(h)
	if commit == nil {
		return time.Time{}, nil
	}
	return commit.Author.When, nil
}
