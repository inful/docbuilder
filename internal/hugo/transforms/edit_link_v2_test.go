package transforms

import (
	"strings"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/docs"
	"git.home.luguber.info/inful/docbuilder/internal/hugo/fmcore"
)

// dummyGenerator minimal provider implementing Config() for generatorProvider access.
// stub resolver implementing the minimal Resolve interface expected by EditLinkInjectorV2.
type stubResolver struct{}

func (s stubResolver) Resolve(df docs.DocFile) string {
	return "https://example.com/edit/" + df.RelativePath
}

type dummyGenerator struct {
	cfg      *config.Config
	resolver stubResolver
}

func (d dummyGenerator) Config() *config.Config { return d.cfg }
func (d dummyGenerator) EditLinkResolver() interface{ Resolve(docs.DocFile) string } {
	return d.resolver
}

// helper to create a baseline DocFile
func testDocFile() docs.DocFile {
	return docs.DocFile{
		Repository:   "repo1",
		RelativePath: "intro.md",
		DocsBase:     "docs",
		Name:         "intro",
		Extension:    ".md",
		Content:      []byte("# Intro\n"),
	}
}

func TestEditLinkInjectorV2_HextraAdds(t *testing.T) {
	cfg := &config.Config{Repositories: []config.Repository{{Name: "repo1", URL: "https://github.com/example/repo1.git", Branch: "main", Paths: []string{"docs"}}}}
	SetGeneratorProvider(func() any { return dummyGenerator{cfg: cfg, resolver: stubResolver{}} })
	doc := testDocFile()
	shim := &PageShim{Doc: doc}
	// Run builder then edit link injector
	if err := (FrontMatterBuilderV2{}).Transform(shim); err != nil {
		t.Fatalf("builder v2 error: %v", err)
	}
	if err := (EditLinkInjectorV2{}).Transform(shim); err != nil {
		t.Fatalf("edit link v2 error: %v", err)
	}
	// Assert patch with Source edit_link_v2 present
	found := false
	for _, p := range shim.Patches {
		if p.Source == "edit_link_v2" {
			found = true
			if p.Data == nil || p.Data["editURL"] == nil {
				t.Fatalf("edit_link_v2 patch missing editURL key: %+v", p.Data)
			}
			if mode := p.Mode; mode != fmcore.MergeSetIfMissing {
				t.Fatalf("expected MergeSetIfMissing mode, got %v", mode)
			}
			if val, _ := p.Data["editURL"].(string); !strings.Contains(val, "/edit/") && !strings.Contains(val, "/_edit/") && !strings.Contains(val, "/src/") {
				// Accept various forge patterns; basic sanity check presence of URL
				if !strings.HasPrefix(val, "https://") {
					t.Fatalf("unexpected editURL value: %q", val)
				}
			}
		}
	}
	if !found {
		t.Fatalf("expected edit_link_v2 patch to be added; patches=%+v", shim.Patches)
	}
}

func TestEditLinkInjectorV2_RespectsExisting(t *testing.T) {
	cfg := &config.Config{Repositories: []config.Repository{{Name: "repo1", URL: "https://github.com/example/repo1.git", Branch: "main", Paths: []string{"docs"}}}}
	SetGeneratorProvider(func() any { return dummyGenerator{cfg: cfg, resolver: stubResolver{}} })
	doc := testDocFile()
	shim := &PageShim{Doc: doc, OriginalFrontMatter: map[string]any{"editURL": "https://custom/edit"}, HadFrontMatter: true}
	if err := (FrontMatterBuilderV2{}).Transform(shim); err != nil {
		t.Fatalf("builder v2 error: %v", err)
	}
	if err := (EditLinkInjectorV2{}).Transform(shim); err != nil {
		t.Fatalf("edit link v2 error: %v", err)
	}
	for _, p := range shim.Patches {
		if p.Source == "edit_link_v2" {
			t.Fatalf("did not expect edit_link_v2 patch when original front matter already had editURL; patches=%+v", shim.Patches)
		}
	}
}

func TestEditLinkInjectorV2_DocsyAlsoGeneratesEditLinks(t *testing.T) {
	cfg := &config.Config{Repositories: []config.Repository{{Name: "repo1", URL: "https://github.com/example/repo1.git", Branch: "main", Paths: []string{"docs"}}}}
	SetGeneratorProvider(func() any { return dummyGenerator{cfg: cfg, resolver: stubResolver{}} })
	doc := testDocFile()
	shim := &PageShim{Doc: doc}
	if err := (FrontMatterBuilderV2{}).Transform(shim); err != nil {
		t.Fatalf("builder v2 error: %v", err)
	}
	if err := (EditLinkInjectorV2{}).Transform(shim); err != nil {
		t.Fatalf("edit link v2 error: %v", err)
	}
	// Verify edit link is generated for docsy theme too
	found := false
	for _, p := range shim.Patches {
		if p.Source == "edit_link_v2" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected edit_link_v2 patch for docsy theme; patches=%+v", shim.Patches)
	}
}
