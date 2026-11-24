package config

import "testing"

// The versioning decides which pages the site holds. A change of it
// has to rebuild the site: the hash of the configuration is what the
// daemon looks at.
func TestTheSnapshotHoldsTheVersioning(t *testing.T) {
	base := &Config{}
	base.Versioning = &VersioningConfig{Enabled: false, DefaultBranchOnly: false}

	changes := map[string]func(*Config){
		"versioning.enabled":             func(c *Config) { c.Versioning.Enabled = true },
		"versioning.default_branch_only": func(c *Config) { c.Versioning.DefaultBranchOnly = true },
	}

	baseHash := base.Snapshot()

	for name, change := range changes {
		changed := &Config{Versioning: &VersioningConfig{}}
		change(changed)
		if changed.Snapshot() == baseHash {
			t.Errorf("the snapshot did not change with %s", name)
		}
	}
}
