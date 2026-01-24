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
