package config

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// linkVerificationConfigTemplate is a minimal valid v2 config whose
// daemon section is filled in by the test.
const linkVerificationConfigTemplate = `version: "2.0"
forges:
  - name: minimal-github
    type: github
    organizations:
      - test-org
    auth:
      type: token
      token: test-token
hugo:
  title: Link Verification Defaults
daemon:
  sync:
    schedule: "0 0 1 1 *"
%s`

// loadLinkVerificationConfig loads a config with the given
// daemon.link_verification YAML block and returns the resulting flag.
func loadLinkVerificationConfig(t *testing.T, block string) bool {
	t.Helper()

	content := fmt.Sprintf(linkVerificationConfigTemplate, block)
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Daemon == nil || cfg.Daemon.LinkVerification == nil {
		t.Fatalf("expected daemon.link_verification to be defaulted")
	}
	return cfg.Daemon.LinkVerification.Enabled
}

// TestLinkVerificationDisabledByDefault pins that link verification is
// opt-in: a daemon without link_verification settings must not check links.
func TestLinkVerificationDisabledByDefault(t *testing.T) {
	if enabled := loadLinkVerificationConfig(t, ""); enabled {
		t.Fatalf("expected link verification to be disabled by default")
	}
}

// TestLinkVerificationExplicitEnable pins that an explicit opt-in is kept.
func TestLinkVerificationExplicitEnable(t *testing.T) {
	if enabled := loadLinkVerificationConfig(t, "  link_verification:\n    enabled: true\n"); !enabled {
		t.Fatalf("expected explicit link verification opt-in to be preserved")
	}
}

// TestLinkVerificationExplicitDisable pins that an explicit "enabled: false"
// is kept instead of being forced back on by the defaults pass.
func TestLinkVerificationExplicitDisable(t *testing.T) {
	if enabled := loadLinkVerificationConfig(t, "  link_verification:\n    enabled: false\n"); enabled {
		t.Fatalf("expected explicit link verification opt-out to be preserved")
	}
}
