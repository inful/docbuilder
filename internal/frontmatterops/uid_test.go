package frontmatterops

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEnsureUID_Missing_GeneratesUID(t *testing.T) {
	fields := map[string]any{}

	uid, changed, err := EnsureUID(fields)
	require.NoError(t, err)
	require.True(t, changed)
	require.NotEmpty(t, uid)
	require.Equal(t, uid, fields["uid"])
}

func TestEnsureUID_AlreadyPresent_DoesNotChange(t *testing.T) {
	fields := map[string]any{"uid": "abc"}

	uid, changed, err := EnsureUID(fields)
	require.NoError(t, err)
	require.False(t, changed)
	require.Equal(t, "abc", uid)
	require.Equal(t, "abc", fields["uid"])
}

func TestEnsureUIDAlias_Missing_AddsExpected(t *testing.T) {
	fields := map[string]any{}

	changed, err := EnsureUIDAlias(fields, "abc")
	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, []string{"/_uid/abc/"}, fields["aliases"])
}

func TestEnsureUIDAlias_AliasesSliceString_AppendsWhenMissing(t *testing.T) {
	fields := map[string]any{"aliases": []string{"/existing/"}}

	changed, err := EnsureUIDAlias(fields, "abc")
	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, []string{"/existing/", "/_uid/abc/"}, fields["aliases"])
}

func TestEnsureUIDAlias_AliasesSliceString_NoChangeWhenPresent(t *testing.T) {
	fields := map[string]any{"aliases": []string{"/_uid/abc/"}}

	changed, err := EnsureUIDAlias(fields, "abc")
	require.NoError(t, err)
	require.False(t, changed)
	require.Equal(t, []string{"/_uid/abc/"}, fields["aliases"])
}

func TestEnsureUIDAlias_AliasesSliceAny_AppendsWhenMissing(t *testing.T) {
	fields := map[string]any{"aliases": []any{"/existing/"}}

	changed, err := EnsureUIDAlias(fields, "abc")
	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, []string{"/existing/", "/_uid/abc/"}, fields["aliases"])
}

func TestEnsureUIDAlias_AliasesString_NoChangeWhenAlreadyExpected(t *testing.T) {
	fields := map[string]any{"aliases": "/_uid/abc/"}

	changed, err := EnsureUIDAlias(fields, "abc")
	require.NoError(t, err)
	require.False(t, changed)
	require.Equal(t, "/_uid/abc/", fields["aliases"])
}

func TestEnsureUIDAlias_AliasesString_AppendsWhenDifferent(t *testing.T) {
	fields := map[string]any{"aliases": "/existing/"}

	changed, err := EnsureUIDAlias(fields, "abc")
	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, []string{"/existing/", "/_uid/abc/"}, fields["aliases"])
}

func TestEnsureUIDAlias_InvalidUID_ReturnsError(t *testing.T) {
	fields := map[string]any{}

	_, err := EnsureUIDAlias(fields, "")
	require.Error(t, err)
}
