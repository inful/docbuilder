package config

import "testing"

// The hash of the configuration decides whether the daemon rebuilds the
// site. The namespace and the tags of a repository decide the path of
// every page of that repository, and the limit of an asset decides
// whether the asset is copied at all.
func TestTheSnapshotHoldsWhatDecidesThePathsOfThePages(t *testing.T) {
	base := &Config{}
	base.Repositories = []Repository{{Name: "repo", URL: "https://example.com/repo.git"}}
	base.Build.MaxAssetSize = 1024

	changes := map[string]func(*Config){
		"repository.namespace": func(c *Config) {
			c.Repositories = []Repository{{Name: "repo", URL: "https://example.com/repo.git", Namespace: "myforge"}}
		},
		"repository.tags": func(c *Config) {
			c.Repositories = []Repository{{Name: "repo", URL: "https://example.com/repo.git", Tags: map[string]string{"forge_type": "myforge"}}}
		},
		"build.max_asset_size": func(c *Config) {
			c.Build.MaxAssetSize = 2048
		},
	}

	baseHash := base.Snapshot()

	for name, change := range changes {
		changed := &Config{}
		changed.Repositories = append([]Repository{}, base.Repositories...)
		changed.Build = base.Build
		change(changed)
		if changed.Snapshot() == baseHash {
			t.Errorf("the snapshot did not change with %s", name)
		}
	}
}
