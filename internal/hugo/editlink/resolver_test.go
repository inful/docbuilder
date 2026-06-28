package editlink

import (
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/docs"
)

func TestResolver(t *testing.T) {
	t.Run("GitHub URL resolution", func(t *testing.T) {
		resolver := NewResolver()

		cfg := &config.Config{
			Hugo: config.HugoConfig{},
			Repositories: []config.Repository{
				{
					Name:   "test-repo",
					URL:    "https://github.com/owner/repo.git",
					Branch: "main",
				},
			},
		}

		file := docs.DocFile{
			Repository:   "test-repo",
			RelativePath: "docs/guide.md",
			DocsBase:     "docs",
		}

		result := resolver.Resolve(file, cfg)
		if result == "" {
			t.Fatal("expected resolver to generate an edit URL")
		}

		want := "https://github.com/owner/repo/edit/main/docs/docs/guide.md"
		if result != want {
			t.Errorf("expected URL %q, got %q", want, result)
		}
	})

	t.Run("Configured repository tags take precedence", func(t *testing.T) {
		resolver := NewResolver()

		cfg := &config.Config{
			Hugo: config.HugoConfig{},
			Repositories: []config.Repository{
				{
					Name:   "tagged-repo",
					URL:    "https://gitlab.example.com/owner/repo.git",
					Branch: "main",
					Tags: map[string]string{
						"forge_type": "github",
						"full_name":  "owner/repo",
					},
				},
			},
			Forges: []*config.ForgeConfig{
				{Type: config.ForgeGitHub, BaseURL: "https://github.com"},
			},
		}

		file := docs.DocFile{
			Repository:   "tagged-repo",
			RelativePath: "page.md",
			DocsBase:     ".",
		}

		result := resolver.Resolve(file, cfg)
		want := "https://github.com/owner/repo/edit/main/page.md"
		if result != want {
			t.Errorf("expected URL %q, got %q", want, result)
		}
	})

	t.Run("Heuristic detects Forgejo by hostname", func(t *testing.T) {
		resolver := NewResolver()

		cfg := &config.Config{
			Hugo: config.HugoConfig{},
			Repositories: []config.Repository{
				{
					Name:   "self-hosted",
					URL:    "https://forgejo.example.com/owner/repo.git",
					Branch: "main",
				},
			},
		}

		file := docs.DocFile{
			Repository:   "self-hosted",
			RelativePath: "docs/index.md",
			DocsBase:     "docs",
		}

		result := resolver.Resolve(file, cfg)
		want := "https://forgejo.example.com/owner/repo/_edit/main/docs/docs/index.md"
		if result != want {
			t.Errorf("expected URL %q, got %q", want, result)
		}
	})

	t.Run("Empty for missing repository in config", func(t *testing.T) {
		resolver := NewResolver()
		cfg := &config.Config{Hugo: config.HugoConfig{}}
		file := docs.DocFile{Repository: "unknown-repo"}

		if got := resolver.Resolve(file, cfg); got != "" {
			t.Errorf("expected empty result for unknown repo, got %q", got)
		}
	})

	t.Run("Empty when site-level editURL.base is set", func(t *testing.T) {
		resolver := NewResolver()
		cfg := &config.Config{
			Hugo: config.HugoConfig{
				Params: map[string]any{
					"editURL": map[string]any{
						"base": "custom-base", // Non-empty base suppresses per-page links.
					},
				},
			},
		}
		file := docs.DocFile{Repository: "test-repo"}

		if got := resolver.Resolve(file, cfg); got != "" {
			t.Errorf("expected empty result when site-level suppressed, got %q", got)
		}
	})
}

func TestNormalizeSSHURL(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"already HTTPS", "https://github.com/owner/repo", "https://github.com/owner/repo"},
		{"SSH form", "git@github.com:owner/repo.git", "https://github.com/owner/repo.git"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeSSHURL(tt.in); got != tt.want {
				t.Errorf("normalizeSSHURL(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestExtractFullNameFromURL(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"HTTPS", "https://github.com/owner/repo.git", "owner/repo"},
		{"SSH", "git@github.com:owner/repo.git", "owner/repo"},
		{"no .git", "https://github.com/owner/repo", "owner/repo"},
		{"invalid", "://bad url", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractFullNameFromURL(tt.in); got != tt.want {
				t.Errorf("extractFullNameFromURL(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
