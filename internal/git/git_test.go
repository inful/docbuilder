package git

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	appcfg "git.home.luguber.info/inful/docbuilder/internal/config"
)

// TestPruneNonDocTopLevelNormalization ensures that various path forms (./docs, /docs, docs/, docs/api, docs\\api)
// all preserve the same top-level directory when pruning. It also verifies that unrelated top-level directories
// are removed while allowed ones remain.
func TestPruneNonDocTopLevelNormalization(t *testing.T) {
	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "repo")
	if err := os.MkdirAll(repoPath, 0o750); err != nil {
		t.Fatalf("mkdir repo: %v", err)
	}

	// Create top-level directories that should be preserved or removed
	keepDirs := []string{"docs", "documentation"}
	removeDirs := []string{"cmd", "pkg", "scripts", "misc"}
	for _, d := range append(keepDirs, removeDirs...) {
		if err := os.MkdirAll(filepath.Join(repoPath, d), 0o750); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}
	// Add a file at root to ensure non-directory entries are also handled gracefully.
	if err := os.WriteFile(filepath.Join(repoPath, "README.md"), []byte("test"), 0o600); err != nil {
		t.Fatalf("write README: %v", err)
	}

	repo := appcfg.Repository{
		URL:   "https://example.com/repo.git",
		Name:  "repo",
		Paths: []string{"docs", "./docs", "/docs", "docs/", "docs/api", "documentation/guides", "documentation\\windows"},
	}

	client := NewClient(tmpDir).WithBuildConfig(&appcfg.BuildConfig{PruneNonDocPaths: true})
	if err := client.pruneNonDocTopLevel(repoPath, repo); err != nil {
		t.Fatalf("prune: %v", err)
	}

	// Expected survivors: .git (not created here), docs, documentation
	for _, d := range keepDirs {
		if _, err := os.Stat(filepath.Join(repoPath, d)); err != nil {
			t.Errorf("expected directory %s to remain, got error: %v", d, err)
		}
	}
	for _, d := range removeDirs {
		if _, err := os.Stat(filepath.Join(repoPath, d)); !os.IsNotExist(err) {
			t.Errorf("expected directory %s to be removed, stat err=%v", d, err)
		}
	}
	// README.md should also be removed because it's a top-level file not in allowed set
	if _, err := os.Stat(filepath.Join(repoPath, "README.md")); !os.IsNotExist(err) {
		t.Errorf("expected README.md to be removed, stat err=%v", err)
	}
}

// TestPruneAllowDeny ensures that global allow and deny lists interact correctly with pruning.
func TestPruneAllowDeny(t *testing.T) {
	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "repo2")
	if err := os.MkdirAll(repoPath, 0o750); err != nil {
		t.Fatalf("mkdir repo2: %v", err)
	}

	// Create directories and files
	toMake := []string{"docs", "assets", "examples", "legacy", "LICENSE"}
	for _, n := range toMake {
		path := filepath.Join(repoPath, n)
		if strings.Contains(n, ".") { // treat as file
			if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
				t.Fatalf("write %s: %v", n, err)
			}
		} else {
			if err := os.MkdirAll(path, 0o750); err != nil {
				t.Fatalf("mkdir %s: %v", n, err)
			}
		}
	}

	repo := appcfg.Repository{URL: "https://example.com/repo2.git", Name: "repo2", Paths: []string{"docs"}}
	cfg := &appcfg.BuildConfig{PruneNonDocPaths: true, PruneAllow: []string{"assets", "LICENSE"}, PruneDeny: []string{"legacy"}}
	client := NewClient(tmpDir).WithBuildConfig(cfg)
	if err := client.pruneNonDocTopLevel(repoPath, repo); err != nil {
		t.Fatalf("prune: %v", err)
	}

	// Expect docs, assets, LICENSE to remain; legacy and examples removed (examples not allowed; legacy denied)
	survivors := []string{"docs", "assets", "LICENSE"}
	removed := []string{"legacy", "examples"}
	for _, s := range survivors {
		if _, err := os.Stat(filepath.Join(repoPath, s)); err != nil {
			t.Errorf("expected %s to survive: %v", s, err)
		}
	}
	for _, r := range removed {
		if _, err := os.Stat(filepath.Join(repoPath, r)); !os.IsNotExist(err) {
			t.Errorf("expected %s removed, stat err=%v", r, err)
		}
	}
}

// TestPruneAllowDenyGlobs ensures glob patterns in allow/deny lists work.
func TestPruneAllowDenyGlobs(t *testing.T) {
	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "repo3")
	if err := os.MkdirAll(repoPath, 0o750); err != nil {
		t.Fatalf("mkdir repo3: %v", err)
	}

	entries := []string{"docs", "LICENSE", "LICENSE.md", "README.md", "README.old", "CHANGELOG", "tmp", "cache"}
	for _, n := range entries {
		path := filepath.Join(repoPath, n)
		if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
			t.Fatalf("write %s: %v", n, err)
		}
	}

	repo := appcfg.Repository{URL: "https://example.com/repo3.git", Name: "repo3", Paths: []string{"docs"}}
	cfg := &appcfg.BuildConfig{PruneNonDocPaths: true,
		PruneAllow: []string{"LICENSE*", "README.*"},
		PruneDeny:  []string{"README.old", "CHANGELOG", "tmp", "cache"},
	}
	client := NewClient(tmpDir).WithBuildConfig(cfg)
	if err := client.pruneNonDocTopLevel(repoPath, repo); err != nil {
		t.Fatalf("prune: %v", err)
	}

	// Survivors: docs (doc root), LICENSE, LICENSE.md, README.md
	survivors := []string{"docs", "LICENSE", "LICENSE.md", "README.md"}
	removed := []string{"README.old", "CHANGELOG", "tmp", "cache"}
	for _, s := range survivors {
		if _, err := os.Stat(filepath.Join(repoPath, s)); err != nil {
			t.Errorf("expected %s to survive: %v", s, err)
		}
	}
	for _, r := range removed {
		if _, err := os.Stat(filepath.Join(repoPath, r)); !os.IsNotExist(err) {
			t.Errorf("expected %s removed, err=%v", r, err)
		}
	}
}
