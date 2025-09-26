package hugo

import (
    "strings"
    "testing"

    "git.home.luguber.info/inful/docbuilder/internal/config"
    "git.home.luguber.info/inful/docbuilder/internal/docs"
)

func TestGenerateSiteWithReport(t *testing.T) {
    outDir := t.TempDir()
    cfg := &config.Config{Hugo: config.HugoConfig{Title: "R", Theme: "hextra"}, Repositories: []config.Repository{{Name: "r1", URL: "https://github.com/o/r1.git"}}}
    files := []docs.DocFile{{Repository: "r1", Name: "p", RelativePath: "p.md", DocsBase: "docs", Extension: ".md", Content: []byte("Hello")}}
    gen := NewGenerator(cfg, outDir)
    rep, err := gen.GenerateSiteWithReport(files)
    if err != nil { t.Fatalf("generation failed: %v", err) }
    if rep.Repositories != 1 || rep.Files != 1 { t.Fatalf("unexpected counts: %+v", rep) }
    if rep.End.IsZero() { t.Fatalf("report end time not set") }
    if !strings.Contains(rep.Summary(), "repos=1") { t.Fatalf("summary unexpected: %s", rep.Summary()) }
}
