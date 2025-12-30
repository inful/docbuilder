package git

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

func TestComputeRepoHashConsistency(t *testing.T) {
	// Create a temporary repository
	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "test-repo")

	repo, err := git.PlainInit(repoPath, false)
	if err != nil {
		t.Fatalf("Failed to init repo: %v", err)
	}

	// Create test files
	testFile := filepath.Join(repoPath, "test.txt")
	if writeFileErr := os.WriteFile(testFile, []byte("test content"), 0600); writeFileErr != nil {
		t.Fatalf("Failed to write file: %v", writeFileErr)
	}

	// Create docs directory
	docsDir := filepath.Join(repoPath, "docs")
	if mkdirErr := os.MkdirAll(docsDir, 0o750); mkdirErr != nil {
		t.Fatalf("Failed to create docs dir: %v", mkdirErr)
	}

	docsFile := filepath.Join(docsDir, "readme.md")
	if writeFileErr := os.WriteFile(docsFile, []byte("# Documentation"), 0600); writeFileErr != nil {
		t.Fatalf("Failed to write docs file: %v", writeFileErr)
	}

	// Stage and commit
	w, err := repo.Worktree()
	if err != nil {
		t.Fatalf("Failed to get worktree: %v", err)
	}

	if _, addErr := w.Add("."); addErr != nil {
		t.Fatalf("Failed to add files: %v", addErr)
	}

	commit, err := w.Commit("Initial commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test User",
			Email: "test@example.com",
		},
	})
	if err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}

	// Compute hash twice - should be identical
	hash1, err := ComputeRepoHash(repoPath, commit.String(), nil)
	if err != nil {
		t.Fatalf("ComputeRepoHash failed: %v", err)
	}

	hash2, err := ComputeRepoHash(repoPath, commit.String(), nil)
	if err != nil {
		t.Fatalf("ComputeRepoHash failed: %v", err)
	}

	if hash1 != hash2 {
		t.Errorf("Hash not consistent: %s != %s", hash1, hash2)
	}

	if len(hash1) != 64 {
		t.Errorf("Expected 64-char SHA256 hash, got %d chars", len(hash1))
	}
}

func TestComputeRepoHashWithPaths(t *testing.T) {
	// Create a temporary repository
	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "test-repo")

	repo, err := git.PlainInit(repoPath, false)
	if err != nil {
		t.Fatalf("Failed to init repo: %v", err)
	}

	// Create multiple directories
	dirs := []string{"docs", "src", "config"}
	for _, dir := range dirs {
		dirPath := filepath.Join(repoPath, dir)
		if mkdirErr := os.MkdirAll(dirPath, 0o750); mkdirErr != nil {
			t.Fatalf("Failed to create dir %s: %v", dir, mkdirErr)
		}

		file := filepath.Join(dirPath, "file.txt")
		if writeFileErr := os.WriteFile(file, []byte(dir+" content"), 0600); writeFileErr != nil {
			t.Fatalf("Failed to write file: %v", writeFileErr)
		}
	}

	w, err := repo.Worktree()
	if err != nil {
		t.Fatalf("Failed to get worktree: %v", err)
	}

	if _, addErr := w.Add("."); addErr != nil {
		t.Fatalf("Failed to add files: %v", addErr)
	}

	commit, err := w.Commit("Add directories", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test User",
			Email: "test@example.com",
		},
	})
	if err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}

	// Hash entire repo
	hashAll, err := ComputeRepoHash(repoPath, commit.String(), nil)
	if err != nil {
		t.Fatalf("ComputeRepoHash failed: %v", err)
	}

	// Hash only docs
	hashDocs, err := ComputeRepoHash(repoPath, commit.String(), []string{"docs"})
	if err != nil {
		t.Fatalf("ComputeRepoHash(docs) failed: %v", err)
	}

	// Hash only src
	hashSrc, err := ComputeRepoHash(repoPath, commit.String(), []string{"src"})
	if err != nil {
		t.Fatalf("ComputeRepoHash(src) failed: %v", err)
	}

	// All should be different
	if hashAll == hashDocs {
		t.Error("Full repo hash should differ from docs-only hash")
	}

	if hashAll == hashSrc {
		t.Error("Full repo hash should differ from src-only hash")
	}

	if hashDocs == hashSrc {
		t.Error("Docs hash should differ from src hash")
	}
}

