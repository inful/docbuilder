package hugo

import (
	"context"
	"os"
	"strings"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/docs"
	tr "git.home.luguber.info/inful/docbuilder/internal/hugo/transforms"
)

// custom marker transform to test filtering without affecting existing registry order.
type markerTransform struct{}

func (m markerTransform) Name() string  { return "_marker" }
func (m markerTransform) Priority() int { return 55 } // between rewrite (50) and serialize (90)
func (m markerTransform) Transform(p tr.PageAdapter) error {
	if shim, ok := p.(*tr.PageShim); ok {
		shim.Content = shim.Content + "\nMARKER"
	}
	return nil
}

// We register the marker inside test to avoid polluting global registry for other tests; copy List then restore.
func TestTransformFiltering_EnableDisable(t *testing.T) {
	// Snapshot registry and restore after
	snap := tr.SnapshotForTest()
	defer tr.RestoreForTest(snap)
	tr.Register(markerTransform{})
	// Build config disabling marker
	cfg := &config.Config{Hugo: config.HugoConfig{Title: "Filtering", Theme: "hextra", Transforms: &config.HugoTransforms{Disable: []string{"_marker"}}}, Forges: []*config.ForgeConfig{{Name: "f", Type: "github", Auth: &config.AuthConfig{Type: "token", Token: "x"}, Organizations: []string{"org"}}}, Output: config.OutputConfig{Directory: t.TempDir()}}
	g := NewGenerator(cfg, t.TempDir())
	df := docs.DocFile{Name: "page", Repository: "repo1", RelativePath: "docs/page.md", Content: []byte("# Title\n")}
	if err := g.copyContentFiles(context.Background(), []docs.DocFile{df}); err != nil {
		t.Fatalf("copyContentFiles: %v", err)
	}
	// Read generated file
	outPath := g.buildRoot() + "/" + df.GetHugoPath()
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if strings.Contains(string(data), "MARKER") {
		t.Fatalf("marker transform should have been disabled\n%s", string(data))
	}

	// Now enable only marker (restrict others) – expect marker appears and basic serialization still happens.
	cfg2 := &config.Config{Hugo: config.HugoConfig{Title: "Filtering", Theme: "hextra", Transforms: &config.HugoTransforms{Enable: []string{"_marker", "front_matter_parser", "front_matter_builder_v2", "front_matter_merge", "front_matter_serialize"}}}, Forges: []*config.ForgeConfig{{Name: "f", Type: "github", Auth: &config.AuthConfig{Type: "token", Token: "x"}, Organizations: []string{"org"}}}, Output: config.OutputConfig{Directory: t.TempDir()}}
	g2 := NewGenerator(cfg2, t.TempDir())
	if err := g2.copyContentFiles(context.Background(), []docs.DocFile{df}); err != nil {
		t.Fatalf("copyContentFiles(2): %v", err)
	}
	outPath2 := g2.buildRoot() + "/" + df.GetHugoPath()
	data2, err := os.ReadFile(outPath2)
	if err != nil {
		t.Fatalf("read output2: %v", err)
	}
	if !strings.Contains(string(data2), "MARKER") {
		t.Fatalf("marker transform should have run\n%s", string(data2))
	}
	// sanity: relative link rewriter omitted (not enabled) so rewriting of nonexistent links irrelevant here; we just rely on presence of marker.

}
