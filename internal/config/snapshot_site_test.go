package config

import (
	"testing"
)

// The hash of the configuration decides whether the daemon rebuilds the
// site. Everything that changes the site has to be part of it: the Hugo
// settings and the repository list.
func TestSnapshotHoldsTheHugoSettingsThatChangeTheSite(t *testing.T) {
	base := &Config{
		Hugo: HugoConfig{
			BaseURL: "http://localhost/",
			Title:   "Site",
		},
	}

	changes := map[string]func(*Config){
		"hugo.description":             func(c *Config) { c.Hugo.Description = "The site" },
		"hugo.custom_css":              func(c *Config) { c.Hugo.CustomCSS = "body { color: red; }" },
		"hugo.chrome":                  func(c *Config) { c.Hugo.Chrome = "shared" },
		"hugo.enable_page_transitions": func(c *Config) { c.Hugo.EnablePageTransitions = true },
		"hugo.params":                  func(c *Config) { c.Hugo.Params = map[string]any{"version": "2.0"} },
		"hugo.taxonomies":              func(c *Config) { c.Hugo.Taxonomies = map[string]string{"category": "categories"} },
		"hugo.menu":                    func(c *Config) { c.Hugo.Menu = map[string][]Menu{"main": {{Name: "Docs", URL: "/docs/"}}} },
		"repository.url":               func(c *Config) { c.Repositories = []Repository{{Name: "repo", URL: "https://example.com/repo.git"}} },
		"repository.branch": func(c *Config) {
			c.Repositories = []Repository{{Name: "repo", URL: "https://example.com/repo.git", Branch: "main"}}
		},
		"repository.paths": func(c *Config) {
			c.Repositories = []Repository{{Name: "repo", URL: "https://example.com/repo.git", Paths: []string{"docs"}}}
		},
		"repository.auth": func(c *Config) {
			c.Repositories = []Repository{{Name: "repo", URL: "https://example.com/repo.git", Auth: &AuthConfig{Type: AuthTypeToken, Token: "secret"}}}
		},
	}

	baseHash := base.Snapshot()
	if baseHash == "" {
		t.Fatal("the snapshot is empty")
	}

	for name, change := range changes {
		changed := &Config{
			Hugo:         base.Hugo,
			Repositories: base.Repositories,
		}
		change(changed)
		if changed.Snapshot() == baseHash {
			t.Errorf("the snapshot did not change with %s", name)
		}
	}
}

// The order of the repositories does not matter: the site sorts them by
// name.
func TestSnapshotTheOrderOfTheRepositoriesDoesNotMatter(t *testing.T) {
	first := &Config{Repositories: []Repository{{Name: "a"}, {Name: "b"}}}
	second := &Config{Repositories: []Repository{{Name: "b"}, {Name: "a"}}}

	if first.Snapshot() != second.Snapshot() {
		t.Fatal("the order of the repositories changed the snapshot")
	}
}
