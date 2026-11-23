package docs

import (
	"os"
	"path/filepath"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// Two files that land on the same page are refused.
//
// An index.md and an _index.md are the same page for Hugo, and one of
// them is lost.
func TestDiscoveryRefusesTwoFilesForTheSamePage(t *testing.T) {
	repoDir := t.TempDir()
	docsDir := filepath.Join(repoDir, "docs")
	if err := os.MkdirAll(docsDir, 0o750); err != nil {
		t.Fatalf("make the docs dir: %v", err)
	}
	for _, name := range []string{"index.md", "_index.md"} {
		if err := os.WriteFile(filepath.Join(docsDir, name), []byte("# Page\n"), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	repos := []config.Repository{{
		Name:  testRepoName,
		Paths: []string{testDocsDir},
	}}

	discovery := NewDiscovery(repos, &config.BuildConfig{NamespaceForges: config.NamespacingNever})

	_, err := discovery.DiscoverDocs(map[string]string{testRepoName: repoDir})
	if err == nil {
		t.Fatal("the discovery did not refuse the two files of the same page")
	}
}

// A README next to an index is a page of its own, and not a
// collision.
//
// A repository with a README and an index in the same directory is a
// common layout: the pipeline keeps the name of the README, and the
// build failed on it.
func TestDiscoveryAcceptsAReadmeNextToAnIndex(t *testing.T) {
	repoDir := t.TempDir()
	docsDir := filepath.Join(repoDir, "docs")
	if err := os.MkdirAll(docsDir, 0o750); err != nil {
		t.Fatalf("make the docs dir: %v", err)
	}
	for _, name := range []string{"README.md", "index.md"} {
		if err := os.WriteFile(filepath.Join(docsDir, name), []byte("# Page\n"), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	repos := []config.Repository{{
		Name:  testRepoName,
		Paths: []string{testDocsDir},
	}}

	discovery := NewDiscovery(repos, &config.BuildConfig{NamespaceForges: config.NamespacingNever})

	files, err := discovery.DiscoverDocs(map[string]string{testRepoName: repoDir})
	if err != nil {
		t.Fatalf("the discovery refused a readme next to an index: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("the discovery found %d files, want 2", len(files))
	}
}
