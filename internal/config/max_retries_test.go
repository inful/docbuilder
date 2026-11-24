package config

import (
	"os"
	"path/filepath"
	"testing"
)

// An operator who writes "max_retries: 0" means "no retries". The field
// had no memory of being written, so the default turned every zero
// into two.
func TestNoRetriesIsNoRetries(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := `version: "2.0"
repositories:
  - name: repo
    url: https://example.com/repo.git
build:
  max_retries: 0
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write the config: %v", err)
	}

	_, cfg, err := LoadWithResult(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Build.MaxRetries != 0 {
		t.Fatalf("the retries are %d, want 0", cfg.Build.MaxRetries)
	}
}

// Without the field the default holds.
func TestTheDefaultOfTwoRetriesHolds(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := `version: "2.0"
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
	if cfg.Build.MaxRetries != 2 {
		t.Fatalf("the retries are %d, want 2", cfg.Build.MaxRetries)
	}
}
