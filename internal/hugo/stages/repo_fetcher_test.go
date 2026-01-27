package stages

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	ggit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/require"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	gitpkg "git.home.luguber.info/inful/docbuilder/internal/git"
)

func TestDefaultRepoFetcher_FetchPinnedCommit_ChecksOutExactSHAAndSkipsWhenAlreadyAtDesired(t *testing.T) {
	remotePath, commit1, commit2 := initGitRepoWithTwoCommits(t)

	workspace := t.TempDir()
	fetcher := NewDefaultRepoFetcher(workspace, nil)

	repoCfg := config.Repository{
		Name:         "repo-1",
		URL:          remotePath,
		Branch:       "master",
		PinnedCommit: commit1,
	}

	res1 := fetcher.Fetch(t.Context(), config.CloneStrategyFresh, repoCfg)
	require.NoError(t, res1.Err)
	require.Equal(t, commit1, res1.PostHead)
	require.NotEmpty(t, res1.Path)
	require.True(t, res1.Updated)

	head1, err := gitpkg.ReadRepoHead(res1.Path)
	require.NoError(t, err)
	require.Equal(t, commit1, head1)

	res2 := fetcher.Fetch(t.Context(), config.CloneStrategyUpdate, repoCfg)
	require.NoError(t, res2.Err)
	require.Equal(t, commit1, res2.PreHead)
	require.Equal(t, commit1, res2.PostHead)
	require.False(t, res2.Updated)

	head2, err := gitpkg.ReadRepoHead(res2.Path)
	require.NoError(t, err)
	require.Equal(t, commit1, head2)

	repoCfg.PinnedCommit = commit2
	res3 := fetcher.Fetch(t.Context(), config.CloneStrategyUpdate, repoCfg)
	require.NoError(t, res3.Err)
	require.Equal(t, commit1, res3.PreHead)
	require.Equal(t, commit2, res3.PostHead)
	require.True(t, res3.Updated)

	head3, err := gitpkg.ReadRepoHead(res3.Path)
	require.NoError(t, err)
	require.Equal(t, commit2, head3)
}

func initGitRepoWithTwoCommits(t *testing.T) (repoPath, commit1, commit2 string) {
	t.Helper()

	repoPath = t.TempDir()
	repo, err := ggit.PlainInit(repoPath, false)
	require.NoError(t, err)

	wt, err := repo.Worktree()
	require.NoError(t, err)

	fileRel := "README.md"
	fileAbs := filepath.Join(repoPath, fileRel)

	when1 := time.Now().Add(-2 * time.Hour)
	require.NoError(t, os.WriteFile(fileAbs, []byte("one\n"), 0o600))
	_, err = wt.Add(fileRel)
	require.NoError(t, err)
	_, err = wt.Commit("commit 1", &ggit.CommitOptions{Author: &object.Signature{Name: "t", Email: "t@example.invalid", When: when1}})
	require.NoError(t, err)
	ref1, err := repo.Head()
	require.NoError(t, err)
	commit1 = ref1.Hash().String()

	when2 := time.Now().Add(-1 * time.Hour)
	require.NoError(t, os.WriteFile(fileAbs, []byte("two\n"), 0o600))
	_, err = wt.Add(fileRel)
	require.NoError(t, err)
	_, err = wt.Commit("commit 2", &ggit.CommitOptions{Author: &object.Signature{Name: "t", Email: "t@example.invalid", When: when2}})
	require.NoError(t, err)
	ref2, err := repo.Head()
	require.NoError(t, err)
	commit2 = ref2.Hash().String()

	return repoPath, commit1, commit2
}
