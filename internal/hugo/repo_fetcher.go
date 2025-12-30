package hugo

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
		if _, err := gitStatRepo(repoPath); err == nil {
			attemptUpdate = true
			if h, herr := readRepoHead(repoPath); herr == nil {
				preHead = h
			}
		}
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
func gitStatRepo(path string) (bool, error) {
	// minimal existence check reused from stage logic previously
	if fi, err := os.Stat(path); err != nil || !fi.IsDir() { // missing or not dir
		return false, err
	}
	if _, err := os.Stat(path + "/.git"); err != nil { // missing .git
		return false, fmt.Errorf("no git dir: %w", err)
	}
	return true, nil
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
