package lint

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLinter_LintPath_DetectsBrokenLinks(t *testing.T) {
	tmpDir := t.TempDir()

	docsDir := filepath.Join(tmpDir, "docs")
	require.NoError(t, os.MkdirAll(docsDir, 0755))

	// Note: README.md is ignored by the linter/broken-link scanner.
	require.NoError(t, os.WriteFile(filepath.Join(docsDir, "README.md"), []byte(`# Readme

[Broken](./missing.md)
`), 0644))

	indexPath := filepath.Join(docsDir, "index.md")
	require.NoError(t, os.WriteFile(indexPath, []byte(`# Index

[OK](./guide.md)
[Broken](./missing.md)
![Broken Image](./images/missing.png)
`), 0644))

	require.NoError(t, os.WriteFile(filepath.Join(docsDir, "guide.md"), []byte("# Guide\n"), 0644))

	l := NewLinter(&Config{Format: "text"})
	res, err := l.LintPath(docsDir)
	require.NoError(t, err)

	// Two broken links in index.md (missing.md + missing.png). The README.md broken
	// link should not be included because README.md is ignored.
	require.True(t, res.HasErrors())
	require.Equal(t, 2, res.ErrorCount())

	for _, issue := range res.Issues {
		require.Equal(t, "broken-links", issue.Rule)
		require.Equal(t, SeverityError, issue.Severity)
		require.Equal(t, indexPath, issue.FilePath)
		require.NotZero(t, issue.Line)
	}
}
