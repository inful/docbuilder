package forge

import (
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// A repository whose name is refused is not in the list.
//
// The name of a repository is the name of the directory the clone goes
// to: a repository without a name was cloned into the root of the
// workspace, and the removal of the old clone deleted every other
// clone.
func TestARepositoryWithoutANameIsSkipped(t *testing.T) {
	repos := []*Repository{
		{ID: "1", Name: "..", CloneURL: "https://example.com/a.git", FullName: "a"},
		{ID: "2", Name: "good", CloneURL: "https://example.com/b.git", FullName: "b"},
	}

	ds := NewDiscoveryService(NewForgeManager(), nil)
	configured := ds.ConvertToConfigRepositories(repos, NewForgeManager())

	for _, repo := range configured {
		if repo.Name == "" {
			t.Fatalf("the repository without a name is in the list: %+v", configured)
		}
	}
	if len(configured) != 1 {
		t.Fatalf("the list has %d repositories, want 1: %+v", len(configured), configured)
	}
	if configured[0].Name != "good" {
		t.Fatalf("the repository in the list is %q", configured[0].Name)
	}
}

// The name of a repository that is not a single path segment is
// refused.
func TestARepositoryWithAPathInItsNameIsRefused(t *testing.T) {
	repos := []*Repository{
		{ID: "1", Name: "a/b", CloneURL: "https://example.com/a.git", FullName: "a/b"},
	}

	ds := NewDiscoveryService(NewForgeManager(), nil)
	configured := ds.ConvertToConfigRepositories(repos, NewForgeManager())

	if len(configured) != 0 {
		t.Fatalf("the repository with a path in its name is in the list: %+v", configured)
	}
	_ = config.ForgeGitHub
}
