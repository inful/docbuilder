package integration

import (
	"testing"
)

// TestGolden_DaemonPublicFilter tests the daemon-only public frontmatter filter.
// This test verifies:
// - Only pages with `public: true` are included in the generated site.
// - Pages without frontmatter or with `public: false` are excluded.
// - Static assets are still copied even if adjacent pages are excluded.
// - Generated index pages only appear for public scopes.
// - Generated indexes include `public: true`.
func TestGolden_DaemonPublicFilter(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping golden test in short mode")
	}

	runGoldenTest(t,
		"../../test/testdata/repos/public-filter",
		"../../test/testdata/configs/daemon-public-filter.yaml",
		"../../test/testdata/golden/daemon-public-filter",
		*updateGolden,
	)
}
