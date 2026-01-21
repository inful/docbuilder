package pipeline

import (
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/frontmatter"
	"github.com/inful/mdfp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFingerprintContent(t *testing.T) {
	t.Run("adds deterministic fingerprint without changing body", func(t *testing.T) {
		originalBody := "# H1\n\nBody"
		doc := &Document{
			Path: "test.md",
			Raw:  []byte("---\ntitle: Test\n---\n" + originalBody),
		}

		_, err := fingerprintContent(doc)
		require.NoError(t, err)

		fmRaw, body, had, _, err := frontmatter.Split(doc.Raw)
		require.NoError(t, err)
		require.True(t, had)
		require.Equal(t, []byte(originalBody), body)

		fm, err := frontmatter.ParseYAML(fmRaw)
		require.NoError(t, err)
		require.Equal(t, "Test", fm["title"])

		fp, ok := fm["fingerprint"].(string)
		require.True(t, ok)
		require.NotEmpty(t, fp)

		expectedFP := mdfp.CalculateFingerprintFromParts("title: Test", originalBody)
		require.Equal(t, expectedFP, fp)
	})

	t.Run("Preserves UID across fingerprint rewrite", func(t *testing.T) {
		// UID must be preserved even when fingerprint is added/updated.
		doc := &Document{
			Path: "test.md",
			Raw:  []byte("---\ntitle: Test\nuid: stable-123\n---\nContent"),
		}

		_, err := fingerprintContent(doc)
		require.NoError(t, err)
		fmRaw, _, had, _, err := frontmatter.Split(doc.Raw)
		require.NoError(t, err)
		require.True(t, had)
		fm, err := frontmatter.ParseYAML(fmRaw)
		require.NoError(t, err)
		assert.Equal(t, "stable-123", fm["uid"])
		_, ok := fm["fingerprint"].(string)
		assert.True(t, ok)
	})

	t.Run("updates fingerprint when body changes", func(t *testing.T) {
		docA := &Document{Path: "a.md", Raw: []byte("---\ntitle: Test\n---\nBody A")}
		docB := &Document{Path: "b.md", Raw: []byte("---\ntitle: Test\n---\nBody B")}

		_, err := fingerprintContent(docA)
		require.NoError(t, err)
		_, err = fingerprintContent(docB)
		require.NoError(t, err)

		fmRawA, bodyA, _, _, err := frontmatter.Split(docA.Raw)
		require.NoError(t, err)
		fmA, err := frontmatter.ParseYAML(fmRawA)
		require.NoError(t, err)
		fpA := fmA["fingerprint"].(string)

		fmRawB, bodyB, _, _, err := frontmatter.Split(docB.Raw)
		require.NoError(t, err)
		fmB, err := frontmatter.ParseYAML(fmRawB)
		require.NoError(t, err)
		fpB := fmB["fingerprint"].(string)

		require.Equal(t, []byte("Body A"), bodyA)
		require.Equal(t, []byte("Body B"), bodyB)
		require.NotEqual(t, fpA, fpB)
	})

	t.Run("preserves non-fingerprint YAML fields", func(t *testing.T) {
		originalBody := "Body"
		original := "---\n" +
			"title: Test\n" +
			"tags:\n" +
			"  - a\n" +
			"  - b\n" +
			"---\n" + originalBody

		doc := &Document{Path: "test.md", Raw: []byte(original)}
		_, err := fingerprintContent(doc)
		require.NoError(t, err)

		fmRaw, body, _, _, err := frontmatter.Split(doc.Raw)
		require.NoError(t, err)
		require.Equal(t, []byte(originalBody), body)

		fm, err := frontmatter.ParseYAML(fmRaw)
		require.NoError(t, err)
		require.Equal(t, "Test", fm["title"])
		require.Equal(t, []any{"a", "b"}, fm["tags"])
		_, ok := fm["fingerprint"].(string)
		require.True(t, ok)
	})

	t.Run("adds frontmatter when missing", func(t *testing.T) {
		originalBody := "# Title\n\nHello"
		doc := &Document{Path: "test.md", Raw: []byte(originalBody)}

		_, err := fingerprintContent(doc)
		require.NoError(t, err)

		fmRaw, body, had, _, err := frontmatter.Split(doc.Raw)
		require.NoError(t, err)
		require.True(t, had)
		require.Equal(t, []byte(originalBody), body)

		fm, err := frontmatter.ParseYAML(fmRaw)
		require.NoError(t, err)
		fp := fm["fingerprint"].(string)
		expectedFP := mdfp.CalculateFingerprintFromParts("", originalBody)
		require.Equal(t, expectedFP, fp)
	})

	t.Run("Skips non-markdown files", func(t *testing.T) {
		content := []byte("Binary data")
		doc := &Document{
			Path: "image.png",
			Raw:  content,
		}

		_, err := fingerprintContent(doc)
		require.NoError(t, err)

		assert.Equal(t, content, doc.Raw)
	})
}
