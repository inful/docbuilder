package hugo

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/git"
)

// RepoFetchResult captures the outcome path and (optional) pre/post head commits for change detection.
type RepoFetchResult struct {
	Name     string
	Path     string
	PreHead  string // empty if unknown
	PostHead string // empty if clone/update failed to resolve
	Err      error
	Updated  bool // true if repository contents potentially changed (clone, new commits, or forced reset)
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

func newDefaultRepoFetcher(workspace string, buildCfg *config.BuildConfig) *defaultRepoFetcher {
	return &defaultRepoFetcher{workspace: workspace, buildCfg: buildCfg}
}

func (f *defaultRepoFetcher) Fetch(ctx context.Context, strategy config.CloneStrategy, repo config.Repository) RepoFetchResult {
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
	if attemptUpdate {
		path, err = client.UpdateRepository(repo)
	} else {
		path, err = client.CloneRepository(repo)
	}
	res.Path = path
	res.PreHead = preHead
	if err != nil {
		res.Err = err
		return res
	}
	// Determine post head
	if h, herr := readRepoHead(path); herr == nil {
		res.PostHead = h
	}
	// Updated determination: if cloning (preHead empty) or heads differ
	res.Updated = preHead == "" || (preHead != "" && res.PostHead != "" && preHead != res.PostHead)
	return res
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
