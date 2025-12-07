package incremental

import (
	"encoding/json"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/hugo/theme"
	"git.home.luguber.info/inful/docbuilder/internal/pipeline"
)

func TestComputeBuildSignature(t *testing.T) {
	cfg := &config.Config{
		Hugo: config.HugoConfig{
			Title: "Test Site",
			Theme: "hextra",
		},
	}

	plan := &pipeline.BuildPlan{
		Config: cfg,
		ThemeFeatures: theme.Features{
			Name: "hextra",
		},
		TransformNames: []string{"frontmatter", "links"},
	}

	repos := []RepoHash{
		{Name: "repo1", Commit: "abc123", Hash: "hash1"},
		{Name: "repo2", Commit: "def456", Hash: "hash2"},
	}

	sig, err := ComputeBuildSignature(plan, repos)
	if err != nil {
		t.Fatalf("ComputeBuildSignature failed: %v", err)
	}

	if sig == nil {
		t.Fatal("signature should not be nil")
	}

	if sig.BuildHash == "" {
		t.Error("BuildHash should not be empty")
	}

	if sig.Theme != "hextra" {
		t.Errorf("expected theme hextra, got %s", sig.Theme)
	}

	if len(sig.RepoHashes) != 2 {
		t.Errorf("expected 2 repo hashes, got %d", len(sig.RepoHashes))
	}

	if len(sig.Transforms) != 2 {
		t.Errorf("expected 2 transforms, got %d", len(sig.Transforms))
	}
}

func TestBuildSignatureConsistency(t *testing.T) {
	cfg := &config.Config{
		Hugo: config.HugoConfig{
			Title:   "Test Site",
			Theme:   "hextra",
			BaseURL: "https://example.com",
		},
	}

	plan := &pipeline.BuildPlan{
		Config: cfg,
		ThemeFeatures: theme.Features{
			Name: "hextra",
		},
		TransformNames: []string{"frontmatter", "links"},
	}

	repos := []RepoHash{
		{Name: "repo1", Commit: "abc123", Hash: "hash1"},
		{Name: "repo2", Commit: "def456", Hash: "hash2"},
	}

	sig1, err := ComputeBuildSignature(plan, repos)
	if err != nil {
		t.Fatalf("first ComputeBuildSignature failed: %v", err)
	}

	sig2, err := ComputeBuildSignature(plan, repos)
	if err != nil {
		t.Fatalf("second ComputeBuildSignature failed: %v", err)
	}

	if sig1.BuildHash != sig2.BuildHash {
		t.Errorf("signatures should be identical, got %s and %s", sig1.BuildHash, sig2.BuildHash)
	}

	if !sig1.Equals(sig2) {
		t.Error("signatures should be equal")
	}
}

func TestBuildSignatureChangeDetection(t *testing.T) {
	cfg := &config.Config{
		Hugo: config.HugoConfig{
			Title: "Test Site",
			Theme: "hextra",
		},
	}

	basePlan := &pipeline.BuildPlan{
		Config: cfg,
		ThemeFeatures: theme.Features{
			Name: "hextra",
		},
		TransformNames: []string{"frontmatter"},
	}

	baseRepos := []RepoHash{
		{Name: "repo1", Commit: "abc123", Hash: "hash1"},
	}

	baseSig, err := ComputeBuildSignature(basePlan, baseRepos)
	if err != nil {
		t.Fatalf("base signature failed: %v", err)
	}

	tests := []struct {
		name     string
		plan     *pipeline.BuildPlan
		repos    []RepoHash
		wantDiff bool
	}{
		{
			name:     "same inputs",
			plan:     basePlan,
			repos:    baseRepos,
			wantDiff: false,
		},
		{
			name: "different repo hash",
			plan: basePlan,
			repos: []RepoHash{
				{Name: "repo1", Commit: "abc123", Hash: "hash2"},
			},
			wantDiff: true,
		},
		{
			name: "different repo commit",
			plan: basePlan,
			repos: []RepoHash{
				{Name: "repo1", Commit: "xyz789", Hash: "hash1"},
			},
			wantDiff: true,
		},
		{
			name: "different transforms",
			plan: &pipeline.BuildPlan{
				Config: cfg,
				ThemeFeatures: theme.Features{
					Name: "hextra",
				},
				TransformNames: []string{"frontmatter", "links"},
			},
			repos:    baseRepos,
			wantDiff: true,
		},
		{
			name: "different theme",
			plan: &pipeline.BuildPlan{
				Config: cfg,
				ThemeFeatures: theme.Features{
					Name: "docsy",
				},
				TransformNames: []string{"frontmatter"},
			},
			repos:    baseRepos,
			wantDiff: true,
		},
		{
			name: "different config",
			plan: &pipeline.BuildPlan{
				Config: &config.Config{
					Hugo: config.HugoConfig{
						Title: "Different Title",
						Theme: "hextra",
					},
				},
				ThemeFeatures: theme.Features{
					Name: "hextra",
				},
				TransformNames: []string{"frontmatter"},
			},
			repos:    baseRepos,
			wantDiff: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sig, err := ComputeBuildSignature(tt.plan, tt.repos)
			if err != nil {
				t.Fatalf("ComputeBuildSignature failed: %v", err)
			}

			isDiff := sig.BuildHash != baseSig.BuildHash
			if isDiff != tt.wantDiff {
				t.Errorf("expected diff=%v, got diff=%v (hashes: %s vs %s)",
					tt.wantDiff, isDiff, baseSig.BuildHash, sig.BuildHash)
			}
		})
	}
}

