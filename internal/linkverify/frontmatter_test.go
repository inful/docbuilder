package linkverify

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseFrontMatter(t *testing.T) {
	t.Run("no front matter", func(t *testing.T) {
		_, err := ParseFrontMatter([]byte("# Title\n"))
		require.Error(t, err)
		require.True(t, errors.Is(err, ErrNoFrontMatter))
	})

	t.Run("valid YAML front matter", func(t *testing.T) {
		fm, err := ParseFrontMatter([]byte("---\ntitle: Test Page\n---\n# Body\n"))
		require.NoError(t, err)
		require.NotNil(t, fm)
		require.Equal(t, "Test Page", fm["title"])
	})

	t.Run("valid YAML front matter (CRLF)", func(t *testing.T) {
		fm, err := ParseFrontMatter([]byte("---\r\ntitle: Test Page\r\n---\r\n# Body\r\n"))
		require.NoError(t, err)
		require.NotNil(t, fm)
		require.Equal(t, "Test Page", fm["title"])
	})

	t.Run("empty front matter block", func(t *testing.T) {
		fm, err := ParseFrontMatter([]byte("---\n---\n# Body\n"))
		require.NoError(t, err)
		require.Empty(t, fm)
	})

	t.Run("whitespace-only front matter block", func(t *testing.T) {
		fm, err := ParseFrontMatter([]byte("---\n\n---\n# Body\n"))
		require.NoError(t, err)
		require.Empty(t, fm)
	})

	t.Run("malformed front matter (missing closing delimiter)", func(t *testing.T) {
		_, err := ParseFrontMatter([]byte("---\ntitle: Test Page\n# Body\n"))
		require.Error(t, err)
		require.True(t, errors.Is(err, ErrNoFrontMatter))
	})

	t.Run("invalid YAML front matter", func(t *testing.T) {
		_, err := ParseFrontMatter([]byte("---\ntitle: [\n---\n# Body\n"))
		require.Error(t, err)
		require.False(t, errors.Is(err, ErrNoFrontMatter))
	})
}
