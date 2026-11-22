package frontmatterops

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// The content-level helpers are the ones the lint fixer uses: they
// read a document, change its frontmatter and write it back.

func TestExtractUID(t *testing.T) {
	uid, ok := ExtractUID([]byte("---\nuid: abc\n---\n\nbody\n"))
	require.True(t, ok)
	require.Equal(t, testUID, uid)

	_, ok = ExtractUID([]byte("---\ntitle: T\n---\n\nbody\n"))
	require.False(t, ok, "a document without a uid has none")

	_, ok = ExtractUID([]byte("no frontmatter here\n"))
	require.False(t, ok, "a document without frontmatter has no uid")
}

func TestEnsureUIDInContent(t *testing.T) {
	out, changed := EnsureUIDInContent([]byte("---\ntitle: T\n---\n\nbody\n"))
	require.True(t, changed)

	uid, ok := ExtractUID(out)
	require.True(t, ok)
	require.NotEmpty(t, uid)
	require.Contains(t, string(out), UIDAliasPath(uid))
}

func TestEnsureUIDInContent_AlreadyHasUID_DoesNotChange(t *testing.T) {
	content := []byte("---\nuid: abc\n---\n\nbody\n")
	out, changed := EnsureUIDInContent(content)
	require.False(t, changed)
	require.Equal(t, string(content), string(out))
}

func TestEnsureUIDValueInContent(t *testing.T) {
	out, changed := EnsureUIDValueInContent([]byte("---\ntitle: T\n---\n\nbody\n"), testUID)
	require.True(t, changed)

	uid, ok := ExtractUID(out)
	require.True(t, ok)
	require.Equal(t, testUID, uid)
}

func TestEnsureUIDValueInContent_AlreadyHasUID_DoesNotChange(t *testing.T) {
	content := []byte("---\nuid: other\n---\n\nbody\n")
	out, changed := EnsureUIDValueInContent(content, testUID)
	require.False(t, changed)
	require.Equal(t, string(content), string(out))
}

func TestEnsureUIDAliasInContent(t *testing.T) {
	out, changed := EnsureUIDAliasInContent([]byte("---\nuid: abc\n---\n\nbody\n"), testUID)
	require.True(t, changed)
	require.Contains(t, string(out), testUIDAliasPath)
}

func TestEnsureUIDAliasInContent_WithoutFrontmatter_DoesNotChange(t *testing.T) {
	content := []byte("body only\n")
	out, changed := EnsureUIDAliasInContent(content, testUID)
	require.False(t, changed)
	require.Equal(t, string(content), string(out))
}

func TestUIDAliasPath(t *testing.T) {
	require.Equal(t, testUIDAliasPath, UIDAliasPath(testUID))
}
