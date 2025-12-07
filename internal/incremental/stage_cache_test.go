package incremental

import (
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/docs"
	"git.home.luguber.info/inful/docbuilder/internal/storage"
)

func TestCanSkipClone(t *testing.T) {
	store := storage.NewMockStore()
	cache := NewStageCache(store)

	// Initially, no cached clone exists
	skip, path, err := cache.CanSkipClone("repo1", "hash123")
	if err != nil {
		t.Fatalf("CanSkipClone failed: %v", err)
	}
	if skip {
		t.Error("expected skip=false for uncached repo")
	}
	if path != "" {
		t.Error("expected empty path for uncached repo")
	}

	// Save a clone
	treeData := []byte("tree data")
	if err := cache.SaveClone("repo1", "hash123", "/workspace/repo1", treeData); err != nil {
		t.Fatalf("SaveClone failed: %v", err)
	}

	// Now the clone should be found
	skip, path, err = cache.CanSkipClone("repo1", "hash123")
	if err != nil {
		t.Fatalf("CanSkipClone failed: %v", err)
	}
	if !skip {
		t.Error("expected skip=true for cached repo")
	}
	if path != "/workspace/repo1" {
		t.Errorf("expected path /workspace/repo1, got %s", path)
	}
}

func TestCanSkipCloneDifferentHash(t *testing.T) {
	store := storage.NewMockStore()
	cache := NewStageCache(store)

	// Save a clone with hash1
	treeData := []byte("tree data")
	if err := cache.SaveClone("repo1", "hash1", "/workspace/repo1", treeData); err != nil {
		t.Fatalf("SaveClone failed: %v", err)
	}

	// Query with different hash should not find it
	skip, _, err := cache.CanSkipClone("repo1", "hash2")
	if err != nil {
		t.Fatalf("CanSkipClone failed: %v", err)
	}
	if skip {
		t.Error("expected skip=false for different hash")
	}
}

func TestCanSkipCloneInvalidInputs(t *testing.T) {
	store := storage.NewMockStore()
	cache := NewStageCache(store)

	_, _, err := cache.CanSkipClone("", "hash123")
	if err == nil {
		t.Error("expected error for empty repoName")
	}

	_, _, err = cache.CanSkipClone("repo1", "")
	if err == nil {
		t.Error("expected error for empty repoHash")
	}
}

func TestCanSkipDiscovery(t *testing.T) {
	store := storage.NewMockStore()
	cache := NewStageCache(store)

	// Initially, no cached discovery exists
	skip, files, err := cache.CanSkipDiscovery("repo1", "hash123")
	if err != nil {
		t.Fatalf("CanSkipDiscovery failed: %v", err)
	}
	if skip {
		t.Error("expected skip=false for uncached discovery")
	}
	if len(files) != 0 {
		t.Error("expected empty files for uncached discovery")
	}

	// Save discovery
	docFiles := []docs.DocFile{
		{
			Repository:   "repo1",
			Path:         "/workspace/repo1/docs/file1.md",
			RelativePath: "file1.md",
			Content:      []byte("content 1"),
		},
		{
			Repository:   "repo1",
			Path:         "/workspace/repo1/docs/file2.md",
			RelativePath: "file2.md",
			Content:      []byte("content 2"),
		},
	}

	if err := cache.SaveDiscovery("repo1", "hash123", docFiles); err != nil {
		t.Fatalf("SaveDiscovery failed: %v", err)
	}

	// Now discovery should be found
	skip, files, err = cache.CanSkipDiscovery("repo1", "hash123")
	if err != nil {
		t.Fatalf("CanSkipDiscovery failed: %v", err)
	}
	if !skip {
		t.Error("expected skip=true for cached discovery")
	}
	if len(files) != 2 {
		t.Errorf("expected 2 files, got %d", len(files))
	}
	if files[0].Repository != "repo1" {
		t.Errorf("expected repo1, got %s", files[0].Repository)
	}
}

