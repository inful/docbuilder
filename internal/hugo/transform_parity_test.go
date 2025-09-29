package hugo

import (
  "crypto/sha256"
  "encoding/hex"
  "fmt"
  "testing"
  "time"
  tr "git.home.luguber.info/inful/docbuilder/internal/hugo/transforms"
  "git.home.luguber.info/inful/docbuilder/internal/config"
  "git.home.luguber.info/inful/docbuilder/internal/docs"
  "gopkg.in/yaml.v3"
)

// runLegacyPipeline runs existing inline pipeline for a single page and returns hash of raw output.
func runLegacyPipeline(g *Generator, df docs.DocFile) (string, error) {
  p := &Page{File: df, Raw: df.Content, Content: string(df.Content), OriginalFrontMatter: nil, Patches: nil}
  pipe := NewTransformerPipeline(
    &FrontMatterParser{},
    &FrontMatterBuilder{ConfigProvider: func() *Generator { return g }},
    &EditLinkInjector{ConfigProvider: func() *Generator { return g }},
    &MergeFrontMatterTransformer{},
    &RelativeLinkRewriter{},
    &FinalFrontMatterSerializer{},
  )
  if err := pipe.Run(p); err != nil { return "", err }
  h := sha256.Sum256(p.Raw)
  return hex.EncodeToString(h[:]), nil
}

// runRegistryPipeline runs registry transformers producing equivalent output hash.
func runRegistryPipeline(g *Generator, df docs.DocFile) (string, error) {
  regs := tr.List()
  p := &Page{File: df, Raw: df.Content, Content: string(df.Content), OriginalFrontMatter: nil, Patches: nil}
  // replicate logic from content_copy.go (trimmed for test)
  shim := &tr.PageShim{
    FilePath: df.RelativePath,
    Content: p.Content,
    OriginalFrontMatter: p.OriginalFrontMatter,
    HadFrontMatter: p.HadFrontMatter,
    BuildFrontMatter: func(now time.Time) {
      built := BuildFrontMatter(FrontMatterInput{File: p.File, Existing: p.OriginalFrontMatter, Config: g.config, Now: now})
      p.Patches = append(p.Patches, FrontMatterPatch{Source: "builder", Mode: MergeDeep, Priority: 50, Data: built})
    },
    InjectEditLink: func() {
      if p.OriginalFrontMatter != nil { if _, ok := p.OriginalFrontMatter["editURL"]; ok { return } }
      for _, patch := range p.Patches { if patch.Data != nil { if _, ok := patch.Data["editURL"]; ok { return } } }
      if g.editLinkResolver == nil { return }
      val := g.editLinkResolver.Resolve(p.File)
      if val == "" { return }
      p.Patches = append(p.Patches, FrontMatterPatch{Source: "edit_link", Mode: MergeSetIfMissing, Priority: 60, Data: map[string]any{"editURL": val}})
    },
    ApplyPatches: func() { p.applyPatches() },
    RewriteLinks: func(s string) string { return RewriteRelativeMarkdownLinks(s) },
    SyncOriginal: func(fm map[string]any, had bool) { p.OriginalFrontMatter = fm; p.HadFrontMatter = had },
  }
  shim.Serialize = func() error {
    if p.MergedFrontMatter == nil { p.applyPatches() }
    p.Content = shim.Content
    fm := p.MergedFrontMatter
    if fm == nil { fm = map[string]any{} }
    raw, err := serializeFrontMatterAndContent(fm, p.Content)
    if err != nil { return err }
    p.Raw = raw
    return nil
  }
  for _, rt := range regs {
    if err := rt.Transform(shim); err != nil { return "", err }
  }
  h := sha256.Sum256(p.Raw)
  return hex.EncodeToString(h[:]), nil
}

