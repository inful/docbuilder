package hugo

import (
	"os"
	"path/filepath"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/docs"
)

func TestPublicOnly_FiltersMarkdownButKeepsAssetsAndScopesIndexes(t *testing.T) {
	cfg := &config.Config{
		Hugo: config.HugoConfig{Title: "Test", BaseURL: "/"},
		Daemon: &config.DaemonConfig{
			Content: config.DaemonContentConfig{PublicOnly: true},
		},
	}
	gen := NewGenerator(cfg, t.TempDir())

	assetSrc := filepath.Join(t.TempDir(), "img.png")
	if err := os.WriteFile(assetSrc, []byte{0x01, 0x02, 0x03}, 0o600); err != nil {
		t.Fatalf("write asset: %v", err)
	}

	publicDoc := docs.DocFile{Repository: "repo1", Name: "pub", Extension: ".md", RelativePath: "pub.md", Content: []byte("---\npublic: true\n---\n# Public\n")}
	privateDoc := docs.DocFile{Repository: "repo2", Name: "priv", Extension: ".md", RelativePath: "priv.md", Content: []byte("# Private\n")}
	asset := docs.DocFile{Repository: "repo2", Name: "img", Extension: ".png", RelativePath: "img.png", Path: assetSrc, IsAsset: true}

	files := []docs.DocFile{publicDoc, privateDoc, asset}
	if err := gen.copyContentFiles(t.Context(), files); err != nil {
		t.Fatalf("copy: %v", err)
	}

	isSingleRepo := false

	publicOut := filepath.Join(gen.BuildRoot(), publicDoc.GetHugoPath(isSingleRepo))
	if _, err := os.Stat(publicOut); err != nil {
		t.Fatalf("expected public page to exist at %s: %v", publicOut, err)
	}
	// #nosec G304 -- test file reading from controlled test output
	publicBytes, err := os.ReadFile(publicOut)
	if err != nil {
		t.Fatalf("read public page: %v", err)
	}
	if containsAll(string(publicBytes), []string{"editURL:"}) {
		t.Fatalf("expected public-only mode to omit editURL, got: %s", string(publicBytes))
	}

	privateOut := filepath.Join(gen.BuildRoot(), privateDoc.GetHugoPath(isSingleRepo))
	if _, statErr := os.Stat(privateOut); statErr == nil {
		t.Fatalf("expected private page to be excluded, but exists at %s", privateOut)
	}

	assetOut := filepath.Join(gen.BuildRoot(), asset.GetHugoPath(isSingleRepo))
	if _, statErr := os.Stat(assetOut); statErr != nil {
		t.Fatalf("expected asset to be copied at %s: %v", assetOut, statErr)
	}

	rootIdx := filepath.Join(gen.BuildRoot(), "content", "_index.md")
	// #nosec G304 -- test file reading from controlled test output
	data, err := os.ReadFile(rootIdx)
	if err != nil {
		t.Fatalf("expected root index generated: %v", err)
	}
	if len(data) == 0 || !containsAll(string(data), []string{"public: true"}) {
		t.Fatalf("expected root index to include public: true, got: %s", string(data))
	}

	repo1Idx := filepath.Join(gen.BuildRoot(), "content", "repo1", "_index.md")
	// #nosec G304 -- test file reading from controlled test output
	data, err = os.ReadFile(repo1Idx)
	if err != nil {
		t.Fatalf("expected repo1 index generated: %v", err)
	}
	if !containsAll(string(data), []string{"public: true"}) {
		t.Fatalf("expected repo1 index to include public: true, got: %s", string(data))
	}

	repo2Idx := filepath.Join(gen.BuildRoot(), "content", "repo2", "_index.md")
	if _, err := os.Stat(repo2Idx); err == nil {
		t.Fatalf("expected repo2 index to be omitted (no public pages), but exists at %s", repo2Idx)
	}
}

