package config

import (
	"os"
	"path/filepath"
	"testing"
)

// An operator who turns versioning off means it. The strategy alone
// does not turn it on: the configuration of `docbuilder init` holds
// both, and the versioning was on anyway.
func TestVersioningThatIsTurnedOffStaysOff(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := `version: "2.0"
versioning:
  enabled: false
  strategy: branches_and_tags
repositories:
  - name: repo
    url: https://example.com/repo.git
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write the config: %v", err)
	}

	_, cfg, err := LoadWithResult(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Versioning == nil {
		t.Fatal("the versioning is missing")
	}
	if cfg.Versioning.Enabled {
		t.Fatal("the versioning was turned on although the operator turned it off")
	}
}

// Without the field the strategy turns the versioning on: the operator
// asked for a strategy, so they want the versioning.
func TestVersioningWithAStrategyAndWithoutTheFieldIsOn(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := `version: "2.0"
versioning:
  strategy: branches_and_tags
repositories:
  - name: repo
    url: https://example.com/repo.git
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write the config: %v", err)
	}

	_, cfg, err := LoadWithResult(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !cfg.Versioning.Enabled {
		t.Fatal("the versioning was not turned on although a strategy was given")
	}
}
