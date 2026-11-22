package pipeline

import (
	"crypto/sha512"
	"encoding/base64"
	"strings"
	"testing"
)

// TestVendoredDocsyAssetsMatchPinnedVersion guards the vendored
// Docsy assets against the pinned theme version: bumping the version
// without re-vendoring would ship the stylesheet of the old theme.
func TestVendoredDocsyAssetsMatchPinnedVersion(t *testing.T) {
	if !strings.Contains(docsyVendoredVersion, "docsy "+DocsyVersion) {
		t.Errorf("vendored Docsy assets do not record version %s; regenerate them with scripts/vendor-docsy-assets.sh", DocsyVersion)
	}
}

// TestVendoredDocsyStylesheetUsesVersionedFonts checks that the
// stylesheet points at the versioned font directory. The docs server
// gives fonts a one-year immutable cache header, so a stable font URL
// would keep serving the old font after a theme bump.
func TestVendoredDocsyStylesheetUsesVersionedFonts(t *testing.T) {
	want := "../webfonts/" + DocsyVersion + "/"
	if !strings.Contains(string(docsyStylesheet), want) {
		t.Errorf("vendored Docsy stylesheet does not reference %s", want)
	}
	if strings.Contains(string(docsyStylesheet), "url(../webfonts/fa-") {
		t.Error("vendored Docsy stylesheet references the fonts without the version directory")
	}
}

func TestIndexContent(t *testing.T) {
	body := indexContent("Title", "Description")
	if strings.Contains(body, "children") {
		t.Errorf("body carries a children shortcode: %q", body)
	}
	if !strings.Contains(body, "# Title") || !strings.Contains(body, "Description") {
		t.Errorf("body lost its title or description: %q", body)
	}
}

// TestVendoredHeadLoadsLunrLocally checks that the vendored head
// partial does not pull the offline-search library from a CDN. The
// theme hardcodes the unpkg URL; the site serves its own copy.
func TestVendoredHeadLoadsLunrLocally(t *testing.T) {
	head := string(docsyHeadHTML)
	if strings.Contains(head, "unpkg.com") || strings.Contains(head, "cdn.") {
		t.Errorf("vendored head partial still loads from a CDN:\n%s", head)
	}
	if !strings.Contains(head, `resources.Get "vendor/lunr/lunr.min.js"`) {
		t.Error("vendored head partial does not load the vendored lunr")
	}
}

// TestVendoredLunrMatchesThemeIntegrity checks that the vendored lunr
// is the exact file the theme pins: the theme's own head partial
// carried a subresource-integrity hash for it.
func TestVendoredLunrMatchesThemeIntegrity(t *testing.T) {
	const want = "203J0SNzyqHby3iU6hzvzltrWi/M41wOP5Gu+BiJMz5nwKykbkUx8Kp7iti0Lpli"
	got := sha384Base64(docsyLunrJS)
	if got != want {
		t.Errorf("vendored lunr hash is %s, want %s", got, want)
	}
}

// sha384Base64 returns the base64 SHA-384 digest of b, the form the
// subresource-integrity attribute uses.
func sha384Base64(b []byte) string {
	sum := sha512.Sum384(b)
	return base64.StdEncoding.EncodeToString(sum[:])
}
