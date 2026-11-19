package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The name of a repository is its identity in a build: the clone
// folder, the state map and the content path all use it. Two
// repositories with one name eat each other and one of them vanishes
// from the site without a word. A name with a slash or a dot points
// outside the workspace.
func TestRepositoryNameValidation(t *testing.T) {
	tests := []struct {
		name          string
		configContent string
		expectedError string
	}{
		{
			name: "Duplicate repository names",
			configContent: `version: "2.0"
repositories:
  - name: docs
    url: https://example.com/a.git
    branch: main
  - name: docs
    url: https://example.com/b.git
    branch: main`,
			expectedError: "duplicate repository name",
		},
		{
			name: "Duplicate repository names in different case",
			configContent: `version: "2.0"
repositories:
  - name: docs
    url: https://example.com/a.git
    branch: main
  - name: Docs
    url: https://example.com/b.git
    branch: main`,
			expectedError: "duplicate repository name",
		},
		{
			name: "Empty repository name",
			configContent: `version: "2.0"
repositories:
  - name: ""
    url: https://example.com/a.git
    branch: main`,
			expectedError: "repository name cannot be empty",
		},
		{
			name: "Repository name with a slash",
			configContent: `version: "2.0"
repositories:
  - name: ../escape
    url: https://example.com/a.git
    branch: main`,
			expectedError: "repository name must be a single path segment",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(path, []byte(tt.configContent), 0o600); err != nil {
				t.Fatalf("write config: %v", err)
			}

			_, err := Load(path)
			if err == nil {
				t.Fatalf("expected an error, got nil")
			}
			if !strings.Contains(err.Error(), tt.expectedError) {
				t.Fatalf("error = %q, want it to contain %q", err.Error(), tt.expectedError)
			}
		})
	}
}

// A repository with a normal name still loads.
func TestRepositoryNameValidation_AcceptsNormalNames(t *testing.T) {
	content := `version: "2.0"
repositories:
  - name: platform-services
    url: https://example.com/a.git
    branch: main`
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("expected the config to load, got %v", err)
	}
	if len(cfg.Repositories) != 1 || cfg.Repositories[0].Name != "platform-services" {
		t.Fatalf("repositories = %+v, want one named platform-services", cfg.Repositories)
	}
}
