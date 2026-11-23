package state

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// The state file is written when the state is saved, and not by every
// change: a build writes the state of every repository, and a write for
// each of them rewrote the whole file hundreds of times.
func TestTheStateFileIsWrittenWhenTheStateIsSaved(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()

	storeResult := NewJSONStore(dir)
	if storeResult.IsErr() {
		t.Fatalf("create the store: %v", storeResult.UnwrapErr())
	}
	store := storeResult.Unwrap()

	statePath := filepath.Join(dir, "daemon-state.json")

	repo := &Repository{URL: "https://example.com/repo.git", Name: "repo", Branch: "main"}
	if createResult := store.Repositories().Create(ctx, repo); createResult.IsErr() {
		t.Fatalf("create the repository: %v", createResult.UnwrapErr())
	}
	if hashResult := store.Repositories().SetDocFilesHash(ctx, repo.URL, "hash-1"); hashResult.IsErr() {
		t.Fatalf("set the hash: %v", hashResult.UnwrapErr())
	}

	// The changes are not on the disk yet.
	if _, err := os.Stat(statePath); !os.IsNotExist(err) {
		t.Fatalf("the file was written by the changes: %v", err)
	}

	// The save writes them.
	if flushResult := store.Flush(ctx); flushResult.IsErr() {
		t.Fatalf("save the state: %v", flushResult.UnwrapErr())
	}
	if _, err := os.Stat(statePath); err != nil {
		t.Fatalf("the file was not written by the save: %v", err)
	}

	// The value is in the file.
	reopened := NewJSONStore(dir)
	if reopened.IsErr() {
		t.Fatalf("open the store: %v", reopened.UnwrapErr())
	}
	stored := reopened.Unwrap().Repositories().GetByURL(ctx, repo.URL)
	if stored.IsErr() || stored.Unwrap().IsNone() {
		t.Fatalf("the repository was not written: %v", stored.UnwrapErr())
	}
	if hash := stored.Unwrap().Unwrap().DocFilesHash; hash.IsNone() || hash.Unwrap() != "hash-1" {
		t.Fatalf("the hash was not written: %v", hash)
	}
}
