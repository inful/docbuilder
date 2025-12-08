package incremental

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/hugo/theme"
	"git.home.luguber.info/inful/docbuilder/internal/manifest"
	"git.home.luguber.info/inful/docbuilder/internal/pipeline"
	"git.home.luguber.info/inful/docbuilder/internal/storage"
)

func TestShouldSkipBuild(t *testing.T) {
	store := storage.NewMockStore()
	cache := NewBuildCache(store, t.TempDir())

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

	// Initially, no cached build exists
	skip, entry, err := cache.ShouldSkipBuild(sig)
	if err != nil {
		t.Fatalf("ShouldSkipBuild failed: %v", err)
	}
	if skip {
		t.Error("expected skip=false for new build")
	}
	if entry != nil {
		t.Error("expected nil entry for new build")
	}

	// Save a build
	m := &manifest.BuildManifest{
		ID:        "build-001",
		Timestamp: time.Now(),
		Status:    "completed",
		Inputs: manifest.Inputs{
			Repos: []manifest.RepoInput{
				{Name: "repo1", Commit: "abc123", Hash: "hash1"},
			},
		},
	}

	if err := cache.SaveBuild(sig, m, "/tmp/output"); err != nil {
		t.Fatalf("SaveBuild failed: %v", err)
	}

	// Now the build should be found
	skip, entry, err = cache.ShouldSkipBuild(sig)
	if err != nil {
		t.Fatalf("ShouldSkipBuild failed: %v", err)
	}
	if !skip {
		t.Error("expected skip=true for cached build")
	}
	if entry == nil {
		t.Fatal("expected non-nil entry for cached build")
	}
	if entry.BuildID != "build-001" {
		t.Errorf("expected build ID build-001, got %s", entry.BuildID)
	}
}

func TestShouldSkipBuildDifferentSignatures(t *testing.T) {
	store := storage.NewMockStore()
	cache := NewBuildCache(store, t.TempDir())

	cfg := &config.Config{
		Hugo: config.HugoConfig{
			Title: "Test Site",
			Theme: "hextra",
		},
	}

	// Build 1
	plan1 := &pipeline.BuildPlan{
		Config: cfg,
		ThemeFeatures: theme.Features{
			Name: "hextra",
		},
		TransformNames: []string{"frontmatter"},
	}

	repos1 := []RepoHash{
		{Name: "repo1", Commit: "abc123", Hash: "hash1"},
	}

	sig1, err := ComputeBuildSignature(plan1, repos1)
	if err != nil {
		t.Fatalf("ComputeBuildSignature failed: %v", err)
	}

	m1 := &manifest.BuildManifest{
		ID:        "build-001",
		Timestamp: time.Now(),
		Status:    "completed",
	}

	if err := cache.SaveBuild(sig1, m1, "/tmp/output1"); err != nil {
		t.Fatalf("SaveBuild failed: %v", err)
	}

	// Build 2 with different repo hash
	repos2 := []RepoHash{
		{Name: "repo1", Commit: "abc123", Hash: "hash2"},
	}

	sig2, err := ComputeBuildSignature(plan1, repos2)
	if err != nil {
		t.Fatalf("ComputeBuildSignature failed: %v", err)
	}

	// Should not skip build 2 (different signature)
	skip, _, err := cache.ShouldSkipBuild(sig2)
	if err != nil {
		t.Fatalf("ShouldSkipBuild failed: %v", err)
	}
	if skip {
		t.Error("expected skip=false for different signature")
	}
}

func TestShouldSkipBuildMultipleBuilds(t *testing.T) {
	store := storage.NewMockStore()
	cache := NewBuildCache(store, t.TempDir())

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

	// Save multiple builds with same signature (e.g., rebuilds)
	for i := 1; i <= 3; i++ {
		m := &manifest.BuildManifest{
			ID:        "build-" + string(rune('0'+i)),
			Timestamp: time.Now().Add(time.Duration(i) * time.Second),
			Status:    "completed",
		}
		if err := cache.SaveBuild(sig, m, "/tmp/output"); err != nil {
			t.Fatalf("SaveBuild %d failed: %v", i, err)
		}
		time.Sleep(10 * time.Millisecond) // Ensure different timestamps
	}

	// Should find the most recent build
	skip, entry, err := cache.ShouldSkipBuild(sig)
	if err != nil {
		t.Fatalf("ShouldSkipBuild failed: %v", err)
	}
	if !skip {
		t.Error("expected skip=true")
	}
	if entry == nil {
		t.Fatal("expected non-nil entry")
	}
	// Most recent should be the last one saved
	t.Logf("Found build: %s", entry.BuildID)
}

