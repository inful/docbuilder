package hugo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// TestSharedNavFiles pins the set of files the shared sidebar mode writes
// into the Hugo project root. The theme's own sidebar partial must be
// shadowed (that is the whole point), and the vendored copy must be written
// under a different name so the fragment can still render the navigation.
func TestSharedNavFiles(t *testing.T) {
	files := SharedNavFiles()

	want := []string{
		filepath.Join("layouts", "partials", "menu.html"),
		filepath.Join("layouts", "partials", "nav-full.html"),
		filepath.Join("layouts", "_default", "home.sidebar.html"),
		filepath.Join("assets", "js", "nav-shared.js"),
	}
	for _, rel := range want {
		if _, ok := files[rel]; !ok {
			t.Errorf("missing shared nav file %q", rel)
		}
	}
	if len(files) != len(want) {
		t.Errorf("unexpected number of shared nav files: got %d, want %d", len(files), len(want))
	}

	// The placeholder must point at the fragment and load the script.
	placeholder := files[filepath.Join("layouts", "partials", "menu.html")]
	for _, needle := range []string{`id="R-sidebar-menu"`, `data-nav-src=`, "js/nav-shared.js", "fingerprint"} {
		if !strings.Contains(placeholder, needle) {
			t.Errorf("placeholder is missing %q:\n%s", needle, placeholder)
		}
	}
	if strings.Contains(placeholder, "inline/menu") {
		t.Error("the placeholder must not render the navigation itself")
	}
	// The build id lets the script keep the fragment in the browser and
	// invalidate it when the site is rebuilt.
	if !strings.Contains(placeholder, `data-build="{{ site.Params.build_date }}"`) {
		t.Errorf("placeholder must carry the build id:\n%s", placeholder)
	}

	// The fragment layout renders the vendored navigation.
	fragment := files[filepath.Join("layouts", "_default", "home.sidebar.html")]
	if !strings.Contains(fragment, `partial "nav-full.html"`) {
		t.Errorf("fragment layout must render nav-full.html:\n%s", fragment)
	}
}

// TestVendoredRelearnNavMatchesPinnedVersion guards the vendored Relearn
// navigation against the pinned theme version: bumping the version in
// config_writer.go without re-vendoring the template would render the
// fragment with an outdated navigation.
func TestVendoredRelearnNavMatchesPinnedVersion(t *testing.T) {
	nav := string(relearnNavTemplate)
	if !strings.Contains(nav, "release "+relearnVersion) {
		t.Errorf("vendored Relearn navigation does not record release %s; regenerate it with tools/vendor-relearn-nav.sh", relearnVersion)
	}
	if !strings.Contains(nav, "hugo-theme-relearn") {
		t.Error("vendored Relearn navigation does not name its upstream source")
	}
}

// TestWriteSharedNavFiles checks that the files land in the build root.
func TestWriteSharedNavFiles(t *testing.T) {
	out := t.TempDir()
	gen := NewGenerator(&config.Config{Hugo: config.HugoConfig{
		Sidebar: &config.SidebarConfig{Mode: config.SidebarModeShared},
	}}, out)

	if err := gen.WriteSharedNavFiles(); err != nil {
		t.Fatalf("write shared nav files: %v", err)
	}

	for rel := range SharedNavFiles() {
		path := filepath.Join(out, rel)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected %s to exist: %v", rel, err)
		}
	}
}

// TestSharedNavScriptReinitialisesTheme guards the shared navigation script
// against losing the theme's sidebar behavior. The theme binds to the
// sidebar's contents on DOMContentLoaded, which happens before the fragment
// arrives, so the script has to re-run the parts that need the real
// navigation - above all the menu scrollbar, without which the sidebar does
// not scroll at all.
func TestSharedNavScriptReinitialisesTheme(t *testing.T) {
	script := string(navSharedScript)
	for _, needle := range []string{"initMenuScrollbar", "initSwipeHandler", "scrollToPositions"} {
		if !strings.Contains(script, needle) {
			t.Errorf("shared navigation script does not re-run %s; the sidebar would not scroll", needle)
		}
	}
}

// TestSharedNavScriptCachesFragment guards the browser-side cache: the
// fragment is the same on every page of a build, so the script keeps it in
// local storage and restores it instead of downloading it again on every
// navigation. The cache is keyed by the build id and the fragment URL, so a
// rebuild invalidates it.
func TestSharedNavScriptCachesFragment(t *testing.T) {
	script := string(navSharedScript)
	for _, needle := range []string{
		"localStorage.getItem",
		"localStorage.setItem",
		"data-build",
		"entry.build !== build",
	} {
		if !strings.Contains(script, needle) {
			t.Errorf("shared navigation script does not use the browser cache (%q); the navigation is downloaded on every page", needle)
		}
	}
}
