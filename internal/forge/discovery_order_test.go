package forge

import "testing"

// The name of a repository is its address on the site: the folder of
// the content, the key of the state, the address of the pages. The
// forges are asked at the same time, and the order of the answer is the
// order of the race between them. Two repositories that share a name
// would then swap their names from one run to the next, and the pages
// of one of them would move.
func TestTheNameOfARepositoryDoesNotDependOnTheOrderOfTheResult(t *testing.T) {
	ds := NewDiscoveryService(NewForgeManager(), nil)
	manager := NewForgeManager()

	first := &Repository{
		ID:       "1",
		Name:     "docs",
		FullName: "acme/docs",
		CloneURL: "https://forge-a.example.com/acme/docs.git",
		Metadata: map[string]string{"forge_name": "forge-a"},
	}
	second := &Repository{
		ID:       "2",
		Name:     "docs",
		FullName: "acme/docs",
		CloneURL: "https://forge-b.example.com/acme/docs.git",
		Metadata: map[string]string{"forge_name": "forge-b"},
	}

	names := map[string]string{}
	for _, repos := range [][]*Repository{{first, second}, {second, first}} {
		for _, repo := range ds.ConvertToConfigRepositories(repos, manager) {
			if before, ok := names[repo.URL]; ok && before != repo.Name {
				t.Fatalf("the repository %s got the name %q and later the name %q", repo.URL, before, repo.Name)
			}
			names[repo.URL] = repo.Name
		}
	}

	if len(names) != 2 {
		t.Fatalf("converted %d repositories, want 2", len(names))
	}
	if names[first.CloneURL] == names[second.CloneURL] {
		t.Fatalf("both repositories have the name %q", names[first.CloneURL])
	}
}
