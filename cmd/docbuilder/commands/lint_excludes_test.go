package commands

import (
	"os"
	"path/filepath"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// The documented lint.excludes was ignored by the command line: only
// the MCP server read it.
func TestTheLintExcludesOfTheConfigurationAreUsed(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	content := `version: "2.0"
repositories:
  - name: repo
    url: https://example.com/repo.git
lint:
  excludes:
    - "**/README.md"
`
	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write the config: %v", err)
	}

	excludes, pairs, err := lintSettingsFromConfig(&CLI{Config: configPath})
	if err != nil {
		t.Fatalf("the excludes: %v", err)
	}
	if pairs != nil {
		t.Fatalf("the pairs are %v, want none", pairs)
	}
	if len(excludes) != 1 || excludes[0] != "**/README.md" {
		t.Fatalf("the excludes are %v", excludes)
	}
}

// A configuration that is not there is no error: the linter works
// without one.
func TestTheLintWithoutAConfiguration(t *testing.T) {
	excludes, _, err := lintSettingsFromConfig(&CLI{Config: filepath.Join(t.TempDir(), "config.yaml")})
	if err != nil {
		t.Fatalf("the excludes: %v", err)
	}
	if excludes != nil {
		t.Fatalf("the excludes are %v, want none", excludes)
	}
}

// The configuration without a lint block leaves the defaults of the
// linter alone.
func TestTheLintWithoutALintBlock(t *testing.T) {
	cfg := &config.Config{}
	if got := cfg.LintExcludes(); got != nil {
		t.Fatalf("the excludes are %v, want none", got)
	}
}
