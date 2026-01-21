package lint

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/frontmatter"
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

		body := "# Title\n\nHello\n"
		fields := map[string]any{
			"title": "My Title",
			"tags":  []string{"one", "two"},
		}

		hashStyle := frontmatter.Style{Newline: "\n"}
		frontmatterForHashBytes, err := frontmatter.SerializeYAML(fields, hashStyle)
		require.NoError(t, err)
		frontmatterForHash := strings.TrimSuffix(string(frontmatterForHashBytes), "\n")

		fields[mdfp.FingerprintField] = mdfp.CalculateFingerprintFromParts(frontmatterForHash, body)
		frontmatterBytes, err := frontmatter.SerializeYAML(fields, hashStyle)
		require.NoError(t, err)

		contentBytes := frontmatter.Join(frontmatterBytes, []byte(body), true, frontmatter.Style{Newline: "\n"})
		require.NoError(t, os.WriteFile(path, contentBytes, 0o600))

		issues, err := rule.Check(path)
		require.NoError(t, err)
		require.Len(t, issues, 0)
	})
}
