package docs

import (
	"os"
	"path/filepath"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// A README and an index in the same directory are written to the same
// page: the pipeline renames the README to _index. The discovery has to
// see it and refuse the build, or one of the two pages is lost.
func TestDiscoveryRefusesTwoFilesForTheSamePage(t *testing.T) {
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

	_, err := discovery.DiscoverDocs(map[string]string{testRepoName: repoDir})
	if err == nil {
		t.Fatal("the discovery did not refuse the two files of the same page")
	}
}