// TestPublicOnly_PublicDocWithImageLink_AssetAndLinkAgree reproduces the
// production symptom where enabling daemon.content.public_only causes images
// to render with broken links or to be missing/incorrectly placed.
//
// Setup:
//   - public_only: true
//   - One public doc in repo1/guides/advanced/install.md referencing
//     ![Alt](images/foo.png) (relative to the doc).
//   - A sibling private doc in the same section (filtered out by public_only).
//   - The image asset images/foo.png.
//
// Expected behaviour (matches the no-public_only instance):
//   - The kept public doc's image link is rewritten to an absolute path
//     pointing at the asset's Hugo location.
//   - The asset is copied to that same Hugo location.
//
// Bug (symptom reported in production):
//   - Asset and link disagree (asset in repo1/guides/images/, link in
//     repo1/guides/advanced/images/; or asset missing entirely).
func TestPublicOnly_PublicDocWithImageLink_AssetAndLinkAgree(t *testing.T) {
	cfg := &config.Config{
		Hugo: config.HugoConfig{Title: "Test", BaseURL: "/"},
		Daemon: &config.DaemonConfig{
			Content: config.DaemonContentConfig{PublicOnly: true},
		},
	}
	gen := NewGenerator(cfg, t.TempDir())

	assetSrc := filepath.Join(t.TempDir(), "foo.png")
	if err := os.WriteFile(assetSrc, []byte{0x89, 0x50, 0x4e, 0x47}, 0o600); err != nil {
		t.Fatalf("write asset: %v", err)
	}

	publicDoc := docs.DocFile{
		Repository:   "repo1",
		Section:      "guides/advanced",
		Name:         "install",
		Extension:    ".md",
		RelativePath: "guides/advanced/install.md",
		Content: []byte("---\npublic: true\n---\n# Install\n\n" +
			"![Diagram](images/foo.png)\n"),
	}
	privateDoc := docs.DocFile{
		Repository:   "repo1",
		Section:      "guides/advanced",
		Name:         "internal",
		Extension:    ".md",
		RelativePath: "guides/advanced/internal.md",
		Content:      []byte("# Internal\n"),
	}
	asset := docs.DocFile{
		Repository:   "repo1",
		Section:      "guides/advanced",
		Name:         "foo",
		Extension:    ".png",
		RelativePath: "guides/advanced/images/foo.png",
		Path:         assetSrc,
		IsAsset:      true,
	}
	// Second repo so isSingleRepo=false in the pipeline; otherwise the
	// repository namespace is omitted and the link/asset paths below don't
	// match the production scenario.
	otherDoc := docs.DocFile{
		Repository:   "repo2",
		Section:      "notes",
		Name:         "readme",
		Extension:    ".md",
		RelativePath: "notes/readme.md",
		Content:      []byte("---\npublic: true\n---\n# Notes\n"),
	}

	files := []docs.DocFile{publicDoc, privateDoc, asset, otherDoc}
	if err := gen.copyContentFiles(t.Context(), files); err != nil {
		t.Fatalf("copy: %v", err)
	}

	const expectedLink = "/repo1/guides/advanced/images/foo.png"
	publicOut := filepath.Join(gen.BuildRoot(), publicDoc.GetHugoPath(false))
	// #nosec G304 -- test file reading from controlled test output
	publicBytes, err := os.ReadFile(publicOut)
	if err != nil {
		t.Fatalf("read public page: %v", err)
	}
	if !stringsContains(string(publicBytes), expectedLink) {
		t.Fatalf("expected rewritten image link %q in public doc, got:\n%s",
			expectedLink, string(publicBytes))
	}

	assetOut := filepath.Join(gen.BuildRoot(), asset.GetHugoPath(false))
	if _, err := os.Stat(assetOut); err != nil {
		t.Fatalf("expected asset at %s (where the link points): %v", assetOut, err)
	}

	if !fileExists(filepath.Join(gen.BuildRoot(), "content", "repo1", "guides", "advanced", "_index.md")) {
		t.Fatalf("expected section _index.md to be generated for the public doc's section")
	}
}

func TestPublicOnly_ZeroPublicPages_ProducesNoIndexes(t *testing.T) {
	cfg := &config.Config{
		Hugo: config.HugoConfig{Title: "Test", BaseURL: "/"},
		Daemon: &config.DaemonConfig{
			Content: config.DaemonContentConfig{PublicOnly: true},
		},
	}
	gen := NewGenerator(cfg, t.TempDir())

	privateDoc := docs.DocFile{Repository: "repo", Name: "priv", Extension: ".md", RelativePath: "priv.md", Content: []byte("# Private\n")}
	if err := gen.copyContentFiles(t.Context(), []docs.DocFile{privateDoc}); err != nil {
		t.Fatalf("copy: %v", err)
	}

	rootIdx := filepath.Join(gen.BuildRoot(), "content", "_index.md")
	if _, err := os.Stat(rootIdx); err == nil {
		t.Fatalf("expected no root index when zero public pages, but %s exists", rootIdx)
	}

	privateOut := filepath.Join(gen.BuildRoot(), privateDoc.GetHugoPath(true))
	if _, err := os.Stat(privateOut); err == nil {
		t.Fatalf("expected private page to be excluded, but exists at %s", privateOut)
	}
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func containsAll(s string, parts []string) bool {
	for _, p := range parts {
		if !stringsContains(s, p) {
			return false
		}
	}
	return true
}

func stringsContains(s, substr string) bool {
	// avoid importing strings in every test file; keep helper tiny
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