func TestCanSkipDiscoveryDifferentHash(t *testing.T) {
	store := storage.NewMockStore()
	cache := NewStageCache(store)

	docFiles := []docs.DocFile{
		{
			Repository:   "repo1",
			Path:         "/workspace/repo1/docs/file1.md",
			RelativePath: "file1.md",
			Content:      []byte("content 1"),
		},
	}

	if err := cache.SaveDiscovery("repo1", "hash1", docFiles); err != nil {
		t.Fatalf("SaveDiscovery failed: %v", err)
	}

	// Query with different hash should not find it
	skip, _, err := cache.CanSkipDiscovery("repo1", "hash2")
	if err != nil {
		t.Fatalf("CanSkipDiscovery failed: %v", err)
	}
	if skip {
		t.Error("expected skip=false for different hash")
	}
}

func TestCanSkipDiscoveryInvalidInputs(t *testing.T) {
	store := storage.NewMockStore()
	cache := NewStageCache(store)

	_, _, err := cache.CanSkipDiscovery("", "hash123")
	if err == nil {
		t.Error("expected error for empty repoName")
	}

	_, _, err = cache.CanSkipDiscovery("repo1", "")
	if err == nil {
		t.Error("expected error for empty repoHash")
	}
}

func TestCanSkipTransform(t *testing.T) {
	store := storage.NewMockStore()
	cache := NewStageCache(store)

	// Initially, no cached transform exists
	skip, err := cache.CanSkipTransform("repo1", "hash123", "frontmatter")
	if err != nil {
		t.Fatalf("CanSkipTransform failed: %v", err)
	}
	if skip {
		t.Error("expected skip=false for uncached transform")
	}

	// Save transform
	content := []byte("transformed content")
	if err := cache.SaveTransform("repo1", "hash123", "frontmatter", content); err != nil {
		t.Fatalf("SaveTransform failed: %v", err)
	}

	// Now transform should be found
	skip, err = cache.CanSkipTransform("repo1", "hash123", "frontmatter")
	if err != nil {
		t.Fatalf("CanSkipTransform failed: %v", err)
	}
	if !skip {
		t.Error("expected skip=true for cached transform")
	}
}

func TestCanSkipTransformDifferentParams(t *testing.T) {
	store := storage.NewMockStore()
	cache := NewStageCache(store)

	content := []byte("transformed content")
	if err := cache.SaveTransform("repo1", "hash1", "frontmatter", content); err != nil {
		t.Fatalf("SaveTransform failed: %v", err)
	}

	// Different hash
	skip, err := cache.CanSkipTransform("repo1", "hash2", "frontmatter")
	if err != nil {
		t.Fatalf("CanSkipTransform failed: %v", err)
	}
	if skip {
		t.Error("expected skip=false for different hash")
	}

	// Different transform
	skip, err = cache.CanSkipTransform("repo1", "hash1", "links")
	if err != nil {
		t.Fatalf("CanSkipTransform failed: %v", err)
	}
	if skip {
		t.Error("expected skip=false for different transform")
	}

	// Different repo
	skip, err = cache.CanSkipTransform("repo2", "hash1", "frontmatter")
	if err != nil {
		t.Fatalf("CanSkipTransform failed: %v", err)
	}
	if skip {
		t.Error("expected skip=false for different repo")
	}
}

func TestCanSkipTransformInvalidInputs(t *testing.T) {
	store := storage.NewMockStore()
	cache := NewStageCache(store)

	_, err := cache.CanSkipTransform("", "hash123", "frontmatter")
	if err == nil {
		t.Error("expected error for empty repoName")
	}

	_, err = cache.CanSkipTransform("repo1", "", "frontmatter")
	if err == nil {
		t.Error("expected error for empty repoHash")
	}

	_, err = cache.CanSkipTransform("repo1", "hash123", "")
	if err == nil {
		t.Error("expected error for empty transformName")
	}
}

