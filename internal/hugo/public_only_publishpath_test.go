package hugo

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/docs"
)

// TestPublicOnly_PageBundleImage_PublishPath exercises Hugo's page-bundle
// publishing behavior when daemon.content.public_only is enabled, with a
// minimal real Hugo binary invocation against the docbuilder output tree.
//
// This is the integration test for the production symptom where the
// public_only instance serves images at public/drift/drift/img/... (doubled
// path) instead of public/drift/img/... The test sets up the smallest
// layout that mirrors the production case: two repos (so isSingleRepo is
// false and the docbuilder pipeline inserts the "drift/" namespace), a
// page bundle in drift/img/nett/ with a public doc, a private doc, and a
// sibling image asset, then runs Hugo and inspects the public/ tree.
//
// Result: both public_only on and public_only off produce
// public/drift/img/nett/pic.png - no doubling. The minimal case does not
// reproduce the bug, which suggests the production trigger is something
// more specific (Relearn theme permalinks, multiple page bundles, a
// static-asset conflict, or a stale public/ from a prior build).
//
// If you can share the actual content/ tree from the broken instance,
// this test is the place to grow the fixture until it reproduces.
// TestPublicOnly_CaseDifferentRepoDirs_DriftVsDrift reproduces the
// production symptom where site/content contains both "Drift" and "drift"
// directories on the public_only instance, while the non-public_only
// instance has only one of them.
//
// Root cause hypothesis: the index generator uses the raw Repository name
// from DocFile (e.g. "Drift") when computing the _index.md path, while
// the rest of the pipeline lowercases via GetHugoPath (which uses
// strings.ToLower(Repository)). On case-insensitive filesystems the two
// collide; on case-sensitive ones they are siblings.
//
// This test asserts that index paths in the generated content/ tree are
// always lowercased, matching GetHugoPath's behavior, so a case-different
// repo name cannot produce two different directories.
func TestPublicOnly_CaseDifferentRepoDirs_DriftVsDrift(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{
		Build:  config.BuildConfig{RenderMode: config.RenderModeNever},
		Hugo:   config.HugoConfig{Title: "Test", BaseURL: "https://www.local"},
		Daemon: &config.DaemonConfig{Content: config.DaemonContentConfig{PublicOnly: true}},
	}
	gen := NewGenerator(cfg, dir)

	// Note: Repository="Drift" with capital D, Section lowercased.
	pageDoc := docs.DocFile{
		Repository:   "Drift",
		Section:      "img/nett",
		Name:         "page",
		Extension:    ".md",
		RelativePath: "img/nett/page.md",
		DocsBase:     "docs",
		Content:      []byte("---\npublic: true\n---\n# Page\n\n![Pic](pic.png)\n"),
	}
	secretDoc := docs.DocFile{
		Repository:   "Drift",
		Section:      "img/nett",
		Name:         "secret",
		Extension:    ".md",
		RelativePath: "img/nett/secret.md",
		DocsBase:     "docs",
		Content:      []byte("# Secret\n"),
	}
	assetPath := filepath.Join(dir, "pic.png")
	if err := os.WriteFile(assetPath, []byte{0x89, 0x50, 0x4e, 0x47}, 0o600); err != nil {
		t.Fatalf("write asset: %v", err)
	}
	asset := docs.DocFile{
		Repository:   "Drift",
		Section:      "img/nett",
		Name:         "pic",
		Extension:    ".png",
		RelativePath: "img/nett/pic.png",
		DocsBase:     "docs",
		Path:         assetPath,
		IsAsset:      true,
	}
	// Second repo with different name so isSingleRepo=false.
	otherDoc := docs.DocFile{
		Repository:   "other",
		Section:      "notes",
		Name:         "readme",
		Extension:    ".md",
		RelativePath: "notes/readme.md",
		DocsBase:     "docs",
		Content:      []byte("---\npublic: true\n---\n# Other\n"),
	}
	files := []docs.DocFile{pageDoc, secretDoc, asset, otherDoc}
	if err := gen.copyContentFiles(t.Context(), files); err != nil {
		t.Fatalf("copy: %v", err)
	}

	// Walk the generated content/ tree and dump every directory name.
	var dirs []string
	if err := filepath.Walk(filepath.Join(dir, "content"), func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			rel, _ := filepath.Rel(filepath.Join(dir, "content"), path)
			dirs = append(dirs, rel)
		}
		return nil
	}); err != nil {
		t.Fatalf("walk: %v", err)
	}
	t.Logf("content/ dirs: %v", dirs)

	// The bug: both "Drift" and "drift" exist as siblings (or as aliased
	// dirs on a case-insensitive FS). We assert they don't both appear
	// under the same parent.
	hasUpper, hasLower := false, false
	for _, d := range dirs {
		for p := range strings.SplitSeq(d, string(filepath.Separator)) {
			if p == "Drift" {
				hasUpper = true
			}
			if p == "drift" {
				hasLower = true
			}
		}
	}
	if hasUpper && hasLower {
		t.Errorf("content/ has BOTH 'Drift' and 'drift' directories - case collision that doubles Hugo's publish paths")
	}
}

