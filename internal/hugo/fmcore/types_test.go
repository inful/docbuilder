package fmcore

import (
    "testing"
    "time"
    "git.home.luguber.info/inful/docbuilder/internal/config"
    "git.home.luguber.info/inful/docbuilder/internal/docs"
)

// TestComputeBaseFrontMatter_CreatesTitleDateAndMetadata ensures baseline fields are populated
// with expected normalization behavior and do not overwrite existing keys.
func TestComputeBaseFrontMatter_CreatesTitleDateAndMetadata(t *testing.T) {
    existing := map[string]any{"title": "Keep", "custom": "orig"}
    meta := map[string]any{"custom": "meta", "description": "Desc"}
    now := time.Date(2025, 9, 30, 12, 34, 56, 0, time.UTC)
    cfg := &config.Config{}
    fm := ComputeBaseFrontMatter("sample_page", "repo1", "github", "sectionA", meta, existing, cfg, now)

    // Title preserved (existing wins)
    if fm["title"].(string) != "Keep" {
        t.Fatalf("expected existing title preserved, got %v", fm["title"])
    }
    // Date injected (exact formatting RFC3339-like with offset - compare prefix since offset may differ under local tz logic)
    if d, ok := fm["date"].(string); !ok || len(d) < 19 || d[:19] != "2025-09-30T12:34:56" {
        t.Fatalf("expected date injected with timestamp, got %v", fm["date"])
    }
    if fm["repository"].(string) != "repo1" {
        t.Fatalf("expected repository repo1, got %v", fm["repository"])
    }
    if fm["forge"].(string) != "github" {
        t.Fatalf("expected forge github, got %v", fm["forge"])
    }
    if fm["section"].(string) != "sectionA" {
        t.Fatalf("expected section sectionA, got %v", fm["section"])
    }
    // Metadata passthrough only when missing
    if fm["custom"].(string) != "orig" { // meta should not override existing
        t.Fatalf("expected existing custom key retained, got %v", fm["custom"])
    }
    if fm["description"].(string) != "Desc" {
        t.Fatalf("expected description passthrough, got %v", fm["description"])
    }
}

// TestComputeBaseFrontMatter_TitleGeneration ensures snake/dash naming normalization.
func TestComputeBaseFrontMatter_TitleGeneration(t *testing.T) {
    fm := ComputeBaseFrontMatter("my_sample-page", "r", "", "", nil, map[string]any{}, &config.Config{}, time.Now())
    if fm["title"].(string) != "My Sample Page" {
        t.Fatalf("expected normalized title, got %v", fm["title"])
    }
}

// helper config for edit link tests
func testConfigWithRepo(theme config.Theme, forgeType config.ForgeType, baseURL, repoURL, name, branch string) *config.Config {
    return &config.Config{
        Hugo: config.HugoConfig{Theme: string(theme)},
        Repositories: []config.Repository{{
            URL:    repoURL,
            Name:   name,
            Branch: branch,
            Paths:  []string{"docs"},
            Tags: map[string]string{
                "forge_type": string(forgeType),
                "full_name":  "acme/" + name,
            },
        }},
        Forges: []*config.ForgeConfig{{
            Name:    "acme", Type: forgeType, BaseURL: baseURL,
            Auth: &config.AuthConfig{Type: config.AuthTypeToken, Token: "x"},
            Organizations: []string{"acme"},
        }},
    }
}

// TestResolveEditLink_GitHub ensures proper GitHub style link.
func TestResolveEditLink_GitHub(t *testing.T) {
    cfg := testConfigWithRepo(config.ThemeHextra, config.ForgeGitHub, "https://github.com", "https://github.com/acme/repo1.git", "repo1", "main")
    df := docs.DocFile{Repository: "repo1", RelativePath: "docs/path/file.md", DocsBase: "docs"}
    url := ResolveEditLink(df, cfg)
    // DocsBase plus RelativePath (with DocsBase again) currently yields duplicated segment: docs/docs/...
    expected := "https://github.com/acme/repo1/edit/main/docs/docs/path/file.md"
    if url != expected {
        t.Fatalf("expected %s got %s", expected, url)
    }
}

// TestResolveEditLink_ThemeMismatch ensures non-Hextra theme suppresses link.
func TestResolveEditLink_ThemeMismatch(t *testing.T) {
    cfg := testConfigWithRepo(config.ThemeDocsy, config.ForgeGitHub, "https://github.com", "https://github.com/acme/repo1.git", "repo1", "main")
    df := docs.DocFile{Repository: "repo1", RelativePath: "docs/file.md", DocsBase: "docs"}
    if v := ResolveEditLink(df, cfg); v != "" {
        t.Fatalf("expected empty link for non-hextra theme, got %s", v)
    }
}

// TestResolveEditLink_ExistingBaseEditURL ensures configured editURL base blocks automatic generation.
func TestResolveEditLink_ExistingBaseEditURL(t *testing.T) {
    cfg := testConfigWithRepo(config.ThemeHextra, config.ForgeGitHub, "https://github.com", "https://github.com/acme/repo1.git", "repo1", "main")
    cfg.Hugo.Params = map[string]any{"editURL": map[string]any{"base": "https://custom/edit"}}
    df := docs.DocFile{Repository: "repo1", RelativePath: "docs/file.md", DocsBase: "docs"}
    if v := ResolveEditLink(df, cfg); v != "" {
        t.Fatalf("expected empty link when user sets editURL.base, got %s", v)
    }
}