func TestBuildSignatureRepoOrdering(t *testing.T) {
	cfg := &config.Config{
		Hugo: config.HugoConfig{
			Title: "Test Site",
			Theme: "hextra",
		},
	}

	plan := &pipeline.BuildPlan{
		Config: cfg,
		ThemeFeatures: theme.Features{
			Name: "hextra",
		},
		TransformNames: []string{"frontmatter"},
	}

	repos1 := []RepoHash{
		{Name: "alpha", Commit: "abc123", Hash: "hash1"},
		{Name: "beta", Commit: "def456", Hash: "hash2"},
		{Name: "gamma", Commit: "ghi789", Hash: "hash3"},
	}

	repos2 := []RepoHash{
		{Name: "gamma", Commit: "ghi789", Hash: "hash3"},
		{Name: "alpha", Commit: "abc123", Hash: "hash1"},
		{Name: "beta", Commit: "def456", Hash: "hash2"},
	}

	sig1, err := ComputeBuildSignature(plan, repos1)
	if err != nil {
		t.Fatalf("first signature failed: %v", err)
	}

	sig2, err := ComputeBuildSignature(plan, repos2)
	if err != nil {
		t.Fatalf("second signature failed: %v", err)
	}

	if sig1.BuildHash != sig2.BuildHash {
		t.Errorf("signatures should be identical regardless of repo order, got %s and %s",
			sig1.BuildHash, sig2.BuildHash)
	}

	for i := 0; i < len(sig1.RepoHashes)-1; i++ {
		if sig1.RepoHashes[i].Name > sig1.RepoHashes[i+1].Name {
			t.Errorf("repos not sorted in sig1: %s > %s",
				sig1.RepoHashes[i].Name, sig1.RepoHashes[i+1].Name)
		}
	}
}

func TestBuildSignatureTransformOrdering(t *testing.T) {
	cfg := &config.Config{
		Hugo: config.HugoConfig{
			Title: "Test Site",
			Theme: "hextra",
		},
	}

	repos := []RepoHash{
		{Name: "repo1", Commit: "abc123", Hash: "hash1"},
	}

	plan1 := &pipeline.BuildPlan{
		Config: cfg,
		ThemeFeatures: theme.Features{
			Name: "hextra",
		},
		TransformNames: []string{"frontmatter", "links", "metadata"},
	}

	plan2 := &pipeline.BuildPlan{
		Config: cfg,
		ThemeFeatures: theme.Features{
			Name: "hextra",
		},
		TransformNames: []string{"metadata", "frontmatter", "links"},
	}

	sig1, err := ComputeBuildSignature(plan1, repos)
	if err != nil {
		t.Fatalf("first signature failed: %v", err)
	}

	sig2, err := ComputeBuildSignature(plan2, repos)
	if err != nil {
		t.Fatalf("second signature failed: %v", err)
	}

	if sig1.BuildHash != sig2.BuildHash {
		t.Errorf("signatures should be identical regardless of transform order, got %s and %s",
			sig1.BuildHash, sig2.BuildHash)
	}
}

