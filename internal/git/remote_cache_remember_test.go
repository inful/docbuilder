package git

import (
	"testing"

	appcfg "git.home.luguber.info/inful/docbuilder/internal/config"
)

// The cache has to remember what the local clone holds. Without it
// every check says "changed", and every webhook rebuilds the whole
// site - the check that is supposed to skip the build never skips it.
//
// The cache holds the clone and not the remote: the check compares the
// remote with the clone, and the clone is updated by the build that
// follows the check.
func TestTheRemoteHeadCacheRemembersWhatTheCloneHolds(t *testing.T) {
	bare := localRemote(t)

	cache, err := NewRemoteHeadCache(t.TempDir())
	if err != nil {
		t.Fatalf("new cache: %v", err)
	}
	client := NewClient(t.TempDir()).WithRemoteHeadCache(cache)
	repo := appcfg.Repository{Name: "repo", URL: bare, Branch: "master"}

	// The clone holds the head of the remote.
	if _, cloneErr := client.CloneRepoWithMetadata(t.Context(), repo); cloneErr != nil {
		t.Fatalf("clone: %v", cloneErr)
	}

	changed, sha, err := client.CheckRemoteChanged(t.Context(), cache, repo, "master")
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if changed {
		t.Fatal("the check said the remote moved, but the clone is at its head")
	}
	if sha == "" {
		t.Fatal("the check found no commit")
	}
}

// A remote that moved is a change, even when the cache knows it.
func TestTheRemoteHeadCacheSeesAMovedRemote(t *testing.T) {
	bare := localRemote(t)

	cache, err := NewRemoteHeadCache(t.TempDir())
	if err != nil {
		t.Fatalf("new cache: %v", err)
	}
	client := NewClient(t.TempDir())
	repo := appcfg.Repository{Name: "repo", URL: bare, Branch: "master"}

	if _, _, checkErr := client.CheckRemoteChanged(t.Context(), cache, repo, "master"); checkErr != nil {
		t.Fatalf("check: %v", checkErr)
	}

	// A different branch of the same remote is a different entry.
	cache.Set(repo.URL, "master", "0000000000000000000000000000000000000000")

	changed, _, err := client.CheckRemoteChanged(t.Context(), cache, repo, "master")
	if err != nil {
		t.Fatalf("second check: %v", err)
	}
	if !changed {
		t.Fatal("the check did not see the moved remote")
	}
}
