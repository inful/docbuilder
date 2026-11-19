package forge

import "testing"

// Two repositories can share a name: the same project name in two
// GitLab subgroups, the same name on two forges. The build keys every
// repository by its name - the clone folder, the state map, the content
// path - so two repos with one name eat each other and one of them
// vanishes from the site without a word.
func TestConvertToConfigRepositories_GivesEveryRepositoryItsOwnName(t *testing.T) {
	ds := NewDiscoveryService(NewForgeManager(), nil)
	manager := NewForgeManager()

	repos := []*Repository{
		{ID: "1", Name: "docs", FullName: "platform/team-a/docs", CloneURL: "https://gitlab.example.com/platform/team-a/docs.git"},
		{ID: "2", Name: "docs", FullName: "platform/team-b/docs", CloneURL: "https://gitlab.example.com/platform/team-b/docs.git"},
	}

	converted := ds.ConvertToConfigRepositories(repos, manager)
	if len(converted) != 2 {
		t.Fatalf("converted %d repositories, want 2", len(converted))
	}

	if converted[0].Name != "docs" {
		t.Errorf("first name = %q, want %q", converted[0].Name, "docs")
	}
	if converted[1].Name == converted[0].Name {
		t.Fatalf("both repositories have the name %q", converted[0].Name)
	}
	if converted[1].Name != "platform-team-b-docs" {
		t.Errorf("second name = %q, want %q", converted[1].Name, "platform-team-b-docs")
	}
}

// The name must also be safe as a folder name: it becomes the clone
// folder and a path in the site.
func TestConvertToConfigRepositories_DuplicateNamesStayPathSegments(t *testing.T) {
	ds := NewDiscoveryService(NewForgeManager(), nil)
	manager := NewForgeManager()

	repos := []*Repository{
		{ID: "1", Name: "docs", FullName: "team-a/docs", CloneURL: "https://gitlab.example.com/team-a/docs.git"},
		{ID: "2", Name: "docs", FullName: "team-b/docs", CloneURL: "https://gitlab.example.com/team-b/docs.git"},
		{ID: "3", Name: "docs", FullName: "team-c/docs", CloneURL: "https://gitlab.example.com/team-c/docs.git"},
	}

	converted := ds.ConvertToConfigRepositories(repos, manager)
	seen := make(map[string]struct{}, len(converted))
	for i := range converted {
		name := converted[i].Name
		if _, taken := seen[name]; taken {
			t.Errorf("repository %d reuses the name %q", i, name)
		}
		seen[name] = struct{}{}
		if name == "" || name == "." || name == ".." {
			t.Errorf("repository %d has the unusable name %q", i, name)
		}
	}
}
