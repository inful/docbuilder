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
	fields := map[string]any{"uid": testUID}

	uid, changed, err := EnsureUID(fields)
	require.NoError(t, err)
	require.False(t, changed)
	require.Equal(t, testUID, uid)
	require.Equal(t, testUID, fields["uid"])
}

func TestEnsureUIDAlias_Missing_AddsExpected(t *testing.T) {
	fields := map[string]any{}

	changed, err := EnsureUIDAlias(fields, testUID)
	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, []string{testUIDAliasPath}, fields["aliases"])
}

func TestEnsureUIDAlias_AliasesSliceString_AppendsWhenMissing(t *testing.T) {
	fields := map[string]any{"aliases": []string{testExistingAlias}}

	changed, err := EnsureUIDAlias(fields, testUID)
	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, []string{testExistingAlias, testUIDAliasPath}, fields["aliases"])
}

func TestEnsureUIDAlias_AliasesSliceString_NoChangeWhenPresent(t *testing.T) {
	fields := map[string]any{"aliases": []string{testUIDAliasPath}}

	changed, err := EnsureUIDAlias(fields, testUID)
	require.NoError(t, err)
	require.False(t, changed)
	require.Equal(t, []string{testUIDAliasPath}, fields["aliases"])
}

func TestEnsureUIDAlias_AliasesSliceAny_AppendsWhenMissing(t *testing.T) {
	fields := map[string]any{"aliases": []any{testExistingAlias}}

	changed, err := EnsureUIDAlias(fields, testUID)
	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, []string{testExistingAlias, testUIDAliasPath}, fields["aliases"])
}

func TestEnsureUIDAlias_AliasesString_NoChangeWhenAlreadyExpected(t *testing.T) {
	fields := map[string]any{"aliases": testUIDAliasPath}

	changed, err := EnsureUIDAlias(fields, testUID)
	require.NoError(t, err)
	require.False(t, changed)
	require.Equal(t, []string{testUIDAliasPath}, fields["aliases"])
}

func TestEnsureUIDAlias_AliasesString_AppendsWhenDifferent(t *testing.T) {
	fields := map[string]any{"aliases": testExistingAlias}

	changed, err := EnsureUIDAlias(fields, testUID)
	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, []string{testExistingAlias, testUIDAliasPath}, fields["aliases"])
}

func TestEnsureUIDValue_Missing_SetsValue(t *testing.T) {
	fields := map[string]any{}

	changed, err := EnsureUIDValue(fields, testUID)
	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, testUID, fields["uid"])
}

func TestEnsureUIDValue_AlreadyPresent_DoesNotChange(t *testing.T) {
	fields := map[string]any{"uid": "existing"}

	changed, err := EnsureUIDValue(fields, testUID)
	require.NoError(t, err)
	require.False(t, changed)
	require.Equal(t, "existing", fields["uid"])
}

func TestEnsureUIDValue_Empty_ReturnsError(t *testing.T) {
	fields := map[string]any{}

	_, err := EnsureUIDValue(fields, "")
	require.Error(t, err)
}

func TestEnsureUIDAlias_InvalidUID_ReturnsError(t *testing.T) {
	fields := map[string]any{}

	_, err := EnsureUIDAlias(fields, "")
	require.Error(t, err)
}
