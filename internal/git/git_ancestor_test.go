package git

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	helpers "git.home.luguber.info/inful/docbuilder/internal/testutil/testutils"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// helper to add a file and commit returning hash.
func addCommit(t *testing.T, repo *git.Repository, repoPath, filename, content, msg string) plumbing.Hash {
	t.Helper()
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatalf("worktree: %v", err)
	}
	full := filepath.Join(repoPath, filename)
	if writeFileErr := os.WriteFile(full, []byte(content), 0o600); writeFileErr != nil {
		t.Fatalf("write file: %v", writeFileErr)
	}
	if _, addErr := wt.Add(filename); addErr != nil {
		t.Fatalf("add: %v", addErr)
	}
	hash, err := wt.Commit(msg, &git.CommitOptions{Author: &object.Signature{Name: "tester", Email: "t@example.com", When: time.Now()}})
	if err != nil {
		t.Fatalf("commit: %v", err)
	}
	return hash
}

func TestIsAncestorEdgeCases(t *testing.T) {
	repo, _, tmp := helpers.SetupTestGitRepo(t)

	// Create a simple linear history: A -> B -> C
	a := addCommit(t, repo, tmp, "a.txt", "A", "A")
	b := addCommit(t, repo, tmp, "b.txt", "B", "B")
	c := addCommit(t, repo, tmp, "c.txt", "C", "C")

	// 1. Identical commit hash should be ancestor of itself
	same, err := isAncestor(repo, b, b)
	if err != nil || !same {
		t.Fatalf("expected identical hash ancestor true, got %v err=%v", same, err)
	}

	// 2. A is ancestor of C
	res, err := isAncestor(repo, a, c)
	if err != nil || !res {
		t.Fatalf("expected A ancestor of C: res=%v err=%v", res, err)
	}

	// 3. C is not ancestor of A
	res, err = isAncestor(repo, c, a)
	if err != nil {
		t.Fatalf("unexpected error reverse direction: %v", err)
	}
	if res {
		t.Fatalf("expected C not ancestor of A")
	}

	// 4. Nonexistent 'a' hash should return false without error (current behavior)
	missingA := plumbing.NewHash(strings.Repeat("1", 40))
	res, err = isAncestor(repo, missingA, c)
	if err != nil {
		t.Fatalf("unexpected error for missing ancestor hash: %v", err)
	}
	if res {
		t.Fatalf("expected false for missing ancestor hash")
	}

	// 5. Nonexistent 'b' hash should produce an error (can't load starting commit)
	missingB := plumbing.NewHash(strings.Repeat("2", 40))
	_, err = isAncestor(repo, a, missingB)
	if err == nil {
		t.Fatalf("expected error for nonexistent starting commit b")
	}

	// 6. Identical nonexistent hash returns true (locks current semantics of early equality)
	nonExist := plumbing.NewHash(strings.Repeat("3", 40))
	res, err = isAncestor(repo, nonExist, nonExist)
	if !res || err != nil {
		t.Fatalf("expected true (early equality) for identical nonexistent hash, got res=%v err=%v", res, err)
	}
}
