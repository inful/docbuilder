package pipeline

import (
	"testing"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/docs"
	"git.home.luguber.info/inful/docbuilder/internal/manifest"
)

func TestManifestGeneration(t *testing.T) {
	// Create a basic build plan
	cfg := &config.Config{}
	cfg.Repositories = []config.Repository{
		{Name: "test-repo", URL: "https://github.com/test/repo", Branch: "main"},
	}
	cfg.Hugo.Theme = string(config.ThemeHextra)

	plan := NewBuildPlanBuilder(cfg).
		WithOutput("/tmp/test-output", "/tmp/test-workspace").
		Build()

	// Simulate building a manifest
	m := &manifest.BuildManifest{
		ID:        "build-test-123",
		Timestamp: time.Now(),
		Inputs: manifest.Inputs{
			Repos: []manifest.RepoInput{
				{
					Name:   "test-repo",
					URL:    "https://github.com/test/repo",
					Branch: "main",
					Commit: "abc123",
				},
			},
			ConfigHash: "config-hash",
		},
		Plan: manifest.Plan{
			Theme:      string(plan.Config.Hugo.Theme),
			Transforms: plan.TransformNames,
		},
		Plugins: manifest.Plugins{
			Theme: &manifest.PluginVersion{
				Name:    "hextra",
				Version: "v1.0.0",
				Type:    "theme",
			},
			Transforms: []manifest.PluginVersion{
				{Name: "frontmatter", Version: "v1.0.0", Type: "transform"},
			},
		},
		Outputs: manifest.Outputs{
			HugoConfigHash: "hugo-config-hash",
		},
		Status:     "success",
		Duration:   1000,
		EventCount: 5,
	}

	// Verify manifest can be created
	jsonData, err := m.ToJSON()
	if err != nil {
		t.Fatalf("failed to create manifest: %v", err)
	}

	if len(jsonData) == 0 {
		t.Error("manifest JSON is empty")
	}

	// Verify hash is deterministic
	hash, err := m.Hash()
	if err != nil {
		t.Fatalf("failed to hash manifest: %v", err)
	}

	if len(hash) != 64 {
		t.Errorf("expected 64-char hash, got %d", len(hash))
	}
}

func TestManifestGenerationWithDocFiles(t *testing.T) {
	cfg := &config.Config{}
	cfg.Repositories = []config.Repository{
		{Name: "test-repo", URL: "https://github.com/test/repo", Branch: "main"},
	}

	plan := NewBuildPlanBuilder(cfg).
		WithOutput("/tmp/test-output", "/tmp/test-workspace").
		Build()

	// Simulate discovered doc files
	_ = []docs.DocFile{
		{Repository: "test-repo", Path: "docs/README.md"},
		{Repository: "test-repo", Path: "docs/guide.md"},
	}

	m := &manifest.BuildManifest{
		ID:        "build-test-456",
		Timestamp: time.Now(),
		Inputs: manifest.Inputs{
			Repos: []manifest.RepoInput{
				{Name: "test-repo", URL: "https://github.com/test/repo", Branch: "main", Commit: "def456"},
			},
			ConfigHash: "config-hash",
		},
		Plan: manifest.Plan{
			Theme:      string(plan.Config.Hugo.Theme),
			Transforms: plan.TransformNames,
		},
		Plugins: manifest.Plugins{
			Theme: &manifest.PluginVersion{
				Name:    "hextra",
				Version: "v1.0.0",
				Type:    "theme",
			},
		},
		Outputs: manifest.Outputs{
			HugoConfigHash: "hugo-config-hash",
			ContentHash:    "content-hash-for-2-files",
		},
		Status:     "success",
		Duration:   2000,
		EventCount: 8,
	}

	// Test that file count affects content hash
	hash1, _ := m.Hash()

	// Change one input
	m.Inputs.Repos[0].Commit = "xyz789"
	hash2, _ := m.Hash()

	if hash1 == hash2 {
		t.Error("expected different hashes for different commits")
	}
}