func TestMixedScenario(t *testing.T) {
	// Simulates 5 unchanged repos, 1 changed repo
	store := storage.NewMockStore()
	cache := NewStageCache(store)

	// Save 6 repos
	for i := 1; i <= 6; i++ {
		repoName := "repo" + string(rune('0'+i))
		repoHash := "hash" + string(rune('0'+i))

		// Save clone
		treeData := []byte("tree data " + repoName)
		if err := cache.SaveClone(repoName, repoHash, "/workspace/"+repoName, treeData); err != nil {
			t.Fatalf("SaveClone failed for %s: %v", repoName, err)
		}

		// Save discovery
		docFiles := []docs.DocFile{
			{
				Repository:   repoName,
				Path:         "/workspace/" + repoName + "/docs/file.md",
				RelativePath: "file.md",
				Content:      []byte("content"),
			},
		}
		if err := cache.SaveDiscovery(repoName, repoHash, docFiles); err != nil {
			t.Fatalf("SaveDiscovery failed for %s: %v", repoName, err)
		}

		// Save transform
		if err := cache.SaveTransform(repoName, repoHash, "frontmatter", []byte("transformed")); err != nil {
			t.Fatalf("SaveTransform failed for %s: %v", repoName, err)
		}
	}

	// Check that first 5 repos can be skipped
	for i := 1; i <= 5; i++ {
		repoName := "repo" + string(rune('0'+i))
		repoHash := "hash" + string(rune('0'+i))

		skip, _, err := cache.CanSkipClone(repoName, repoHash)
		if err != nil {
			t.Fatalf("CanSkipClone failed for %s: %v", repoName, err)
		}
		if !skip {
			t.Errorf("expected repo %s to be skippable", repoName)
		}

		skip, _, err = cache.CanSkipDiscovery(repoName, repoHash)
		if err != nil {
			t.Fatalf("CanSkipDiscovery failed for %s: %v", repoName, err)
		}
		if !skip {
			t.Errorf("expected discovery for %s to be skippable", repoName)
		}

		skip, err = cache.CanSkipTransform(repoName, repoHash, "frontmatter")
		if err != nil {
			t.Fatalf("CanSkipTransform failed for %s: %v", repoName, err)
		}
		if !skip {
			t.Errorf("expected transform for %s to be skippable", repoName)
		}
	}

	// Repo 6 has changed (different hash)
	skip, _, err := cache.CanSkipClone("repo6", "hash-changed")
	if err != nil {
		t.Fatalf("CanSkipClone failed for repo6: %v", err)
	}
	if skip {
		t.Error("expected repo6 to NOT be skippable (hash changed)")
	}
}

func TestGetCachedClonePath(t *testing.T) {
	store := storage.NewMockStore()
	cache := NewStageCache(store)

	// Without cache, should return workspace path
	path := cache.GetCachedClonePath("repo1", "hash123", "/workspace")
	if path != "/workspace/repo1" {
		t.Errorf("expected /workspace/repo1, got %s", path)
	}

	// With cache, should return cached path
	treeData := []byte("tree data")
	if err := cache.SaveClone("repo1", "hash123", "/custom/path/repo1", treeData); err != nil {
		t.Fatalf("SaveClone failed: %v", err)
	}

	path = cache.GetCachedClonePath("repo1", "hash123", "/workspace")
	if path != "/custom/path/repo1" {
		t.Errorf("expected /custom/path/repo1, got %s", path)
	}
}

func TestSaveCloneInvalidInputs(t *testing.T) {
	store := storage.NewMockStore()
	cache := NewStageCache(store)

	err := cache.SaveClone("", "hash123", "/path", []byte("data"))
	if err == nil {
		t.Error("expected error for empty repoName")
	}

	err = cache.SaveClone("repo1", "", "/path", []byte("data"))
	if err == nil {
		t.Error("expected error for empty repoHash")
	}
}

func TestSaveDiscoveryInvalidInputs(t *testing.T) {
	store := storage.NewMockStore()
	cache := NewStageCache(store)

	docFiles := []docs.DocFile{{Repository: "repo1"}}

	err := cache.SaveDiscovery("", "hash123", docFiles)
	if err == nil {
		t.Error("expected error for empty repoName")
	}

	err = cache.SaveDiscovery("repo1", "", docFiles)
	if err == nil {
		t.Error("expected error for empty repoHash")
	}
}

func TestSaveTransformInvalidInputs(t *testing.T) {
	store := storage.NewMockStore()
	cache := NewStageCache(store)

	content := []byte("content")

	err := cache.SaveTransform("", "hash123", "frontmatter", content)
	if err == nil {
		t.Error("expected error for empty repoName")
	}

	err = cache.SaveTransform("repo1", "", "frontmatter", content)
	if err == nil {
		t.Error("expected error for empty repoHash")
	}

	err = cache.SaveTransform("repo1", "hash123", "", content)
	if err == nil {
		t.Error("expected error for empty transformName")
	}
}
