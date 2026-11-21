package pipeline

import (
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
