package lint

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/inful/mdfp"
	"github.com/stretchr/testify/require"
)

func TestFrontmatterFingerprintRule_Check(t *testing.T) {
	rule := &FrontmatterFingerprintRule{}

	t.Run("reports missing fingerprint", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "doc.md")
		require.NoError(t, os.WriteFile(path, []byte("# Title\n\nHello\n"), 0o600))

		issues, err := rule.Check(path)
		require.NoError(t, err)
		require.Len(t, issues, 1)
		require.Equal(t, SeverityError, issues[0].Severity)
		require.Equal(t, "frontmatter-fingerprint", issues[0].Rule)
	})

	t.Run("passes when fingerprint is valid", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "doc.md")

		content, err := mdfp.ProcessContent("# Title\n\nHello\n")
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

		issues, err := rule.Check(path)
		require.NoError(t, err)
		require.Len(t, issues, 0)
	})
}
