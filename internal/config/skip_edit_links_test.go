package config

import (
	"os"
	"path/filepath"
	"testing"
)

// The operator can turn the verification of the edit links on. The
// field was a plain bool and the default turned every false into a
// true, so the documented "skip_edit_links: false" did nothing.
func TestTheEditLinksCanBeVerified(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := `version: "2.0"
repositories:
  - name: repo
    url: https://example.com/repo.git
daemon:
  link_verification:
    skip_edit_links: false
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write the config: %v", err)
	}

	_, cfg, err := LoadWithResult(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Daemon == nil || cfg.Daemon.LinkVerification == nil {
		t.Fatal("the link verification is missing")
	}
	lv := cfg.Daemon.LinkVerification
	if lv.SkipEditLinks == nil {
		t.Fatal("the field is missing")
	}
	if *lv.SkipEditLinks {
		t.Fatal("the edit links are still skipped although the operator asked for them")
	}
}

// Without the field the edit links are skipped: they need a login.
func TestTheEditLinksAreSkippedWithoutTheField(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := `version: "2.0"
repositories:
  - name: repo
    url: https://example.com/repo.git
daemon:
  link_verification:
    enabled: true
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write the config: %v", err)
	}

	_, cfg, err := LoadWithResult(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	lv := cfg.Daemon.LinkVerification
	if lv.SkipEditLinks == nil || !*lv.SkipEditLinks {
		t.Fatal("the edit links are not skipped by default")
	}
}