func TestComputeRepoHashChangesWithContent(t *testing.T) {
	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "test-repo")

	repo, err := git.PlainInit(repoPath, false)
	if err != nil {
		t.Fatalf("Failed to init repo: %v", err)
	}

	testFile := filepath.Join(repoPath, "test.txt")
	if writeFileErr := os.WriteFile(testFile, []byte("version 1"), 0600); writeFileErr != nil {
		t.Fatalf("Failed to write file: %v", writeFileErr)
	}

	w, err := repo.Worktree()
	if err != nil {
		t.Fatalf("Failed to get worktree: %v", err)
	}

	if _, addErr := w.Add("."); addErr != nil {
		t.Fatalf("Failed to add files: %v", addErr)
	}

	commit1, err := w.Commit("Version 1", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test User",
			Email: "test@example.com",
		},
	})
	if err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}

	hash1, err := ComputeRepoHash(repoPath, commit1.String(), nil)
	if err != nil {
		t.Fatalf("ComputeRepoHash failed: %v", err)
	}

	// Modify file and commit again
	if writeFileErr := os.WriteFile(testFile, []byte("version 2"), 0600); writeFileErr != nil {
		t.Fatalf("Failed to write file: %v", writeFileErr)
	}

	if _, addErr := w.Add("."); addErr != nil {
		t.Fatalf("Failed to add files: %v", addErr)
	}

	commit2, err := w.Commit("Version 2", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test User",
			Email: "test@example.com",
		},
	})
	if err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}

	hash2, err := ComputeRepoHash(repoPath, commit2.String(), nil)
	if err != nil {
		t.Fatalf("ComputeRepoHash failed: %v", err)
	}

	if hash1 == hash2 {
		t.Error("Hash should change when content changes")
	}
}

func TestComputeRepoHashFromWorkdir(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test structure
	docsDir := filepath.Join(tmpDir, "docs")
	if err := os.MkdirAll(docsDir, 0o750); err != nil {
		t.Fatalf("Failed to create docs dir: %v", err)
	}

	file1 := filepath.Join(docsDir, "readme.md")
	if err := os.WriteFile(file1, []byte("# Documentation"), 0600); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	file2 := filepath.Join(docsDir, "guide.md")
	if err := os.WriteFile(file2, []byte("# Guide"), 0600); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	// Compute hash
	hash, err := ComputeRepoHashFromWorkdir(tmpDir, []string{"docs"})
	if err != nil {
		t.Fatalf("ComputeRepoHashFromWorkdir failed: %v", err)
	}

	if len(hash) != 64 {
		t.Errorf("Expected 64-char hash, got %d", len(hash))
	}

	// Should be deterministic
	hash2, err := ComputeRepoHashFromWorkdir(tmpDir, []string{"docs"})
	if err != nil {
		t.Fatalf("ComputeRepoHashFromWorkdir failed: %v", err)
	}

	if hash != hash2 {
		t.Error("Hash not deterministic")
	}
}

func TestGetRepoTree(t *testing.T) {
	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "test-repo")

	repo, err := git.PlainInit(repoPath, false)
	if err != nil {
		t.Fatalf("Failed to init repo: %v", err)
	}

	testFile := filepath.Join(repoPath, "test.txt")
	if writeFileErr := os.WriteFile(testFile, []byte("test"), 0600); writeFileErr != nil {
		t.Fatalf("Failed to write file: %v", writeFileErr)
	}

	w, err := repo.Worktree()
	if err != nil {
		t.Fatalf("Failed to get worktree: %v", err)
	}

	if _, addErr := w.Add("."); addErr != nil {
		t.Fatalf("Failed to add files: %v", addErr)
	}

	commit, err := w.Commit("Test", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test User",
			Email: "test@example.com",
		},
	})
	if err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}

	paths := []string{"docs"}
	tree, err := GetRepoTree(repoPath, commit.String(), paths)
	if err != nil {
		t.Fatalf("GetRepoTree failed: %v", err)
	}

	if tree.RepoPath != repoPath {
		t.Errorf("RepoPath mismatch: got %s, want %s", tree.RepoPath, repoPath)
	}

	if tree.Commit != commit.String() {
		t.Errorf("Commit mismatch: got %s, want %s", tree.Commit, commit.String())
	}

	if len(tree.Paths) != len(paths) {
		t.Errorf("Paths mismatch: got %d, want %d", len(tree.Paths), len(paths))
	}

	if tree.Hash == "" {
		t.Error("Hash should not be empty")
	}
}

func TestComputeRepoHashNonexistentPath(t *testing.T) {
	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "test-repo")

	repo, err := git.PlainInit(repoPath, false)
	if err != nil {
		t.Fatalf("Failed to init repo: %v", err)
	}

	testFile := filepath.Join(repoPath, "test.txt")
	if writeErr := os.WriteFile(testFile, []byte("test"), 0600); writeErr != nil {
		t.Fatalf("Failed to write file: %v", writeErr)
	}

	w, err := repo.Worktree()
	if err != nil {
		t.Fatalf("Failed to get worktree: %v", err)
	}

	if _, addErr := w.Add("."); addErr != nil {
		t.Fatalf("Failed to add files: %v", addErr)
	}

	commit, err := w.Commit("Test", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test User",
			Email: "test@example.com",
		},
	})
	if err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}

	// Try to hash a nonexistent path - should not error, just skip it
	hash, err := ComputeRepoHash(repoPath, commit.String(), []string{"nonexistent"})
	if err != nil {
		t.Fatalf("Should not error on nonexistent path: %v", err)
	}

	// Hash should still be computed (just based on commit)
	if len(hash) != 64 {
		t.Errorf("Expected 64-char hash, got %d", len(hash))
	}
}
