package commands

import (
	"path/filepath"
	"strings"
	"testing"
)

// The operator named a configuration file that is not there: that is
// an error, not a reason to build something else. The build silently
// built the local docs directory and exited 0.
func TestTheBuildFailsWhenTheNamedConfigurationFileIsMissing(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "definitely-missing.yaml")

	out := "./out"
	b := &BuildCmd{DocsDir: "./docs", Output: &out}
	root := &CLI{Config: missing}

	err := b.Run(&Global{}, root)
	if err == nil {
		t.Fatal("the build accepted a configuration file that is not there")
	}
	if !strings.Contains(err.Error(), missing) {
		t.Fatalf("the error does not name the configuration file: %v", err)
	}
}