// serializeFrontMatterAndContent duplicates minimal logic for test locality.
func serializeFrontMatterAndContent(fm map[string]any, body string) ([]byte, error) {
  if fm == nil { fm = map[string]any{} }
  // Normalize volatile keys (if any appear) to stable placeholder
  if _, ok := fm["build_date"]; ok { fm["build_date"] = "IGNORE" }
  y, err := yaml.Marshal(fm)
  if err != nil { return nil, err }
  return []byte("---\n" + string(y) + "---\n" + body), nil
}

func TestRegistryParity_Basic(t *testing.T) {
  cfg := &config.Config{Hugo: config.HugoConfig{Title: "Parity", Theme: "hextra"}, Repositories: []config.Repository{{Name: "repo1", URL: "https://github.com/org/repo1.git", Branch: "main", Paths: []string{"docs"}}}}
  g := NewGenerator(cfg, t.TempDir())
  df := docs.DocFile{Name: "intro", Repository: "repo1", RelativePath: "docs/intro.md", Path: "", Section: "", Content: []byte("# Intro\n\nSee [Guide](guide.md).\n")}
  legacy, err := runLegacyPipeline(g, df)
  if err != nil { t.Fatalf("legacy pipeline error: %v", err) }
  reg, err := runRegistryPipeline(g, df)
  if err != nil { t.Fatalf("registry pipeline error: %v", err) }
  if legacy != reg { t.Fatalf("hash mismatch legacy=%s registry=%s", legacy, reg) }
}

// parityHashPair computes both hashes for a given content fixture.
func parityHashPair(t *testing.T, g *Generator, content string) (legacy, reg string) {
  t.Helper()
  df := docs.DocFile{Name: "test", Repository: "repo1", RelativePath: "docs/test.md", Content: []byte(content)}
  l, err := runLegacyPipeline(g, df); if err != nil { t.Fatalf("legacy: %v", err) }
  r, err := runRegistryPipeline(g, df); if err != nil { t.Fatalf("registry: %v", err) }
  return l, r
}

func TestRegistryParity_ExistingFrontMatterProtectedKeys(t *testing.T) {
  cfg := &config.Config{Hugo: config.HugoConfig{Title: "Parity", Theme: "hextra"}, Repositories: []config.Repository{{Name: "repo1", URL: "https://github.com/org/repo1.git", Branch: "main", Paths: []string{"docs"}}}}
  g := NewGenerator(cfg, t.TempDir())
  // Existing title should be preserved; builder attempts to set title from filename if missing.
  body := "---\ntitle: Custom Title\ndescription: Existing desc\n---\n# Heading\n"
  l, r := parityHashPair(t, g, body)
  if l != r {
    // produce verbose diff
    df := docs.DocFile{Name: "test", Repository: "repo1", RelativePath: "docs/test.md", Content: []byte(body)}
    // recreate raw outputs
    rawLegacy := debugRawLegacy(t, g, df)
    rawReg := debugRawRegistry(t, g, df)
    t.Fatalf("hash mismatch protected keys: %s vs %s\nLEGACY:\n%s\n---\nREGISTRY:\n%s\n---", l, r, rawLegacy, rawReg)
  }
}

func TestRegistryParity_TaxonomyUnion(t *testing.T) {
  cfg := &config.Config{Hugo: config.HugoConfig{Title: "Parity", Theme: "hextra"}, Repositories: []config.Repository{{Name: "repo1", URL: "https://github.com/org/repo1.git", Branch: "main", Paths: []string{"docs"}}}}
  g := NewGenerator(cfg, t.TempDir())
  // Original tags plus builder may add none; ensure union logic identical.
  body := "---\ntags: [alpha, beta]\n---\nContent referencing [link](other.md).\n"
  l, r := parityHashPair(t, g, body)
  if l != r {
    df := docs.DocFile{Name: "test", Repository: "repo1", RelativePath: "docs/test.md", Content: []byte(body)}
    t.Fatalf("hash mismatch taxonomy union: %s vs %s\n%s", l, r, debugDiff(t, g, df))
  }
}

