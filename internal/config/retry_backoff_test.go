package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The retry backoff of a configuration that leaves the field out is the
// documented one: linear. The normalizer rewrote it to "fixed" and
// printed a warning about it.
func TestTheRetryBackoffDefaultsToLinear(t *testing.T) {
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

	result, cfg, err := LoadWithResult(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Build.RetryBackoff != RetryBackoffLinear {
		t.Fatalf("the retry backoff is %q, want %q", cfg.Build.RetryBackoff, RetryBackoffLinear)
	}
	for _, warning := range result.Warnings {
		if strings.Contains(warning, "retry_backoff") {
			t.Errorf("the configuration warned about the retry backoff: %s", warning)
		}
	}
}

// A retry backoff that nobody knows is still refused, with a warning.
func TestAnUnknownRetryBackoffIsRefused(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := `version: "2.0"
repositories:
  - name: repo
    url: https://example.com/repo.git
build:
  retry_backoff: bogus
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write the config: %v", err)
	}

	result, cfg, err := LoadWithResult(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Build.RetryBackoff != RetryBackoffFixed {
		t.Fatalf("the retry backoff is %q, want %q", cfg.Build.RetryBackoff, RetryBackoffFixed)
	}
	found := false
	for _, warning := range result.Warnings {
		if strings.Contains(warning, "retry_backoff") {
			found = true
		}
	}
	if !found {
		t.Error("the configuration did not warn about the unknown retry backoff")
	}
}
