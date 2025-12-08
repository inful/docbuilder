package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/docs"
	"git.home.luguber.info/inful/docbuilder/internal/git"
	"git.home.luguber.info/inful/docbuilder/internal/storage"
)

func TestStoreIntegrationWithRepoTree(t *testing.T) {
	// Create temporary directory for storage
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, ".docbuilder")

	// Create object store
	store, err := storage.NewFSStore(storePath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Create a test repository
	repoPath := filepath.Join(tmpDir, "test-repo")
	if err := os.MkdirAll(filepath.Join(repoPath, "docs"), 0o750); err != nil {
		t.Fatalf("Failed to create repo: %v", err)
	}

	testFile := filepath.Join(repoPath, "docs", "readme.md")
	if err := os.WriteFile(testFile, []byte("# Test Documentation"), 0600); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	// Compute repository hash (using workdir since no git repo)
	repoHash, err := git.ComputeRepoHashFromWorkdir(repoPath, []string{"docs"})
	if err != nil {
		t.Fatalf("Failed to compute repo hash: %v", err)
	}

	// Store the repo tree in object store
	repoTreeData := []byte("repo-tree-content")
	obj := &storage.Object{
		Type: storage.ObjectTypeRepoTree,
		Data: repoTreeData,
		Metadata: storage.Metadata{
			Custom: map[string]string{
				"repository": "test-repo",
				"hash":       repoHash,
			},
		},
	}

	ctx := context.Background()
	storedHash, err := store.Put(ctx, obj)
	if err != nil {
		t.Fatalf("Failed to store object: %v", err)
	}

	// Verify we can retrieve it
	retrieved, err := store.Get(ctx, storedHash)
	if err != nil {
		t.Fatalf("Failed to retrieve object: %v", err)
	}

	if retrieved.Type != storage.ObjectTypeRepoTree {
		t.Errorf("Wrong object type: got %v, want %v", retrieved.Type, storage.ObjectTypeRepoTree)
	}

	if string(retrieved.Data) != string(repoTreeData) {
		t.Error("Object data mismatch")
	}

	if retrieved.Metadata.Custom["repository"] != "test-repo" {
		t.Error("Metadata lost")
	}
}

func TestStoreIntegrationWithDocsManifest(t *testing.T) {
	// Create temporary directory for storage
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, ".docbuilder")

	// Create object store
	store, err := storage.NewFSStore(storePath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Create test documentation files
	docFiles := []docs.DocFile{
		{
			Path:         "/repo/docs/readme.md",
			RelativePath: "readme.md",
			Repository:   "test-repo",
			Section:      "docs",
			Content:      []byte("# Documentation"),
		},
		{
			Path:         "/repo/docs/guide.md",
			RelativePath: "guide.md",
			Repository:   "test-repo",
			Section:      "docs",
			Content:      []byte("# Guide"),
		},
	}

	// Create docs manifest
	manifest, err := docs.CreateDocsManifest(docFiles)
	if err != nil {
		t.Fatalf("Failed to create manifest: %v", err)
	}

	// Serialize manifest
	manifestData, err := manifest.ToJSON()
	if err != nil {
		t.Fatalf("Failed to serialize manifest: %v", err)
	}

	// Store manifest in object store
	obj := &storage.Object{
		Type: storage.ObjectTypeDocsManifest,
		Data: manifestData,
		Metadata: storage.Metadata{
			Custom: map[string]string{
				"repository": "test-repo",
				"file_count": "2",
				"hash":       manifest.Hash,
			},
		},
	}

	ctx := context.Background()
	storedHash, err := store.Put(ctx, obj)
	if err != nil {
		t.Fatalf("Failed to store manifest: %v", err)
	}

	// Verify we can retrieve and deserialize
	retrieved, err := store.Get(ctx, storedHash)
	if err != nil {
		t.Fatalf("Failed to retrieve manifest: %v", err)
	}

	if retrieved.Type != storage.ObjectTypeDocsManifest {
		t.Errorf("Wrong object type: got %v, want %v", retrieved.Type, storage.ObjectTypeDocsManifest)
	}

	// Deserialize manifest
	retrievedManifest, err := docs.FromJSON(retrieved.Data)
	if err != nil {
		t.Fatalf("Failed to deserialize manifest: %v", err)
	}

	if retrievedManifest.Hash != manifest.Hash {
		t.Error("Manifest hash mismatch")
	}

	if retrievedManifest.FileCount() != 2 {
		t.Errorf("Expected 2 files, got %d", retrievedManifest.FileCount())
	}
}

