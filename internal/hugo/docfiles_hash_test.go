package hugo

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/docs"
)

// TestDocFilesHashChanges ensures BuildReport.DocFilesHash changes when the discovered doc file set changes.
func TestDocFilesHashChanges(t *testing.T) {
	out := t.TempDir()
	cfg := &config.Config{Hugo: config.HugoConfig{Theme: "hextra"}}
	gen := NewGenerator(cfg, out).WithRenderer(&NoopRenderer{})

	filesA := []docs.DocFile{{Repository: "r", Name: "a", RelativePath: "a.md", DocsBase: "docs", Extension: ".md", Content: []byte("# A\n")}}
	if err := gen.GenerateSite(filesA); err != nil {
		t.Fatalf("first build failed: %v", err)
	}
	hashA := readHash(t, filepath.Join(out, "build-report.json"))
	if hashA == "" {
		t.Fatalf("expected non-empty hash for first build")
	}

	// Second build with same files -> hash should remain identical
	gen2 := NewGenerator(cfg, out).WithRenderer(&NoopRenderer{})
	if err := gen2.GenerateSite(filesA); err != nil {
		t.Fatalf("second build failed: %v", err)
	}
	hashA2 := readHash(t, filepath.Join(out, "build-report.json"))
	if hashA2 != hashA {
		t.Fatalf("expected identical hash when file set unchanged: %s vs %s", hashA2, hashA)
	}

	// Third build with additional file -> hash must change
	filesB := append(filesA, docs.DocFile{Repository: "r", Name: "b", RelativePath: "b.md", DocsBase: "docs", Extension: ".md", Content: []byte("# B\n")})
	gen3 := NewGenerator(cfg, out).WithRenderer(&NoopRenderer{})
	if err := gen3.GenerateSite(filesB); err != nil {
		t.Fatalf("third build failed: %v", err)
	}
	hashB := readHash(t, filepath.Join(out, "build-report.json"))
	if hashB == hashA {
		t.Fatalf("expected hash to change after adding file; still %s", hashB)
	}
}

func readHash(t *testing.T, path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(b, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	v, _ := parsed["doc_files_hash"].(string)
	return v
}
