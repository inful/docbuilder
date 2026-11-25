package stages

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	ggit "github.com/go-git/go-git/v5"
	ggitcfg "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/require"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/git"
)

// localRemoteRepo creates a bare repository with one commit and returns
// its path.
func localRemoteRepo(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	bare := filepath.Join(tmp, "remote.git")
	if _, err := ggit.PlainInit(bare, true); err != nil {
		t.Fatalf("init bare: %v", err)
	}
	work := filepath.Join(tmp, "work")
	repo, err := ggit.PlainInit(work, false)
	if err != nil {
		t.Fatalf("init work: %v", err)
	}
	if writeErr := os.WriteFile(filepath.Join(work, "a.txt"), []byte("a"), 0o600); writeErr != nil {
		t.Fatalf("write file: %v", writeErr)
	}
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatalf("worktree: %v", err)
	}
	if _, err := wt.Add("a.txt"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, err := wt.Commit("a", &ggit.CommitOptions{Author: &object.Signature{Name: "a", Email: "a@b"}}); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if _, err := repo.CreateRemote(&ggitcfg.RemoteConfig{Name: "origin", URLs: []string{bare}}); err != nil {
		t.Fatalf("remote: %v", err)
	}
	if err := repo.Push(&ggit.PushOptions{RemoteName: "origin"}); err != nil {
		t.Fatalf("push: %v", err)
	}
	return bare
}

// The build fetches only the repository whose remote moved: the cache
// holds what the local clone has, and the fetch of a repository that
// did not move is a request to the forge for nothing.
func TestTheFetcherSkipsTheRepositoryThatDidNotMove(t *testing.T) {
	bare := localRemoteRepo(t)

	cache, err := git.NewRemoteHeadCache(t.TempDir())
	require.NoError(t, err)

	fetcher := NewDefaultRepoFetcher(t.TempDir(), nil, cache)
	repo := config.Repository{Name: "repo", URL: bare, Branch: "master"}

	// The first fetch clones the repository.
	res := fetcher.Fetch(t.Context(), config.CloneStrategyUpdate, repo)
	require.NoError(t, res.Err)

	entry := cache.Get(bare, "master")
	require.NotNil(t, entry, "the cache does not hold what the clone has")
	require.Equal(t, res.PostHead, entry.CommitSHA)

	// The remote did not move: the second fetch is skipped.
	var sink bytes.Buffer
	old := slog.Default()
	defer slog.SetDefault(old)
	slog.SetDefault(slog.New(slog.NewTextHandler(&sink, &slog.HandlerOptions{Level: slog.LevelInfo})))

	res = fetcher.Fetch(t.Context(), config.CloneStrategyUpdate, repo)
	require.NoError(t, res.Err)
	require.Contains(t, sink.String(), "skipping fetch",
		"the repository was fetched even though its remote did not move")
}