func TestStoreIntegrationWithBuildRef(t *testing.T) {
	// Create temporary directory for storage
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, ".docbuilder")

	// Create object store
	store, err := storage.NewFSStore(storePath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Store multiple objects
	hash1, _ := store.Put(ctx, &storage.Object{
		Type: storage.ObjectTypeRepoTree,
		Data: []byte("repo1"),
	})

	hash2, _ := store.Put(ctx, &storage.Object{
		Type: storage.ObjectTypeDocsManifest,
		Data: []byte("docs1"),
	})

	hash3, _ := store.Put(ctx, &storage.Object{
		Type: storage.ObjectTypeArtifact,
		Data: []byte("artifact1"),
	})

	// Create build reference
	buildID := "build-test-123"
	buildHashes := []string{hash1, hash2, hash3}

	err = store.AddBuildRef(buildID, buildHashes)
	if err != nil {
		t.Fatalf("Failed to add build ref: %v", err)
	}

	// Retrieve build reference
	retrieved, err := store.GetBuildRef(buildID)
	if err != nil {
		t.Fatalf("Failed to get build ref: %v", err)
	}

	if len(retrieved) != len(buildHashes) {
		t.Errorf("Expected %d hashes, got %d", len(buildHashes), len(retrieved))
	}

	// Verify GC preserves referenced objects
	referenced := make(map[string]bool)
	for _, h := range buildHashes {
		referenced[h] = true
	}

	// Add unreferenced object
	hash4, _ := store.Put(ctx, &storage.Object{
		Type: storage.ObjectTypeArtifact,
		Data: []byte("unreferenced"),
	})

	// Run GC
	removed, err := store.GC(ctx, referenced)
	if err != nil {
		t.Fatalf("GC failed: %v", err)
	}

	if removed != 1 {
		t.Errorf("Expected 1 object removed, got %d", removed)
	}

	// Verify referenced objects still exist
	exists1, _ := store.Exists(ctx, hash1)
	exists2, _ := store.Exists(ctx, hash2)
	exists3, _ := store.Exists(ctx, hash3)

	if !exists1 || !exists2 || !exists3 {
		t.Error("Referenced objects were removed by GC")
	}

	// Verify unreferenced object was removed
	exists4, _ := store.Exists(ctx, hash4)
	if exists4 {
		t.Error("Unreferenced object was not removed by GC")
	}
}

func TestStoreIntegrationListByType(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, ".docbuilder")

	store, err := storage.NewFSStore(storePath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Store objects of different types
	store.Put(ctx, &storage.Object{Type: storage.ObjectTypeRepoTree, Data: []byte("tree1")})
	store.Put(ctx, &storage.Object{Type: storage.ObjectTypeRepoTree, Data: []byte("tree2")})
	store.Put(ctx, &storage.Object{Type: storage.ObjectTypeDocsManifest, Data: []byte("docs1")})
	store.Put(ctx, &storage.Object{Type: storage.ObjectTypeArtifact, Data: []byte("artifact1")})
	store.Put(ctx, &storage.Object{Type: storage.ObjectTypeArtifact, Data: []byte("artifact2")})
	store.Put(ctx, &storage.Object{Type: storage.ObjectTypeArtifact, Data: []byte("artifact3")})

	// List all objects
	all, err := store.List(ctx, "")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(all) != 6 {
		t.Errorf("Expected 6 total objects, got %d", len(all))
	}

	// List by type
	trees, _ := store.List(ctx, storage.ObjectTypeRepoTree)
	if len(trees) != 2 {
		t.Errorf("Expected 2 repo trees, got %d", len(trees))
	}

	manifests, _ := store.List(ctx, storage.ObjectTypeDocsManifest)
	if len(manifests) != 1 {
		t.Errorf("Expected 1 docs manifest, got %d", len(manifests))
	}

	artifacts, _ := store.List(ctx, storage.ObjectTypeArtifact)
	if len(artifacts) != 3 {
		t.Errorf("Expected 3 artifacts, got %d", len(artifacts))
	}
}

func TestStoreIntegrationWithConfig(t *testing.T) {
	// Demonstrate how config could be integrated with storage
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, ".docbuilder")

	store, err := storage.NewFSStore(storePath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Create a sample config
	cfg := &config.Config{
		Hugo: config.HugoConfig{
			Title:       "Test Documentation",
			Description: "Test site for storage integration",
		},
	}

	// Store config as artifact
	configData := []byte("hugo:\n  title: Test Documentation\n  description: Test site for storage integration")
	obj := &storage.Object{
		Type: storage.ObjectTypeArtifact,
		Data: configData,
		Metadata: storage.Metadata{
			Custom: map[string]string{
				"artifact_type": "hugo_config",
				"title":         cfg.Hugo.Title,
			},
		},
	}

	ctx := context.Background()
	hash, err := store.Put(ctx, obj)
	if err != nil {
		t.Fatalf("Failed to store config: %v", err)
	}

	// Retrieve and verify
	retrieved, err := store.Get(ctx, hash)
	if err != nil {
		t.Fatalf("Failed to retrieve config: %v", err)
	}

	if retrieved.Metadata.Custom["artifact_type"] != "hugo_config" {
		t.Error("Config metadata lost")
	}
}
