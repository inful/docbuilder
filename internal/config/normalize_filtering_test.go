package config

import "testing"

func TestNormalizeFiltering_DedupeTrimSort(t *testing.T) {
    c := &Config{Version: "2.0", Filtering: &FilteringConfig{
        RequiredPaths:   []string{" docs ", "docs", "guides"},
        IgnoreFiles:     []string{" .docignore ", ".docignore", "README.md"},
        IncludePatterns: []string{"  lib-*  ", "api-*", "lib-*", "core"},
        ExcludePatterns: []string{"temp", " temp", "scratch", "scratch"},
    }, Hugo: HugoConfig{Theme: "hextra"}}
    res, err := NormalizeConfig(c)
    if err != nil { t.Fatalf("normalize: %v", err) }
    if len(res.Warnings) == 0 { t.Fatalf("expected warnings for filtering normalization") }
    // RequiredPaths should be [docs,guides] sorted
    if len(c.Filtering.RequiredPaths) != 2 || c.Filtering.RequiredPaths[0] != "docs" || c.Filtering.RequiredPaths[1] != "guides" {
        t.Fatalf("unexpected required_paths: %#v", c.Filtering.RequiredPaths)
    }
    // IgnoreFiles trimmed/deduped and sorted => [".docignore","README.md"] (space-trim) keep order alphabetical
    if len(c.Filtering.IgnoreFiles) != 2 || c.Filtering.IgnoreFiles[0] != ".docignore" || c.Filtering.IgnoreFiles[1] != "README.md" {
        t.Fatalf("unexpected ignore_files: %#v", c.Filtering.IgnoreFiles)
    }
    // IncludePatterns => ["api-*","core","lib-*"]
    if got := c.Filtering.IncludePatterns; len(got) != 3 || got[0] != "api-*" || got[1] != "core" || got[2] != "lib-*" {
        t.Fatalf("unexpected include_patterns: %#v", got)
    }
    // ExcludePatterns => ["scratch","temp"]
    if got := c.Filtering.ExcludePatterns; len(got) != 2 || got[0] != "scratch" || got[1] != "temp" {
        t.Fatalf("unexpected exclude_patterns: %#v", got)
    }
}

func TestSnapshot_IncludesFiltering(t *testing.T) {
    c := &Config{Version: "2.0", Filtering: &FilteringConfig{IncludePatterns: []string{"b","a"}}, Hugo: HugoConfig{Theme: "hextra"}}
    if _, err := NormalizeConfig(c); err != nil { t.Fatalf("normalize: %v", err) }
    if err := applyDefaults(c); err != nil { t.Fatalf("defaults: %v", err) }
    snap1 := c.Snapshot()
    // reorder slice differently to confirm order-insensitive snapshot
    c.Filtering.IncludePatterns = []string{"a","b"}
    snap2 := c.Snapshot()
    if snap1 != snap2 { t.Fatalf("expected snapshot stable ignoring filtering include order") }
    // change pattern set
    c.Filtering.IncludePatterns = []string{"a","c"}
    snap3 := c.Snapshot()
    if snap3 == snap2 { t.Fatalf("expected snapshot change after filtering modification") }
}
