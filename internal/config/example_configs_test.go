package config

import (
	"os"
	"path/filepath"
	"testing"
)

// The example configs are the first thing a new user copies. They must
// load: a missing version key or a stale setting makes them fail with a
// message that says nothing about what to fix.
func TestExampleConfigsLoad(t *testing.T) {
	root := "../.."
	files := []string{
		"config.example.yaml",
		"dev-config.yaml",
		"examples/configs/config-v2-test.yaml",
		"examples/configs/demo-config.yaml",
		"examples/configs/git-home-config.yaml",
		"examples/configs/relearn-config.yaml",
		"examples/configs/relearn-example.yaml",
		"test/config.test.yaml",
		"test/testdata/config.test.yaml",
	}
	for _, name := range files {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(root, name)
			if _, err := os.Stat(path); err != nil {
				t.Skipf("%s does not exist", name)
			}
			if _, _, err := LoadWithResult(path); err != nil {
				t.Errorf("%s does not load: %v", name, err)
			}
		})
	}
}
