package pipeline

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// A page that uses the children shortcode must build. The documentation
// of the repositories was written for a theme that had the shortcode,
// and a shortcode without a template fails the build of the whole site.
func TestTheChildrenShortcodeBuilds(t *testing.T) {
	if _, err := exec.LookPath("hugo"); err != nil {
		t.Skip("hugo is not installed")
	}

	dir := t.TempDir()
	hugoDir := filepath.Join(dir, "hugo")

	contentDir := filepath.Join(hugoDir, "content", "repo", "onboarding")
	if err := os.MkdirAll(contentDir, 0o750); err != nil {
		t.Fatalf("make the content dir: %v", err)
	}
	index := "---\ntitle: Onboarding\n---\n\nThe pages below:\n\n{{% children description=\"true\" %}}\n"
	if err := os.WriteFile(filepath.Join(contentDir, "_index.md"), []byte(index), 0o600); err != nil {
		t.Fatalf("write the index: %v", err)
	}
	if err := os.WriteFile(filepath.Join(contentDir, "first.md"), []byte("---\ntitle: First\n---\n\nThe first page.\n"), 0o600); err != nil {
		t.Fatalf("write the page: %v", err)
	}

	// The shortcode of the site.
	shortcodeDir := filepath.Join(hugoDir, "layouts", "_shortcodes")
	if err := os.MkdirAll(shortcodeDir, 0o750); err != nil {
		t.Fatalf("make the shortcode dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(shortcodeDir, "children.html"), []byte(docsyChildrenShortcode), 0o600); err != nil {
		t.Fatalf("write the shortcode: %v", err)
	}

	// Hugo has no templates of its own since 0.146, so the test brings
	// the two it needs.
	defaultDir := filepath.Join(hugoDir, "layouts", "_default")
	if err := os.MkdirAll(defaultDir, 0o750); err != nil {
		t.Fatalf("make the default layout dir: %v", err)
	}
	for _, name := range []string{"list.html", "single.html"} {
		if err := os.WriteFile(filepath.Join(defaultDir, name), []byte("{{ .Title }}{{ .Content }}"), 0o600); err != nil {
			t.Fatalf("write the layout: %v", err)
		}
	}

	hugoYAML := "baseURL: https://www.local\ntitle: Test\ndisableKinds: [\"RSS\", \"sitemap\", \"taxonomy\", \"term\"]\n"
	if err := os.WriteFile(filepath.Join(hugoDir, "hugo.yaml"), []byte(hugoYAML), 0o600); err != nil {
		t.Fatalf("write hugo.yaml: %v", err)
	}

	cmd := exec.CommandContext(t.Context(), "hugo", "--logLevel", "error")
	cmd.Dir = hugoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("hugo failed: %v\n%s", err, string(out))
	}

	// #nosec G304 -- the path is the test's own directory
	page, err := os.ReadFile(filepath.Join(hugoDir, "public", "repo", "onboarding", "index.html"))
	if err != nil {
		t.Fatalf("read the page: %v", err)
	}
	if !strings.Contains(string(page), "First") {
		t.Fatalf("the page does not list the pages below it:\n%s", page)
	}
}
