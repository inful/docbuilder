package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUnmarshalStrict_RejectsUnknownKey(t *testing.T) {
	raw := "version: \"2.0\"\nhugo:\n  theme: relearn\n"
	_, err := unmarshalStrict([]byte(raw))
	if err == nil {
		t.Fatal("expected an error for the unknown hugo.theme key")
	}
	if !strings.Contains(err.Error(), "theme") {
		t.Errorf("error should name the offending key, got: %v", err)
	}
}

func TestUnmarshalStrict_RejectsUnknownNestedKey(t *testing.T) {
	raw := "version: \"2.0\"\ndaemon:\n  sync:\n    interval_minutes: 5\n"
	_, err := unmarshalStrict([]byte(raw))
	if err == nil {
		t.Fatal("expected an error for the unknown daemon.sync.interval_minutes key")
	}
	if !strings.Contains(err.Error(), "interval_minutes") {
		t.Errorf("error should name the offending key, got: %v", err)
	}
}

func TestUnmarshalStrict_AcceptsKnownKeys(t *testing.T) {
	raw := "version: \"2.0\"\nhugo:\n  title: T\n  base_url: https://example.com\n" +
		"repositories:\n  - url: https://example.com/a.git\n    name: a\n"
	cfg, err := unmarshalStrict([]byte(raw))
	if err != nil {
		t.Fatalf("known keys must load: %v", err)
	}
	if cfg.Hugo.Title != "T" {
		t.Errorf("title: got %q want %q", cfg.Hugo.Title, "T")
	}
}

func TestUnmarshalStrict_AllowsArbitraryKeysInsideMaps(t *testing.T) {
	raw := "version: \"2.0\"\nhugo:\n  params:\n    anything: 1\n    nested:\n      deep: 2\n" +
		"forges:\n  - name: g\n    type: github\n    options:\n      custom: 3\n"
	if _, err := unmarshalStrict([]byte(raw)); err != nil {
		t.Fatalf("map values accept any key: %v", err)
	}
}

func TestLoadWithResult_RejectsUnknownKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	raw := "version: \"2.0\"\nhugo:\n  theme: relearn\n" +
		"repositories:\n  - url: https://example.com/a.git\n    name: a\n"
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	_, _, err := LoadWithResult(path)
	if err == nil {
		t.Fatal("expected LoadWithResult to reject the unknown key")
	}
	if !strings.Contains(err.Error(), "theme") {
		t.Errorf("error should name the offending key, got: %v", err)
	}
}
