package git

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The address of a repository can hold a token, and the cache is
// written to disk: the token of the repository was in the state file
// for the life of the install.
func TestTheCacheOfTheRemoteHeadHoldsNoToken(t *testing.T) {
	dir := t.TempDir()

	cache, err := NewRemoteHeadCache(dir)
	if err != nil {
		t.Fatalf("new cache: %v", err)
	}

	//nolint:gosec // the token of the test is not a credential
	url := "https://x-access-token:secret-token@example.invalid/org/repo.git"
	cache.Set(url, "main", "0123456789abcdef")

	if saveErr := cache.Save(); saveErr != nil {
		t.Fatalf("save: %v", saveErr)
	}

	// #nosec G304 -- the path is the file of the cache of the test.
	data, err := os.ReadFile(filepath.Join(dir, "remote-heads.json"))
	if err != nil {
		t.Fatalf("read the cache: %v", err)
	}
	if got := string(data); strings.Contains(got, "secret-token") {
		t.Fatalf("the token of the repository is in the cache: %s", got)
	}

	// The entry is still found: the address without the token is the
	// key of the entry.
	entry := cache.Get(url, "main")
	if entry == nil {
		t.Fatal("the entry of the repository is not in the cache")
	}
	if entry.CommitSHA != "0123456789abcdef" {
		t.Fatalf("the entry holds %s", entry.CommitSHA)
	}
}
