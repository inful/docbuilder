package frontmatterops

import (
	"testing"
	"time"

	"github.com/inful/mdfp"
	"github.com/stretchr/testify/require"
)

func TestUpsertFingerprintAndMaybeLastmod(t *testing.T) {
	now := time.Date(2026, 1, 22, 12, 0, 0, 0, time.FixedZone("X", 2*60*60))
	expectedLastmod := now.UTC().Format("2006-01-02")

	t.Run("sets fingerprint and lastmod when missing", func(t *testing.T) {
		fields := map[string]any{"title": "Test"}
		body := []byte("hello")

		fp, changed, err := UpsertFingerprintAndMaybeLastmod(fields, body, now)
		require.NoError(t, err)
		require.True(t, changed)
		require.Equal(t, fp, fields[mdfp.FingerprintField])
		require.Equal(t, expectedLastmod, fields["lastmod"])
	})

	t.Run("does not update lastmod when fingerprint unchanged", func(t *testing.T) {
		fields := map[string]any{"title": "Test"}
		body := []byte("hello")

		existing, err := ComputeFingerprint(fields, body)
		require.NoError(t, err)
		fields[mdfp.FingerprintField] = existing
		fields["lastmod"] = "1999-01-01"

		fp, changed, err := UpsertFingerprintAndMaybeLastmod(fields, body, now)
		require.NoError(t, err)
		require.False(t, changed)
		require.Equal(t, existing, fp)
		require.Equal(t, "1999-01-01", fields["lastmod"], "lastmod should not change")
	})

	t.Run("updates lastmod when fingerprint changes", func(t *testing.T) {
		fields := map[string]any{"title": "Test"}
		bodyA := []byte("hello")
		bodyB := []byte("hello! (changed)")

		existing, err := ComputeFingerprint(fields, bodyA)
		require.NoError(t, err)
		fields[mdfp.FingerprintField] = existing
		fields["lastmod"] = "1999-01-01"

		fp, changed, err := UpsertFingerprintAndMaybeLastmod(fields, bodyB, now)
		require.NoError(t, err)
		require.True(t, changed)
		require.NotEqual(t, existing, fp)
		require.Equal(t, fp, fields[mdfp.FingerprintField])
		require.Equal(t, expectedLastmod, fields["lastmod"])
	})
}
