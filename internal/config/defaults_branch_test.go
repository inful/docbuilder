package config

import "testing"

// The branch of a repository is not defaulted: an empty branch means
// the branch the remote points at, and the default branch of a
// repository is not always "main".
func TestTheBranchOfARepositoryIsNotDefaulted(t *testing.T) {
	cfg := &Config{
		Repositories: []Repository{
			{Name: "first", URL: "https://example.com/first.git"},
			{Name: "second", URL: "https://example.com/second.git", Branch: "trunk"},
		},
	}

	if err := (&RepositoryDefaultApplier{}).ApplyDefaults(cfg); err != nil {
		t.Fatalf("apply the defaults: %v", err)
	}

	if cfg.Repositories[0].Branch != "" {
		t.Errorf("the branch of the first repository is %q, want none", cfg.Repositories[0].Branch)
	}
	if cfg.Repositories[1].Branch != "trunk" {
		t.Errorf("the branch of the second repository is %q", cfg.Repositories[1].Branch)
	}
}