func TestCanReuseOutput(t *testing.T) {
	tmpDir := t.TempDir()
	store := storage.NewMockStore()
	cache := NewBuildCache(store, tmpDir)

	// Create a valid output directory
	outputDir := filepath.Join(tmpDir, "output")
	if err := os.MkdirAll(filepath.Join(outputDir, "content"), 0750); err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(outputDir, "hugo.yaml"), []byte("title: Test"), 0600); err != nil {
		t.Fatalf("failed to create hugo.yaml: %v", err)
	}

	entry := &BuildCacheEntry{
		BuildID:    "build-001",
		OutputPath: outputDir,
	}

	if !cache.CanReuseOutput(entry) {
		t.Error("expected output to be reusable")
	}

	// Test with missing output directory
	entry2 := &BuildCacheEntry{
		BuildID:    "build-002",
		OutputPath: filepath.Join(tmpDir, "nonexistent"),
	}

	if cache.CanReuseOutput(entry2) {
		t.Error("expected output to be not reusable (missing dir)")
	}

	// Test with incomplete output directory
	incompleteDir := filepath.Join(tmpDir, "incomplete")
	if err := os.MkdirAll(incompleteDir, 0750); err != nil {
		t.Fatalf("failed to create incomplete dir: %v", err)
	}

	entry3 := &BuildCacheEntry{
		BuildID:    "build-003",
		OutputPath: incompleteDir,
	}

	if cache.CanReuseOutput(entry3) {
		t.Error("expected output to be not reusable (missing hugo.yaml)")
	}
}

func TestSaveBuildInvalidInputs(t *testing.T) {
	store := storage.NewMockStore()
	cache := NewBuildCache(store, t.TempDir())

	m := &manifest.BuildManifest{
		ID:        "build-001",
		Timestamp: time.Now(),
		Status:    "completed",
	}

	// Test with nil signature
	err := cache.SaveBuild(nil, m, "/tmp/output")
	if err == nil {
		t.Error("expected error for nil signature")
	}

	// Test with empty signature
	sig := &BuildSignature{BuildHash: ""}
	err = cache.SaveBuild(sig, m, "/tmp/output")
	if err == nil {
		t.Error("expected error for empty signature hash")
	}

	// Test with nil manifest
	sig.BuildHash = "hash123"
	err = cache.SaveBuild(sig, nil, "/tmp/output")
	if err == nil {
		t.Error("expected error for nil manifest")
	}
}

func TestShouldSkipBuildInvalidSignature(t *testing.T) {
	store := storage.NewMockStore()
	cache := NewBuildCache(store, t.TempDir())

	// Test with nil signature
	_, _, err := cache.ShouldSkipBuild(nil)
	if err == nil {
		t.Error("expected error for nil signature")
	}

	// Test with empty signature
	sig := &BuildSignature{BuildHash: ""}
	_, _, err = cache.ShouldSkipBuild(sig)
	if err == nil {
		t.Error("expected error for empty signature hash")
	}
}

func TestInvalidateBuild(t *testing.T) {
	store := storage.NewMockStore()
	cache := NewBuildCache(store, t.TempDir())

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

	m := &manifest.BuildManifest{
		ID:        "build-001",
		Timestamp: time.Now(),
		Status:    "completed",
	}

	if err := cache.SaveBuild(sig, m, "/tmp/output"); err != nil {
		t.Fatalf("SaveBuild failed: %v", err)
	}

	// Verify build is cached
	skip, _, err := cache.ShouldSkipBuild(sig)
	if err != nil {
		t.Fatalf("ShouldSkipBuild failed: %v", err)
	}
	if !skip {
		t.Error("expected build to be cached")
	}

	// Invalidate the build
	if err := cache.InvalidateBuild("build-001"); err != nil {
		t.Fatalf("InvalidateBuild failed: %v", err)
	}

	// Verify build is no longer cached
	skip, _, err = cache.ShouldSkipBuild(sig)
	if err != nil {
		t.Fatalf("ShouldSkipBuild failed: %v", err)
	}
	if skip {
		t.Error("expected build to not be cached after invalidation")
	}
}

func TestInvalidateBuildNotFound(t *testing.T) {
	store := storage.NewMockStore()
	cache := NewBuildCache(store, t.TempDir())

	err := cache.InvalidateBuild("nonexistent-build")
	if err == nil {
		t.Error("expected error for nonexistent build")
	}
}

func TestCanReuseOutputNilEntry(t *testing.T) {
	store := storage.NewMockStore()
	cache := NewBuildCache(store, t.TempDir())

	if cache.CanReuseOutput(nil) {
		t.Error("expected false for nil entry")
	}

	entry := &BuildCacheEntry{
		BuildID:    "build-001",
		OutputPath: "",
	}

	if cache.CanReuseOutput(entry) {
		t.Error("expected false for empty output path")
	}
}
