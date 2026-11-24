package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A self-hosted forge has no default address. Without an api_url the
// first request dies with "unsupported protocol scheme", long after the
// daemon started.
func TestASelfHostedForgeNeedsAnAPIUrl(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := `version: "2.0"
forges:
  - name: forgejo
    type: forgejo
    base_url: "https://git.example.com/"
    auth:
      type: token
      token: "t"
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write the config: %v", err)
	}

	_, _, err := LoadWithResult(path)
	if err == nil {
		t.Fatal("the configuration without an api_url was accepted")
	}
	if !strings.Contains(err.Error(), "api_url") {
		t.Fatalf("the error does not mention the api_url: %v", err)
	}
}

// GitHub has a default address: the configuration without one is fine.
func TestGitHubNeedsNoAPIUrl(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := `version: "2.0"
forges:
  - name: github
    type: github
    auto_discover: true
    auth:
      type: token
      token: "t"
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write the config: %v", err)
	}

	_, _, err := LoadWithResult(path)
	if err != nil {
		t.Fatalf("the GitHub configuration without an api_url was refused: %v", err)
	}
}
