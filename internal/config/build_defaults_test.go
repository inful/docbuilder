package config

import (
	"gopkg.in/yaml.v3"
	"testing"
)

func TestDetectDeletionsDefaultEnabled(t *testing.T) {
	// Field omitted -> default should set true
	raw := `version: 2.0
forges:
  - name: f
    type: github
    api_url: https://api.github.com
    base_url: https://github.com
    organizations: [x]
output:
  directory: ./site
hugo:
  theme: hextra
`
	var cfg Config
	if err := yaml.Unmarshal([]byte(raw), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if err := applyDefaults(&cfg); err != nil {
		t.Fatalf("defaults: %v", err)
	}
	if !cfg.Build.DetectDeletions {
		t.Fatalf("expected DetectDeletions default true when omitted")
	}
}

func TestDetectDeletionsExplicitFalsePreserved(t *testing.T) {
	raw := `version: 2.0
build:
  detect_deletions: false
forges:
  - name: f
    type: github
    api_url: https://api.github.com
    base_url: https://github.com
    organizations: [x]
output:
  directory: ./site
hugo:
  theme: hextra
`
	var cfg Config
	if err := yaml.Unmarshal([]byte(raw), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if err := applyDefaults(&cfg); err != nil {
		t.Fatalf("defaults: %v", err)
	}
	if cfg.Build.DetectDeletions {
		t.Fatalf("expected DetectDeletions remain false when explicitly set")
	}
}
