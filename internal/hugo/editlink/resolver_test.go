package editlink

import (
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/docs"
)

func TestDetectorChain(t *testing.T) {
	t.Run("Chain executes in order", func(t *testing.T) {
		var executed []string

		detector1 := &mockDetector{
			name: "first",
			onDetect: func(_ DetectionContext) DetectionResult {
				executed = append(executed, "first")
				return DetectionResult{Found: false}
			},
		}

		detector2 := &mockDetector{
			name: "second",
			onDetect: func(_ DetectionContext) DetectionResult {
				executed = append(executed, "second")
				return DetectionResult{
					ForgeType: config.ForgeGitHub,
					BaseURL:   "https://github.com",
					FullName:  "owner/repo",
					Found:     true,
				}
			},
		}

		detector3 := &mockDetector{
			name: "third",
			onDetect: func(_ DetectionContext) DetectionResult {
				executed = append(executed, "third")
				return DetectionResult{Found: false}
			},
		}

		chain := NewDetectorChain().
			Add(detector1).
			Add(detector2).
			Add(detector3)

		ctx := DetectionContext{}
		result := chain.Detect(ctx)

		if !result.Found {
			t.Error("expected chain to find a result")
		}

		if result.ForgeType != config.ForgeGitHub {
			t.Errorf("expected forge type %s, got %s", config.ForgeGitHub, result.ForgeType)
		}

		// Should only execute first two detectors (stops at first success)
		if len(executed) != 2 || executed[0] != "first" || executed[1] != "second" {
			t.Errorf("expected execution order [first, second], got %v", executed)
		}
	})

	t.Run("Chain returns not found when no detector succeeds", func(t *testing.T) {
		detector1 := &mockDetector{
			name: "failing1",
			onDetect: func(_ DetectionContext) DetectionResult {
				return DetectionResult{Found: false}
			},
		}

		detector2 := &mockDetector{
			name: "failing2",
			onDetect: func(_ DetectionContext) DetectionResult {
				return DetectionResult{Found: false}
			},
		}

		chain := NewDetectorChain().
			Add(detector1).
			Add(detector2)

		ctx := DetectionContext{}
		result := chain.Detect(ctx)

		if result.Found {
			t.Error("expected chain to not find a result")
		}
	})
}

func TestConfiguredDetector(t *testing.T) {
	detector := NewConfiguredDetector()

	t.Run("Detects from repository tags", func(t *testing.T) {
		repo := &config.Repository{
			Tags: map[string]string{
				"forge_type": "github",
				"full_name":  "owner/repo",
			},
		}

		cfg := &config.Config{
			Forges: []*config.ForgeConfig{
				{
					Type:    config.ForgeGitHub,
					BaseURL: "https://github.com",
				},
			},
		}

		ctx := DetectionContext{
			Repository: repo,
			Config:     cfg,
		}

		result := detector.Detect(ctx)

		if !result.Found {
			t.Error("expected detector to find a result")
		}

		if result.ForgeType != config.ForgeGitHub {
			t.Errorf("expected forge type %s, got %s", config.ForgeGitHub, result.ForgeType)
		}

		if result.FullName != "owner/repo" {
			t.Errorf("expected full name 'owner/repo', got %s", result.FullName)
		}

		if result.BaseURL != "https://github.com" {
			t.Errorf("expected base URL 'https://github.com', got %s", result.BaseURL)
		}
	})

	t.Run("Returns not found when tags missing", func(t *testing.T) {
		repo := &config.Repository{
			Tags: map[string]string{
				"other": "value",
			},
		}

		ctx := DetectionContext{
			Repository: repo,
		}

		result := detector.Detect(ctx)

		if result.Found {
			t.Error("expected detector to not find a result")
		}
	})

	t.Run("Returns not found when repository nil", func(t *testing.T) {
		ctx := DetectionContext{
			Repository: nil,
		}

		result := detector.Detect(ctx)

		if result.Found {
			t.Error("expected detector to not find a result")
		}
	})
}

func TestHeuristicDetector(t *testing.T) {
	detector := NewHeuristicDetector()

	tests := []struct {
		name          string
		cloneURL      string
		expectedForge config.ForgeType
		expectedBase  string
		expectedFound bool
	}{
		{
			name:          "GitHub.com",
			cloneURL:      "https://github.com/owner/repo",
			expectedForge: config.ForgeGitHub,
			expectedBase:  "https://github.com",
			expectedFound: true,
		},
		{
			name:          "GitLab.com",
			cloneURL:      "https://gitlab.com/owner/repo",
			expectedForge: config.ForgeGitLab,
			expectedBase:  "https://gitlab.com",
			expectedFound: true,
		},
		{
			name:          "Forgejo instance",
			cloneURL:      "https://forgejo.example.com/owner/repo",
			expectedForge: config.ForgeForgejo,
			expectedBase:  "https://forgejo.example.com",
			expectedFound: true,
		},
		{
			name:          "SSH URL",
			cloneURL:      "git@github.com:owner/repo.git",
			expectedForge: config.ForgeGitHub,
			expectedBase:  "https://github.com",
			expectedFound: true,
		},
		{
			name:          "Empty URL",
			cloneURL:      "",
			expectedFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := DetectionContext{
				CloneURL: tt.cloneURL,
			}

			result := detector.Detect(ctx)

			if result.Found != tt.expectedFound {
				t.Errorf("expected found=%v, got %v", tt.expectedFound, result.Found)
			}

			if !tt.expectedFound {
				return
			}

			if result.ForgeType != tt.expectedForge {
				t.Errorf("expected forge type %s, got %s", tt.expectedForge, result.ForgeType)
			}

			if result.BaseURL != tt.expectedBase {
				t.Errorf("expected base URL %s, got %s", tt.expectedBase, result.BaseURL)
			}

			if result.FullName == "" {
				t.Error("expected full name to be extracted")
			}
		})
	}
}

func TestResolver(t *testing.T) {
	t.Run("Full integration test", func(t *testing.T) {
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
			t.Error("expected resolver to generate an edit URL")
		}

		expectedURL := "https://github.com/owner/repo/edit/main/docs/docs/guide.md"
		if result != expectedURL {
			t.Errorf("expected URL %s, got %s", expectedURL, result)
		}
	})

	t.Run("Returns empty for non-Hextra theme", func(t *testing.T) {
		resolver := NewResolver()

		cfg := &config.Config{
			Hugo: config.HugoConfig{},
		}

		file := docs.DocFile{Repository: "test-repo"}

		result := resolver.Resolve(file, cfg)

		if result != "" {
			t.Errorf("expected empty result for non-Hextra theme, got %s", result)
		}
	})

	t.Run("Returns empty when site-level suppressed", func(t *testing.T) {
		resolver := NewResolver()

		cfg := &config.Config{
			Hugo: config.HugoConfig{
				Params: map[string]any{
					"editURL": map[string]any{
						"base": "custom-base", // Non-empty base suppresses per-page links
					},
				},
			},
		}

		file := docs.DocFile{Repository: "test-repo"}

		result := resolver.Resolve(file, cfg)

		if result != "" {
			t.Errorf("expected empty result when site-level suppressed, got %s", result)
		}
	})
}

// Mock detector for testing.
type mockDetector struct {
	name     string
	onDetect func(DetectionContext) DetectionResult
}

func (m *mockDetector) Name() string {
	return m.name
}

func (m *mockDetector) Detect(ctx DetectionContext) DetectionResult {
	return m.onDetect(ctx)
}