// TestPublicOnly_IndexStage_CaseDifferentRepoNoCollision calls the index
// generation stage directly (not just the doc-copy stage) to verify that
// repository index files use the same case as the doc-copy stage's
// content/ directory. If the index stage uses the raw Repository name
// while the doc-copy stage uses strings.ToLower(Repository), the
// generated _index.md lands in a different directory than the docs - on
// case-insensitive filesystems this is invisible, but on case-sensitive
// filesystems it produces a "Drift/" sibling next to "drift/", and Hugo
// may treat them as the same section with conflicting index files.
//
// The test calls generateIndexPages with one public doc and asserts that
// the index file lands at content/drift/_index.md (lowercased) - matching
// where the doc was written.
func TestPublicOnly_IndexStage_CaseDifferentRepoNoCollision(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{
		Build:  config.BuildConfig{RenderMode: config.RenderModeNever},
		Hugo:   config.HugoConfig{Title: "Test", BaseURL: "https://www.local"},
		Daemon: &config.DaemonConfig{Content: config.DaemonContentConfig{PublicOnly: true}},
	}
	gen := NewGenerator(cfg, dir)

	publicDoc := docs.DocFile{
		Repository:   "Drift",
		Section:      "img/nett",
		Name:         "page",
		Extension:    ".md",
		RelativePath: "img/nett/page.md",
		DocsBase:     "docs",
		Content:      []byte("---\npublic: true\n---\n# Page\n"),
	}
	otherDoc := docs.DocFile{
		Repository:   "other",
		Section:      "notes",
		Name:         "readme",
		Extension:    ".md",
		RelativePath: "notes/readme.md",
		DocsBase:     "docs",
		Content:      []byte("---\npublic: true\n---\n# Other\n"),
	}
	files := []docs.DocFile{publicDoc, otherDoc}
	if err := gen.copyContentFiles(t.Context(), files); err != nil {
		t.Fatalf("copy: %v", err)
	}
	if err := gen.generateIndexPages(files); err != nil {
		t.Fatalf("generateIndexPages: %v", err)
	}

	// Dump content/ tree for debugging.
	_ = filepath.Walk(filepath.Join(dir, "content"), func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(filepath.Join(dir, "content"), path)
		t.Logf("content file: %s", rel)
		return nil
	})

	driftIndex := filepath.Join(dir, "content", "drift", "_index.md")
	if _, err := os.Stat(driftIndex); err != nil {
		t.Errorf("expected repository index at %s (lowercased) but missing: %v", driftIndex, err)
	}
	// On case-insensitive filesystems (macOS HFS+/APFS, Windows NTFS) the
	// OS aliases "Drift" and "drift" to the same directory, so os.Stat
	// cannot distinguish them. Walk the actual directory entries to see
	// the literal name used.
	entries, err := os.ReadDir(filepath.Join(dir, "content"))
	if err != nil {
		t.Fatalf("read content/: %v", err)
	}
	for _, e := range entries {
		if e.Name() == "Drift" {
			t.Errorf("content/ has a literal 'Drift' entry (should be lowercased to 'drift')")
		}
	}
}

