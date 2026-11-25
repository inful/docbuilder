package git

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// A damaged cache file must not take the daemon down: the content
// "null" is a valid JSON, and it leaves the map of the cache empty and
// nil. The next write to the cache panicked.
func TestTheCacheOfADamagedFileIsWrittenAgain(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "remote-heads.json"), []byte("null"), 0o600))

	cache, err := NewRemoteHeadCache(dir)
	require.NoError(t, err)

	require.NotPanics(t, func() {
		cache.Set("https://example.invalid/repo.git", "main", "abc123")
	})
}
