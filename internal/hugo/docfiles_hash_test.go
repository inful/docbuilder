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
	gen := NewGenerator(&config.Config{Hugo: config.HugoConfig{Title: "Test", BaseURL: "/"}}, out).WithRenderer(&NoopRenderer{})

	files := []docs.DocFile{{Repository: "r", Name: "a", RelativePath: "a.md", DocsBase: "docs", Extension: ".md", Content: []byte("# A\n")}}
	if err := gen.GenerateSite(files); err != nil {
		t.Fatalf("first build failed: %v", err)
	}
	firstHash := readHash(t, filepath.Join(out, "build-report.json"))
	if firstHash == "" {
		t.Fatalf("expected non-empty hash for first build")
	}

	// Second build with same files -> hash should remain identical
	gen2 := NewGenerator(&config.Config{Hugo: config.HugoConfig{Title: "Test", BaseURL: "/"}}, out).WithRenderer(&NoopRenderer{})
	if err := gen2.GenerateSite(files); err != nil {
		t.Fatalf("second build failed: %v", err)
	}
	secondHash := readHash(t, filepath.Join(out, "build-report.json"))
	if secondHash != firstHash {
		t.Fatalf("expected identical hash when file set unchanged: %s vs %s", secondHash, firstHash)
	}

	// Third build with additional file -> hash must change
	files = append(files, docs.DocFile{Repository: "r", Name: "b", RelativePath: "b.md", DocsBase: "docs", Extension: ".md", Content: []byte("# B\n")})
	gen3 := NewGenerator(&config.Config{Hugo: config.HugoConfig{Title: "Test", BaseURL: "/"}}, out).WithRenderer(&NoopRenderer{})
	if err := gen3.GenerateSite(files); err != nil {
		t.Fatalf("third build failed: %v", err)
	}
	thirdHash := readHash(t, filepath.Join(out, "build-report.json"))
	if thirdHash == firstHash {
		t.Fatalf("expected hash to change after adding file; still %s", thirdHash)
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