func TestPublicOnly_PageBundleImage_PublishPath(t *testing.T) {
	if _, err := exec.LookPath("hugo"); err != nil {
		t.Skip("hugo binary not found in PATH; skipping")
	}

	cases := []struct {
		name       string
		publicOnly bool
	}{
		{"public_only_on", true},
		{"public_only_off", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()

			assetPath := filepath.Join(dir, "pic.png")
			if err := os.WriteFile(assetPath, []byte{0x89, 0x50, 0x4e, 0x47}, 0o600); err != nil {
				t.Fatalf("write asset: %v", err)
			}

			cfg := &config.Config{
				Build: config.BuildConfig{RenderMode: config.RenderModeNever},
				Hugo:  config.HugoConfig{Title: "Test", BaseURL: "https://www.local"},
			}
			if tc.publicOnly {
				cfg.Daemon = &config.DaemonConfig{
					Content: config.DaemonContentConfig{PublicOnly: true},
				}
			}

			gen := NewGenerator(cfg, dir)

			publicDoc := docs.DocFile{
				Repository:   "drift",
				Section:      "img/nett",
				Name:         "page",
				Extension:    ".md",
				RelativePath: "img/nett/page.md",
				DocsBase:     "docs",
				Content:      []byte("---\npublic: true\n---\n# Page\n\n![Pic](pic.png)\n"),
			}
			privateDoc := docs.DocFile{
				Repository:   "drift",
				Section:      "img/nett",
				Name:         "secret",
				Extension:    ".md",
				RelativePath: "img/nett/secret.md",
				DocsBase:     "docs",
				Content:      []byte("# Secret\n"),
			}
			asset := docs.DocFile{
				Repository:   "drift",
				Section:      "img/nett",
				Name:         "pic",
				Extension:    ".png",
				RelativePath: "img/nett/pic.png",
				DocsBase:     "docs",
				Path:         assetPath,
				IsAsset:      true,
			}
			// Second repo so isSingleRepo=false; the docbuilder pipeline
			// then includes the "drift/" namespace in the Hugo path,
			// matching the production layout.
			otherDoc := docs.DocFile{
				Repository:   "other",
				Section:      "notes",
				Name:         "readme",
				Extension:    ".md",
				RelativePath: "notes/readme.md",
				DocsBase:     "docs",
				Content:      []byte("---\npublic: true\n---\n# Other\n"),
			}

			files := []docs.DocFile{publicDoc, privateDoc, asset, otherDoc}
			if err := gen.copyContentFiles(t.Context(), files); err != nil {
				t.Fatalf("copy: %v", err)
			}

			// Strip the {{ children }} shortcode that the docbuilder
			// pipeline writes into generated _index.md files - it requires
			// the Relearn theme, which we don't load in this test.
			if err := filepath.Walk(filepath.Join(dir, "content"), func(path string, info os.FileInfo, err error) error {
				if err != nil || info.IsDir() {
					return err
				}
				if info.Name() == "_index.md" {
					stripShortcodes(t, path)
				}
				return nil
			}); err != nil {
				t.Fatalf("walk content: %v", err)
			}

			hugoDir := filepath.Join(dir, "hugo")
			if err := copyDir(filepath.Join(dir, "content"), filepath.Join(hugoDir, "content")); err != nil {
				t.Fatalf("copy content: %v", err)
			}

			hugoYAML := "baseURL: https://www.local\n" +
				"title: Test\n" +
				"disableKinds: [\"RSS\", \"sitemap\", \"taxonomy\", \"term\"]\n"
			if err := os.WriteFile(filepath.Join(hugoDir, "hugo.yaml"), []byte(hugoYAML), 0o600); err != nil {
				t.Fatalf("write hugo.yaml: %v", err)
			}

			cmd := exec.CommandContext(t.Context(), "hugo", "--logLevel", "error")
			cmd.Dir = hugoDir
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("hugo failed: %v\n%s", err, string(out))
			}

			publicDir := filepath.Join(hugoDir, "public")
			var walked []string
			_ = filepath.Walk(publicDir, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				rel, _ := filepath.Rel(publicDir, path)
				walked = append(walked, rel)
				return nil
			})
			t.Logf("public/: %v", walked)

			expected := filepath.Join(publicDir, "drift", "img", "nett", "pic.png")
			doubled := filepath.Join(publicDir, "drift", "drift", "img", "nett", "pic.png")

			if _, err := os.Stat(doubled); err == nil {
				t.Errorf("DOUBLED PATH: pic.png was published at %s", doubled)
			}
			if _, err := os.Stat(expected); err != nil {
				t.Errorf("expected pic.png at %s but it is missing", expected)
			}
		})
	}
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o750)
		}
		// #nosec G122,G304 -- test helper operating under t.TempDir() roots
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
			return err
		}
		// #nosec G122,G703 -- test helper writing under t.TempDir() destination
		return os.WriteFile(target, data, 0o600)
	})
}

func stripShortcodes(t *testing.T, path string) {
	t.Helper()
	// #nosec G304 -- test helper reading from a t.TempDir() path
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	s := string(data)
	for {
		start := indexOfAny(s, []string{"{{<", "{{%"})
		if start < 0 {
			break
		}
		closer := ">}}"
		if s[start:start+3] == "{{%" {
			closer = "%}}"
		}
		end := indexOf(s[start:], closer)
		if end < 0 {
			break
		}
		s = s[:start] + s[start+end+len(closer):]
	}
	// #nosec G703 -- test helper writing to a t.TempDir() path
	if err := os.WriteFile(path, []byte(s), 0o600); err != nil {
		t.Fatalf("rewrite %s: %v", path, err)
	}
}

func indexOfAny(s string, subs []string) int {
	best := -1
	for _, sub := range subs {
		if i := indexOf(s, sub); i >= 0 && (best < 0 || i < best) {
			best = i
		}
	}
	return best
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
