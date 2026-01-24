package config

import (
	"os"
	"testing"
)

func TestDaemonPublicOnly_DefaultFalseWhenMissing(t *testing.T) {
	configContent := `version: "2.0"

daemon:
  http:
    docs_port: 9000
    webhook_port: 9001
    admin_port: 9002
  sync:
    schedule: "0 */6 * * *"
    concurrent_builds: 5
    queue_size: 200
  storage:
    state_file: "./custom-state.json"
    repo_cache_dir: "./custom-repos"
    output_dir: "./custom-output"

forges:
  - name: minimal-github
    type: github
    organizations:
      - test-org
    auth:
      type: token
      token: test-token

hugo:
  title: Minimal Config
  base_url: https://example.invalid/

output:
  directory: ./custom-output
  clean: true
`

	tmpFile, err := os.CreateTemp(t.TempDir(), "test-v2-daemon-publiconly-missing-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	if _, writeErr := tmpFile.WriteString(configContent); writeErr != nil {
		t.Fatalf("Failed to write config: %v", writeErr)
	}
	_ = tmpFile.Close()

	cfg, err := Load(tmpFile.Name())
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Daemon == nil {
		t.Fatalf("expected daemon config to be present")
	}
	if cfg.Daemon.Content.PublicOnly {
		t.Fatalf("expected daemon.content.public_only default false")
	}
	if cfg.IsDaemonPublicOnlyEnabled() {
		t.Fatalf("expected IsDaemonPublicOnlyEnabled() to be false")
	}
}

func TestDaemonPublicOnly_ParsesTrue(t *testing.T) {
	configContent := `version: "2.0"

daemon:
  content:
    public_only: true
  http:
    docs_port: 9000
    webhook_port: 9001
    admin_port: 9002
  sync:
    schedule: "0 */6 * * *"
    concurrent_builds: 5
    queue_size: 200
  storage:
    state_file: "./custom-state.json"
    repo_cache_dir: "./custom-repos"
    output_dir: "./custom-output"

forges:
  - name: minimal-github
    type: github
    organizations:
      - test-org
    auth:
      type: token
      token: test-token

hugo:
  title: Minimal Config
  base_url: https://example.invalid/

output:
  directory: ./custom-output
  clean: true
`

	tmpFile, err := os.CreateTemp(t.TempDir(), "test-v2-daemon-publiconly-true-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	if _, writeErr := tmpFile.WriteString(configContent); writeErr != nil {
		t.Fatalf("Failed to write config: %v", writeErr)
	}
	_ = tmpFile.Close()

	cfg, err := Load(tmpFile.Name())
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Daemon == nil {
		t.Fatalf("expected daemon config to be present")
	}
	if !cfg.Daemon.Content.PublicOnly {
		t.Fatalf("expected daemon.content.public_only true")
	}
	if !cfg.IsDaemonPublicOnlyEnabled() {
		t.Fatalf("expected IsDaemonPublicOnlyEnabled() to be true")
	}
}
