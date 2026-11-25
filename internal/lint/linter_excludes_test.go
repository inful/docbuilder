package lint

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// The excludes are paths of the repository: the readme of the
// repository was linted when the path was absolute and skipped when it
// was relative.
func TestTheExcludesHoldForAnAbsolutePathToo(t *testing.T) {
	repo := t.TempDir()
	if err := os.Mkdir(filepath.Join(repo, ".git"), 0o750); err != nil {
		t.Fatalf("create the repository: %v", err)
	}
	readme := filepath.Join(repo, "README.md")
	if err := os.WriteFile(readme, []byte("# The readme\n"), 0o600); err != nil {
		t.Fatalf("write the readme: %v", err)
	}
	page := filepath.Join(repo, "docs", "page.md")
	if err := os.MkdirAll(filepath.Dir(page), 0o750); err != nil {
		t.Fatalf("create the directory: %v", err)
	}
	if err := os.WriteFile(page, []byte("# Page\n"), 0o600); err != nil {
		t.Fatalf("write the page: %v", err)
	}

	linter := NewLinter(&Config{})

	result, err := linter.LintPaths([]string{readme, page})
	require.NoError(t, err)
	require.Equal(t, 1, result.FilesTotal, "the readme of the repository was counted")

	for _, issue := range result.Issues {
		require.NotContains(t, issue.FilePath, "README.md",
			"the readme of the repository was linted")
	}
}
