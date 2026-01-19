package pipeline

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFingerprintContent(t *testing.T) {
	t.Run("Generates fingerprint for markdown", func(t *testing.T) {
		doc := &Document{
			Path: "test.md",
			Raw:  []byte("---\ntitle: Test\n---\nContent"),
		}

		_, err := fingerprintContent(doc)
		require.NoError(t, err)

		raw := string(doc.Raw)
		assert.True(t, strings.HasPrefix(raw, "---\n"))
		assert.Contains(t, raw, "fingerprint:")
	})

	t.Run("Preserves UID across fingerprint rewrite", func(t *testing.T) {
		// mdfp might reorder or rewrite the frontmatter. We want to ensure UID stays.
		doc := &Document{
			Path: "test.md",
			Raw:  []byte("---\ntitle: Test\nuid: stable-123\n---\nContent"),
		}

		_, err := fingerprintContent(doc)
		require.NoError(t, err)

		raw := string(doc.Raw)
		assert.Contains(t, raw, "uid: stable-123")
		assert.Contains(t, raw, "fingerprint:")
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
