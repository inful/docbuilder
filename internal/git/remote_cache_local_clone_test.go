package git

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"

	appcfg "git.home.luguber.info/inful/docbuilder/internal/config"
)

// pushCommit adds a commit to the bare remote and returns its SHA.
func pushCommit(t *testing.T, bare, message string) string {
	t.Helper()

	dir := t.TempDir()
	repo, cloneErr := git.PlainClone(dir, false, &git.CloneOptions{URL: bare})
	if cloneErr != nil {
		t.Fatalf("clone the remote: %v", cloneErr)
	}
	wt, wtErr := repo.Worktree()
	if wtErr != nil {
		t.Fatalf("worktree: %v", wtErr)
	}
	if writeErr := os.WriteFile(filepath.Join(dir, message+".txt"), []byte(message), 0o600); writeErr != nil {
		t.Fatalf("write the file: %v", writeErr)
	}
	if _, addErr := wt.Add(message + ".txt"); addErr != nil {
		t.Fatalf("add: %v", addErr)
	}
	hash, commitErr := wt.Commit(message, &git.CommitOptions{Author: &object.Signature{Name: "a", Email: "a@b"}})
	if commitErr != nil {
		t.Fatalf("commit: %v", commitErr)
	}
	if pushErr := repo.Push(&git.PushOptions{RemoteName: "origin"}); pushErr != nil {
		t.Fatalf("push: %v", pushErr)
	}
	return hash.String()
}

// The cache holds what the local clone has, and not what the remote
// holds. The check wrote the SHA of the remote into the cache before
// the clone was updated, and the build that followed skipped the fetch:
// the site stayed at the old commit.
func TestTheUpdateFetchesWhenTheRemoteMovedAfterTheCheck(t *testing.T) {
	bare := localRemote(t)

	cache, err := NewRemoteHeadCache(t.TempDir())
	if err != nil {
		t.Fatalf("new cache: %v", err)
	}
	workspace := t.TempDir()
	client := NewClient(workspace).WithRemoteHeadCache(cache)
	repo := appcfg.Repository{Name: "repo", URL: bare, Branch: "master"}

	if _, cloneErr := client.CloneRepoWithMetadata(t.Context(), repo); cloneErr != nil {
		t.Fatalf("clone: %v", cloneErr)
	}

	// The remote moves.
	newSHA := pushCommit(t, bare, "b")

	// The check of the daemon sees the move.
	changed, sha, err := client.CheckRemoteChanged(t.Context(), cache, repo, "master")
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if !changed {
		t.Fatal("the check did not see the moved remote")
	}
	if sha != newSHA {
		t.Fatalf("the check found %s and not %s", sha, newSHA)
	}

	// The build that follows updates the clone.
	if _, updateErr := client.UpdateRepo(t.Context(), repo); updateErr != nil {
		t.Fatalf("update: %v", updateErr)
	}

	repository, err := git.PlainOpen(filepath.Join(workspace, "repo"))
	if err != nil {
		t.Fatalf("open the clone: %v", err)
	}
	head, err := repository.Head()
	if err != nil {
		t.Fatalf("head: %v", err)
	}
	if head.Hash().String() != newSHA {
		t.Fatalf("the clone is at %s and not at %s: the site stays at the old commit",
			head.Hash().String(), newSHA)
	}
}