func TestRegistryParity_ArrayStrategies_ResourcesAppend(t *testing.T) {
  cfg := &config.Config{Hugo: config.HugoConfig{Title: "Parity", Theme: "hextra"}, Repositories: []config.Repository{{Name: "repo1", URL: "https://github.com/org/repo1.git", Branch: "main", Paths: []string{"docs"}}}}
  g := NewGenerator(cfg, t.TempDir())
  // resources default strategy is append when existing.
  body := "---\nresources: [{src: img1.png}]\n---\n# Doc\n"
  l, r := parityHashPair(t, g, body)
  if l != r { df := docs.DocFile{Name: "test", Repository: "repo1", RelativePath: "docs/test.md", Content: []byte(body)}; t.Fatalf("hash mismatch resources append: %s vs %s\n%s", l, r, debugDiff(t, g, df)) }
}

func TestRegistryParity_DeepMergeNestedMaps(t *testing.T) {
  cfg := &config.Config{Hugo: config.HugoConfig{Title: "Parity", Theme: "hextra"}, Repositories: []config.Repository{{Name: "repo1", URL: "https://github.com/org/repo1.git", Branch: "main", Paths: []string{"docs"}}}}
  g := NewGenerator(cfg, t.TempDir())
  // existing nested params should be preserved and merged.
  body := "---\nparams:\n  section:\n    enabled: true\n    weight: 5\n---\n# Doc\n"
  l, r := parityHashPair(t, g, body)
  if l != r { df := docs.DocFile{Name: "test", Repository: "repo1", RelativePath: "docs/test.md", Content: []byte(body)}; t.Fatalf("hash mismatch deep merge nested maps: %s vs %s\n%s", l, r, debugDiff(t, g, df)) }
}

func TestRegistryParity_SetIfMissingConflictLogging(t *testing.T) {
  cfg := &config.Config{Hugo: config.HugoConfig{Title: "Parity", Theme: "hextra"}, Repositories: []config.Repository{{Name: "repo1", URL: "https://github.com/org/repo1.git", Branch: "main", Paths: []string{"docs"}}}}
  g := NewGenerator(cfg, t.TempDir())
  // editURL already present should prevent injector from adding patch; parity should still hold.
  body := "---\neditURL: https://example.com/custom/edit\n---\n# Doc\n"
  l, r := parityHashPair(t, g, body)
  if l != r { df := docs.DocFile{Name: "test", Repository: "repo1", RelativePath: "docs/test.md", Content: []byte(body)}; t.Fatalf("hash mismatch set-if-missing editURL: %s vs %s\n%s", l, r, debugDiff(t, g, df)) }
}

