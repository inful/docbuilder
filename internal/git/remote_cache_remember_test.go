package git

import (
	"testing"

	appcfg "git.home.luguber.info/inful/docbuilder/internal/config"
)

// The cache has to remember what the remote held at the last check.
// Without it the cache stays empty, every check says "changed", and
// every webhook rebuilds the whole site - the check that is supposed
// to skip the build never skips it.
func TestTheRemoteHeadCacheRemembersWhatTheRemoteHolds(t *testing.T) {
	bare := localRemote(t)

	cache, err := NewRemoteHeadCache(t.TempDir())
	if err != nil {
		t.Fatalf("new cache: %v", err)
	}
	client := NewClient(t.TempDir())
	repo := appcfg.Repository{Name: "repo", URL: bare, Branch: "master"}

	changed, sha, err := client.CheckRemoteChanged(cache, repo, "master")
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if !changed {
		t.Fatal("the first check did not see a change")
	}
	if sha == "" {
		t.Fatal("the first check found no commit")
	}

	changed, _, err = client.CheckRemoteChanged(cache, repo, "master")
	if err != nil {
		t.Fatalf("second check: %v", err)
	}
	if changed {
		t.Fatal("the second check said the remote moved, but it did not")
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

	if _, _, checkErr := client.CheckRemoteChanged(cache, repo, "master"); checkErr != nil {
		t.Fatalf("check: %v", checkErr)
	}

	// A different branch of the same remote is a different entry.
	cache.Set(repo.URL, "master", "0000000000000000000000000000000000000000")

	changed, _, err := client.CheckRemoteChanged(cache, repo, "master")
	if err != nil {
		t.Fatalf("second check: %v", err)
	}
	if !changed {
		t.Fatal("the check did not see the moved remote")
	}
}
