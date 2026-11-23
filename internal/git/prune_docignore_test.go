package git

import (
	"os"
	"path/filepath"
	"testing"

	appcfg "git.home.luguber.info/inful/docbuilder/internal/config"
)

func writeTreeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// The .docignore of a repository is read from the clone: it is the file
// that keeps a repository out of the site. Pruning it published the
// repository the operator excluded.
func TestPruneNonDocTopLevelKeepsTheDocIgnore(t *testing.T) {
	repoPath := t.TempDir()
	writeTreeFile(t, filepath.Join(repoPath, ".docignore"), "docs\n")
	writeTreeFile(t, filepath.Join(repoPath, ".gitignore"), "*.tmp\n")
	writeTreeFile(t, filepath.Join(repoPath, "docs", "index.md"), "# Index\n")
	writeTreeFile(t, filepath.Join(repoPath, "src", "main.go"), "package main\n")

	client := NewClient(t.TempDir()).WithBuildConfig(&appcfg.BuildConfig{PruneNonDocPaths: true})
	repo := appcfg.Repository{Name: "repo", Paths: []string{"docs"}}

	if err := client.pruneNonDocTopLevel(repoPath, repo); err != nil {
		t.Fatalf("prune: %v", err)
	}

	if _, err := os.Stat(filepath.Join(repoPath, ".docignore")); err != nil {
		t.Fatalf("the .docignore was pruned: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repoPath, "src")); err == nil {
		t.Fatal("the source directory was not pruned")
	}
	if _, err := os.Stat(filepath.Join(repoPath, "docs", "index.md")); err != nil {
		t.Fatalf("the documentation was pruned: %v", err)
	}
}
