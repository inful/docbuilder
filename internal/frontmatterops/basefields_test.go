package frontmatterops

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestEnsureTypeDocs_SetsWhenMissing(t *testing.T) {
	fields := map[string]any{}

	changed := EnsureTypeDocs(fields)
	require.True(t, changed)
	require.Equal(t, "docs", fields["type"])
}

func TestEnsureTypeDocs_DoesNotChangeWhenPresent(t *testing.T) {
	fields := map[string]any{"type": "blog"}

	changed := EnsureTypeDocs(fields)
	require.False(t, changed)
	require.Equal(t, "blog", fields["type"])
}

func TestEnsureTitle_SetsFallbackWhenMissing(t *testing.T) {
	fields := map[string]any{}

	changed := EnsureTitle(fields, "Hello")
	require.True(t, changed)
	require.Equal(t, "Hello", fields["title"])
}

func TestEnsureTitle_SetsFallbackWhenEmptyString(t *testing.T) {
	fields := map[string]any{"title": "   "}

	changed := EnsureTitle(fields, "Hello")
	require.True(t, changed)
	require.Equal(t, "Hello", fields["title"])
}

func TestEnsureTitle_DoesNotChangeWhenNonEmpty(t *testing.T) {
	fields := map[string]any{"title": "Already"}

	changed := EnsureTitle(fields, "Hello")
	require.False(t, changed)
	require.Equal(t, "Already", fields["title"])
}

func TestEnsureDate_SetsCommitDateWhenMissing(t *testing.T) {
	fields := map[string]any{}
	commitDate := time.Date(2024, 2, 3, 4, 5, 6, 0, time.FixedZone("-0700", -7*60*60))
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	changed := EnsureDate(fields, commitDate, now)
	require.True(t, changed)
	require.Equal(t, commitDate.Format("2006-01-02T15:04:05-07:00"), fields["date"])
}

func TestEnsureDate_SetsNowWhenCommitDateZero(t *testing.T) {
	fields := map[string]any{}
	now := time.Date(2026, 1, 1, 2, 3, 4, 0, time.FixedZone("+0100", 1*60*60))

	changed := EnsureDate(fields, time.Time{}, now)
	require.True(t, changed)
	require.Equal(t, now.Format("2006-01-02T15:04:05-07:00"), fields["date"])
}

func TestEnsureDate_DoesNotChangeWhenPresent(t *testing.T) {
	fields := map[string]any{"date": "2020-01-01T00:00:00Z"}

	changed := EnsureDate(fields, time.Time{}, time.Now())
	require.False(t, changed)
	require.Equal(t, "2020-01-01T00:00:00Z", fields["date"])
}
