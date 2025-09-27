package hugo

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/docs"
)

// TestReportPersistence_Success ensures report files are written on success.
func TestReportPersistence_Success(t *testing.T) {
	out := t.TempDir()
	cfg := &config.V2Config{Hugo: config.HugoConfig{Theme: "hextra"}}
	gen := NewGenerator(cfg, out)
	files := []docs.DocFile{{Repository: "r", Name: "p", RelativePath: "p.md", DocsBase: "docs", Extension: ".md", Content: []byte("# Hello\n")}}
	if err := gen.GenerateSite(files); err != nil {
		t.Fatalf("build failed: %v", err)
	}
	jsonPath := filepath.Join(out, "build-report.json")
	if _, err := os.Stat(jsonPath); err != nil {
		t.Fatalf("expected report json: %v", err)
	}
	b, _ := os.ReadFile(jsonPath)
	var parsed map[string]any
	if err := json.Unmarshal(b, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if parsed["outcome"] != "success" {
		t.Fatalf("expected outcome=success got %v", parsed["outcome"])
	}
	if parsed["rendered_pages"].(float64) < 1 {
		t.Fatalf("expected rendered_pages >=1, got %v", parsed["rendered_pages"])
	}
}

// TestReportPersistence_FailureDoesNotOverwrite verifies that a failed build leaves existing report intact.
func TestReportPersistence_FailureDoesNotOverwrite(t *testing.T) {
	out := t.TempDir()
	cfg := &config.V2Config{Hugo: config.HugoConfig{Theme: "hextra"}}
	gen := NewGenerator(cfg, out)
	baseFiles := []docs.DocFile{{Repository: "r", Name: "base", RelativePath: "base.md", DocsBase: "docs", Extension: ".md", Content: []byte("# Base\n")}}
	if err := gen.GenerateSite(baseFiles); err != nil {
		t.Fatalf("initial build failed: %v", err)
	}
	info, err := os.Stat(filepath.Join(out, "build-report.json"))
	if err != nil {
		t.Fatalf("missing initial report: %v", err)
	}
	initialMod := info.ModTime()

	// Now attempt a canceled build
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	gen2 := NewGenerator(cfg, out)
	if _, err := gen2.GenerateSiteWithReportContext(ctx, []docs.DocFile{{Repository: "r", Name: "fail", RelativePath: "fail.md", DocsBase: "docs", Extension: ".md", Content: []byte("# Fail\n")}}); err == nil {
		t.Fatalf("expected cancellation error")
	}
	info2, err := os.Stat(filepath.Join(out, "build-report.json"))
	if err != nil {
		t.Fatalf("report disappeared after failed build: %v", err)
	}
	if info2.ModTime().After(initialMod) {
		t.Fatalf("report was overwritten on failed build")
	}
}