func TestBuildSignatureJSON(t *testing.T) {
	cfg := &config.Config{
		Hugo: config.HugoConfig{
			Title: "Test Site",
			Theme: "hextra",
		},
	}

	plan := &pipeline.BuildPlan{
		Config: cfg,
		ThemeFeatures: theme.Features{
			Name: "hextra",
		},
		TransformNames: []string{"frontmatter"},
	}

	repos := []RepoHash{
		{Name: "repo1", Commit: "abc123", Hash: "hash1"},
	}

	sig, err := ComputeBuildSignature(plan, repos)
	if err != nil {
		t.Fatalf("ComputeBuildSignature failed: %v", err)
	}

	data, err := sig.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON failed: %v", err)
	}

	if len(data) == 0 {
		t.Error("JSON data should not be empty")
	}

	sig2, err := FromJSON(data)
	if err != nil {
		t.Fatalf("FromJSON failed: %v", err)
	}

	if sig.BuildHash != sig2.BuildHash {
		t.Errorf("deserialized signature hash mismatch: %s vs %s", sig.BuildHash, sig2.BuildHash)
	}

	if sig.Theme != sig2.Theme {
		t.Errorf("deserialized theme mismatch: %s vs %s", sig.Theme, sig2.Theme)
	}

	var jsonObj map[string]interface{}
	if err := json.Unmarshal(data, &jsonObj); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	requiredFields := []string{"repo_hashes", "theme", "transforms", "config_hash", "build_hash"}
	for _, field := range requiredFields {
		if _, ok := jsonObj[field]; !ok {
			t.Errorf("JSON missing required field: %s", field)
		}
	}
}

func TestBuildSignatureNilPlan(t *testing.T) {
	repos := []RepoHash{
		{Name: "repo1", Commit: "abc123", Hash: "hash1"},
	}

	_, err := ComputeBuildSignature(nil, repos)
	if err == nil {
		t.Error("expected error for nil plan")
	}
}

func TestBuildSignatureEmptyRepos(t *testing.T) {
	cfg := &config.Config{
		Hugo: config.HugoConfig{
			Title: "Test Site",
			Theme: "hextra",
		},
	}

	plan := &pipeline.BuildPlan{
		Config: cfg,
		ThemeFeatures: theme.Features{
			Name: "hextra",
		},
		TransformNames: []string{"frontmatter"},
	}

	sig, err := ComputeBuildSignature(plan, []RepoHash{})
	if err != nil {
		t.Fatalf("ComputeBuildSignature with empty repos failed: %v", err)
	}

	if len(sig.RepoHashes) != 0 {
		t.Errorf("expected 0 repos, got %d", len(sig.RepoHashes))
	}

	if sig.BuildHash == "" {
		t.Error("BuildHash should not be empty even with no repos")
	}
}

func TestBuildSignatureEquals(t *testing.T) {
	cfg := &config.Config{
		Hugo: config.HugoConfig{
			Title: "Test Site",
			Theme: "hextra",
		},
	}

	plan := &pipeline.BuildPlan{
		Config: cfg,
		ThemeFeatures: theme.Features{
			Name: "hextra",
		},
		TransformNames: []string{"frontmatter"},
	}

	repos := []RepoHash{
		{Name: "repo1", Commit: "abc123", Hash: "hash1"},
	}

	sig1, _ := ComputeBuildSignature(plan, repos)
	sig2, _ := ComputeBuildSignature(plan, repos)

	if !sig1.Equals(sig2) {
		t.Error("identical signatures should be equal")
	}

	var nilSig *BuildSignature
	if sig1.Equals(nilSig) {
		t.Error("signature should not equal nil")
	}

	if nilSig.Equals(sig1) {
		t.Error("nil should not equal signature")
	}

	if !nilSig.Equals(nilSig) {
		t.Error("nil should equal nil")
	}
}