// debug helpers
func debugRawLegacy(t *testing.T, g *Generator, df docs.DocFile) string {
  t.Helper()
  p := &Page{File: df, Raw: df.Content, Content: string(df.Content)}
  pipe := NewTransformerPipeline(&FrontMatterParser{}, &FrontMatterBuilder{ConfigProvider: func() *Generator { return g }}, &EditLinkInjector{ConfigProvider: func() *Generator { return g }}, &MergeFrontMatterTransformer{}, &RelativeLinkRewriter{}, &FinalFrontMatterSerializer{})
  if err := pipe.Run(p); err != nil { t.Fatalf("debug legacy run: %v", err) }
  return string(p.Raw)
}
func debugRawRegistry(t *testing.T, g *Generator, df docs.DocFile) string {
  t.Helper()
  regs := tr.List()
  p := &Page{File: df, Raw: df.Content, Content: string(df.Content)}
  shim := &tr.PageShim{FilePath: df.RelativePath, Content: p.Content, OriginalFrontMatter: p.OriginalFrontMatter, HadFrontMatter: p.HadFrontMatter,
    BuildFrontMatter: func(now time.Time) { built := BuildFrontMatter(FrontMatterInput{File: p.File, Existing: p.OriginalFrontMatter, Config: g.config, Now: now}); p.Patches = append(p.Patches, FrontMatterPatch{Source: "builder", Mode: MergeDeep, Priority: 50, Data: built}) },
    InjectEditLink: func() { if p.OriginalFrontMatter != nil { if _, ok := p.OriginalFrontMatter["editURL"]; ok { return } }; for _, patch := range p.Patches { if patch.Data != nil { if _, ok := patch.Data["editURL"]; ok { return } } }; if g.editLinkResolver == nil { return }; val := g.editLinkResolver.Resolve(p.File); if val == "" { return }; p.Patches = append(p.Patches, FrontMatterPatch{Source: "edit_link", Mode: MergeSetIfMissing, Priority: 60, Data: map[string]any{"editURL": val}}) },
    ApplyPatches: func() { p.applyPatches() }, RewriteLinks: func(s string) string { return RewriteRelativeMarkdownLinks(s) }, SyncOriginal: func(fm map[string]any, had bool) { p.OriginalFrontMatter = fm; p.HadFrontMatter = had }}
  shim.Serialize = func() error { if p.MergedFrontMatter == nil { p.applyPatches() }; p.Content = shim.Content; fm := p.MergedFrontMatter; if fm == nil { fm = map[string]any{} }; raw, err := serializeFrontMatterAndContent(fm, p.Content); if err != nil { return err }; p.Raw = raw; return nil }
  for _, rt := range regs { if err := rt.Transform(shim); err != nil { t.Fatalf("debug registry run: %v", err) } }
  return string(p.Raw)
}
func debugDiff(t *testing.T, g *Generator, df docs.DocFile) string {
  return fmt.Sprintf("LEGACY:\n%s\n---\nREGISTRY:\n%s\n---", debugRawLegacy(t, g, df), debugRawRegistry(t, g, df))
}

// debug helpers

func dumpFrontMatter(t *testing.T, p *Page) {
  t.Logf("YAML Front Matter (%s):", p.File.RelativePath)
  for _, patch := range p.Patches {
    t.Logf("  %s: %v", patch.Source, patch.Data)
  }
  t.Logf("  ---")
  fm := p.MergedFrontMatter
  if fm == nil { fm = map[string]any{} }
  y, err := yaml.Marshal(fm)
  if err != nil { t.Logf("  marshal error: %v", err) }
  t.Logf("  %s", y)
}

func dumpPage(t *testing.T, p *Page) {
  t.Logf("Page Dump (%s):", p.File.RelativePath)
  t.Logf("  Content: %s", p.Content)
  t.Logf("  Raw: %x", p.Raw)
  dumpFrontMatter(t, p)
}

func TestRegistryParity_DebugDiffs(t *testing.T) {
  cfg := &config.Config{Hugo: config.HugoConfig{Title: "Parity", Theme: "hextra"}, Repositories: []config.Repository{{Name: "repo1", URL: "https://github.com/org/repo1.git", Branch: "main", Paths: []string{"docs"}}}}
  g := NewGenerator(cfg, t.TempDir())
  df := docs.DocFile{Name: "intro", Repository: "repo1", RelativePath: "docs/intro.md", Path: "", Section: "", Content: []byte("# Intro\n\nSee [Guide](guide.md).\n")}
  legacy, err := runLegacyPipeline(g, df)
  if err != nil { t.Fatalf("legacy pipeline error: %v", err) }
  reg, err := runRegistryPipeline(g, df)
  if err != nil { t.Fatalf("registry pipeline error: %v", err) }
  if legacy != reg {
    t.Fatalf("hash mismatch legacy=%s registry=%s", legacy, reg)
    // dump for debug
    p := &Page{File: df, Raw: df.Content, Content: string(df.Content), OriginalFrontMatter: nil, Patches: nil}
    if err := (&FrontMatterParser{}).Transform(p); err != nil { t.Fatalf("front matter parse error: %v", err) }
    dumpPage(t, p)
    for _, rt := range tr.List() {
      if err := rt.Transform(p); err != nil { t.Fatalf("transform error: %v", err) }
    }
    dumpPage(t, p)
  }
}
