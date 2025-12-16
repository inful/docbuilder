package hugo

import (
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/docs"
)

// helper to build minimal repository config
func repo(url, name, branch string, paths ...string) config.Repository {
	if len(paths) == 0 {
		paths = []string{"docs"}
	}
	return config.Repository{URL: url, Name: name, Branch: branch, Paths: paths}
}

func TestEditLinkResolver_BasicForgeTypes(t *testing.T) {
	tests := []struct {
		name       string
		hugoParams map[string]any
		cfg        *config.Config
		file       docs.DocFile
		wantPrefix string
		wantEmpty  bool
	}{
		{
			name:       "github basic",
			
			cfg:        &config.Config{Hugo: config.HugoConfig{}, Forges: []*config.ForgeConfig{{Name: "gh", Type: config.ForgeGitHub, BaseURL: "https://github.com", APIURL: "https://api.github.com", Organizations: []string{"org"}, Auth: &config.AuthConfig{Type: config.AuthTypeToken}}}, Repositories: []config.Repository{repo("https://github.com/org/repo1.git", "repo1", "main", "docs")}},
			file:       docs.DocFile{Repository: "repo1", RelativePath: "guide/intro.md", DocsBase: "docs", Name: "intro", Extension: ".md"},
			wantPrefix: "https://github.com/org/repo1/edit/main/docs/guide/intro.md",
		},
		{
			name:       "gitlab basic via forge config",
			
			cfg:        &config.Config{Hugo: config.HugoConfig{}, Forges: []*config.ForgeConfig{{Name: "gl", Type: config.ForgeGitLab, BaseURL: "https://gitlab.example.com", APIURL: "https://gitlab.example.com/api/v4", Groups: []string{"group"}, Auth: &config.AuthConfig{Type: config.AuthTypeToken}}}, Repositories: []config.Repository{repo("https://gitlab.example.com/group/repoA.git", "repoA", "main", "docs")}},
			file:       docs.DocFile{Repository: "repoA", RelativePath: "section/page.md", DocsBase: "docs", Name: "page", Extension: ".md"},
			wantPrefix: "https://gitlab.example.com/group/repoA/-/edit/main/docs/section/page.md",
		},
		{
			name:       "forgejo basic",
			
			cfg:        &config.Config{Hugo: config.HugoConfig{}, Forges: []*config.ForgeConfig{{Name: "fj", Type: config.ForgeForgejo, BaseURL: "https://code.example.org", APIURL: "https://code.example.org/api/v1", Groups: []string{"team"}, Auth: &config.AuthConfig{Type: config.AuthTypeToken}}}, Repositories: []config.Repository{repo("https://code.example.org/team/project.git", "project", "main", "docs")}},
			file:       docs.DocFile{Repository: "project", RelativePath: "x/y/z.md", DocsBase: "docs", Name: "z", Extension: ".md"},
			wantPrefix: "https://code.example.org/team/project/_edit/main/docs/x/y/z.md",
		},
		{
			name:       "docsy theme also generates edit links",
			
			cfg:        &config.Config{Hugo: config.HugoConfig{}, Forges: []*config.ForgeConfig{{Name: "gh", Type: config.ForgeGitHub, BaseURL: "https://github.com", APIURL: "https://api.github.com", Organizations: []string{"org"}, Auth: &config.AuthConfig{Type: config.AuthTypeToken}}}, Repositories: []config.Repository{repo("https://github.com/org/repo1.git", "repo1", "main", "docs")}},
			file:       docs.DocFile{Repository: "repo1", RelativePath: "a.md", DocsBase: "docs", Name: "a", Extension: ".md"},
			wantPrefix: "https://github.com/org/repo1/edit/main/docs/a.md",
		},
		{
			name:       "site level suppression via params.editURL.base",
			
			hugoParams: map[string]any{"editURL": map[string]any{"base": "https://example.com/custom/base"}},
			cfg:        &config.Config{Hugo: config.HugoConfig{}, Forges: []*config.ForgeConfig{{Name: "gh", Type: config.ForgeGitHub, BaseURL: "https://github.com", APIURL: "https://api.github.com", Organizations: []string{"org"}, Auth: &config.AuthConfig{Type: config.AuthTypeToken}}}, Repositories: []config.Repository{repo("https://github.com/org/repo1.git", "repo1", "main", "docs")}},
			file:       docs.DocFile{Repository: "repo1", RelativePath: "b/c.md", DocsBase: "docs", Name: "c", Extension: ".md"},
			wantEmpty:  true,
		},
		{
			name:  "ssh clone URL normalization",
			
			cfg:   &config.Config{Hugo: config.HugoConfig{}, Forges: []*config.ForgeConfig{{Name: "gh", Type: config.ForgeGitHub, BaseURL: "https://github.com", APIURL: "https://api.github.com", Organizations: []string{"org"}, Auth: &config.AuthConfig{Type: config.AuthTypeToken}}}, Repositories: []config.Repository{repo("git@github.com:org/repo2.git", "repo2", "", "docs")}},
			file:  docs.DocFile{Repository: "repo2", RelativePath: "intro.md", DocsBase: "docs", Name: "intro", Extension: ".md"},
			// expect main branch fallback and proper path (no double docs/docs)
			wantPrefix: "https://github.com/org/repo2/edit/main/docs/intro.md",
		},
		{
			name:       "DocsBase dot (.) excluded from path",
			
			cfg:        &config.Config{Hugo: config.HugoConfig{}, Forges: []*config.ForgeConfig{{Name: "gh", Type: config.ForgeGitHub, BaseURL: "https://github.com", APIURL: "https://api.github.com", Organizations: []string{"org"}, Auth: &config.AuthConfig{Type: config.AuthTypeToken}}}, Repositories: []config.Repository{repo("https://github.com/org/repo3.git", "repo3", "main", ".")}},
			file:       docs.DocFile{Repository: "repo3", RelativePath: "plain.md", DocsBase: ".", Name: "plain", Extension: ".md"},
			wantPrefix: "https://github.com/org/repo3/edit/main/plain.md",
		},
		{
			name:       "multi-level docs base path",
			
			cfg:        &config.Config{Hugo: config.HugoConfig{}, Forges: []*config.ForgeConfig{{Name: "gh", Type: config.ForgeGitHub, BaseURL: "https://github.com", APIURL: "https://api.github.com", Organizations: []string{"org"}, Auth: &config.AuthConfig{Type: config.AuthTypeToken}}}, Repositories: []config.Repository{repo("https://github.com/org/repo4.git", "repo4", "main", "documentation/guides")}},
			file:       docs.DocFile{Repository: "repo4", RelativePath: "getting-started/intro.md", DocsBase: "documentation/guides", Name: "intro", Extension: ".md"},
			wantPrefix: "https://github.com/org/repo4/edit/main/documentation/guides/getting-started/intro.md",
		},
		{
			name:       "docs base trailing slash trimmed",
			
			cfg:        &config.Config{Hugo: config.HugoConfig{}, Forges: []*config.ForgeConfig{{Name: "gh", Type: config.ForgeGitHub, BaseURL: "https://github.com", APIURL: "https://api.github.com", Organizations: []string{"org"}, Auth: &config.AuthConfig{Type: config.AuthTypeToken}}}, Repositories: []config.Repository{repo("https://github.com/org/repo5.git", "repo5", "main", "docs/")}},
			file:       docs.DocFile{Repository: "repo5", RelativePath: "sub/page.md", DocsBase: "docs/", Name: "page", Extension: ".md"},
			wantPrefix: "https://github.com/org/repo5/edit/main/docs/sub/page.md",
		},
		{
			name:       "docs base with parent segments cleaned",
			
			cfg:        &config.Config{Hugo: config.HugoConfig{}, Forges: []*config.ForgeConfig{{Name: "gh", Type: config.ForgeGitHub, BaseURL: "https://github.com", APIURL: "https://api.github.com", Organizations: []string{"org"}, Auth: &config.AuthConfig{Type: config.AuthTypeToken}}}, Repositories: []config.Repository{repo("https://github.com/org/repo6.git", "repo6", "main", "docs/../docs/guides")}},
			file:       docs.DocFile{Repository: "repo6", RelativePath: "topic/overview.md", DocsBase: "docs/../docs/guides", Name: "overview", Extension: ".md"},
			wantPrefix: "https://github.com/org/repo6/edit/main/docs/guides/topic/overview.md",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Attach params if provided
			if tt.hugoParams != nil {
				tt.cfg.Hugo.Params = tt.hugoParams
			}
			// Force theme if test overrides (safety for cfg construction)
			r := NewEditLinkResolver(tt.cfg)
			got := r.Resolve(tt.file)
			if tt.wantEmpty {
				if got != "" {
					t.Fatalf("expected empty edit link, got %q", got)
				}
				return
			}
			if got == "" {
				t.Fatalf("expected non-empty edit link")
			}
			if got != tt.wantPrefix {
				t.Fatalf("edit link mismatch\n got: %s\nwant: %s", got, tt.wantPrefix)
			}
		})
	}
}
