package hugo

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/docs"
)

// TestGenerateSiteWithReportContextSetsDocFilesHash ensures the direct generation path
// (bypassing discovery/clone) still computes BuildReport.DocFilesHash.
func TestGenerateSiteWithReportContextSetsDocFilesHash(t *testing.T) {
	out := t.TempDir()
	gen := NewGenerator(&config.Config{Hugo: config.HugoConfig{Title: "Test", BaseURL: "/"}}, out).WithRenderer(&NoopRenderer{})
	files := []docs.DocFile{{Repository: "r1", Name: "page", RelativePath: "page.md", DocsBase: "docs", Extension: ".md", Content: []byte("# Hi\n")}}
	report, err := gen.GenerateSiteWithReportContext(t.Context(), files)
	if err != nil {
		t.Fatalf("GenerateSiteWithReportContext failed: %v", err)
	}
	if report == nil {
		t.Fatalf("expected report")
	}
	if report.DocFilesHash == "" {
		t.Fatalf("expected non-empty DocFilesHash")
	}
	// Sanity: persisted build-report.json contains the same value.
	data, err := os.ReadFile(filepath.Join(out, "build-report.json"))
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	if !bytes.Contains(data, []byte(report.DocFilesHash)) {
		t.Fatalf("persisted report missing hash %s", report.DocFilesHash)
	}
}
