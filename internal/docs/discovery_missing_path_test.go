package docs

import (
	"os"
	"path/filepath"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// A repository whose documentation path is gone has lost its
// documentation. The build has to fail: the site that is published now
// would have the pages of that repository missing, and it would replace
// the site that still has them.
func TestDiscoveryRefusesARepositoryWithoutItsDocumentationPath(t *testing.T) {
	repoDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoDir, "documentation"), 0o750); err != nil {
		t.Fatalf("make the documentation dir: %v", err)
	}

	repos := []config.Repository{{
		Name:  testRepoName,
		Paths: []string{testDocsDir},
	}}

	discovery := NewDiscovery(repos, &config.BuildConfig{NamespaceForges: config.NamespacingNever})

	_, err := discovery.DiscoverDocs(map[string]string{testRepoName: repoDir})
	if err == nil {
		t.Fatal("the discovery did not refuse the repository without its documentation")
	}
}
