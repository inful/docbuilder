package lint

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDocmodelParity_BrokenAndLinkDetection_AgreeOnLineNumbers(t *testing.T) {
	tmpDir := t.TempDir()

	docsDir := filepath.Join(tmpDir, "docs")
	require.NoError(t, os.MkdirAll(docsDir, 0o750))

	sourceFile := filepath.Join(docsDir, "index.md")
	src := "---\n" +
		"title: x\n" +
		"---\n" +
		"Inline code `./missing.md` should be ignored.\n" +
		"```sh\n" +
		"echo ./missing.md\n" +
		"```\n" +
		"Real link: [Missing](./missing.md)\n"
	require.NoError(t, os.WriteFile(sourceFile, []byte(src), 0o600))

	// Broken-link detection should report file-coordinate line numbers.
	broken, err := detectBrokenLinksInFile(sourceFile)
	require.NoError(t, err)
	require.Len(t, broken, 1)
	require.Equal(t, 8, broken[0].LineNumber)

	// Link detection should attribute the same destination to the same file line.
	missingTarget := filepath.Join(docsDir, "missing.md")
	absTarget, err := filepath.Abs(missingTarget)
	require.NoError(t, err)

	linter := NewLinter(&Config{Format: "text"})
	fixer := NewFixer(linter, false, false)

	links, err := fixer.findLinksInFile(sourceFile, absTarget)
	require.NoError(t, err)
	require.Len(t, links, 1)
	require.Equal(t, broken[0].LineNumber, links[0].LineNumber)
}
