package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestFSStorePutAndGet(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFSStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFSStore failed: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	data := []byte("test content for filesystem store")
	obj := &Object{
		Type: ObjectTypeRepoTree,
		Data: data,
		Metadata: Metadata{
			Custom: map[string]string{"test": "value"},
		},
	}

	// Put object
	hash, err := store.Put(ctx, obj)
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}
	if hash == "" {
		t.Fatal("Put returned empty hash")
	}

	// Verify object file exists
	objectPath := store.objectPath(hash)
	if _, err := os.Stat(objectPath); err != nil {
		t.Errorf("Object file not created: %v", err)
	}

	// Get object
	retrieved, err := store.Get(ctx, hash)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if string(retrieved.Data) != string(data) {
		t.Errorf("Got data %q, want %q", retrieved.Data, data)
	}
	if retrieved.Type != ObjectTypeRepoTree {
		t.Errorf("Got type %v, want %v", retrieved.Type, ObjectTypeRepoTree)
	}
}

func TestFSStoreExists(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFSStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFSStore failed: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Check non-existent
	exists, err := store.Exists(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if exists {
		t.Error("Exists returned true for non-existent object")
	}

	// Store object
	obj := &Object{Type: ObjectTypeArtifact, Data: []byte("test")}
	hash, _ := store.Put(ctx, obj)

	// Check exists
	exists, err = store.Exists(ctx, hash)
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if !exists {
		t.Error("Exists returned false for existing object")
	}
}

func TestFSStoreDelete(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFSStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFSStore failed: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Store object
	obj := &Object{Type: ObjectTypeDocsManifest, Data: []byte("manifest")}
	hash, _ := store.Put(ctx, obj)

	// Delete object
	err = store.Delete(ctx, hash)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify deleted
	exists, _ := store.Exists(ctx, hash)
	if exists {
		t.Error("Object still exists after Delete")
	}

	// Delete again should fail
	err = store.Delete(ctx, hash)
	if !IsNotFound(err) {
		t.Errorf("Expected ErrNotFound, got %v", err)
	}
}

func TestFSStoreList(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFSStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFSStore failed: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Store multiple objects
	store.Put(ctx, &Object{Type: ObjectTypeRepoTree, Data: []byte("tree1")})
	store.Put(ctx, &Object{Type: ObjectTypeRepoTree, Data: []byte("tree2")})
	store.Put(ctx, &Object{Type: ObjectTypeDocsManifest, Data: []byte("manifest1")})
	store.Put(ctx, &Object{Type: ObjectTypeArtifact, Data: []byte("artifact1")})

	// List all
	allHashes, err := store.List(ctx, "")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(allHashes) != 4 {
		t.Errorf("Expected 4 objects, got %d", len(allHashes))
	}

	// List repo trees
	trees, err := store.List(ctx, ObjectTypeRepoTree)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(trees) != 2 {
		t.Errorf("Expected 2 repo trees, got %d", len(trees))
	}
}

func TestFSStoreDeduplication(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFSStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFSStore failed: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Store same content twice
	data := []byte("duplicate content")
	obj1 := &Object{Type: ObjectTypeArtifact, Data: data}
	obj2 := &Object{Type: ObjectTypeArtifact, Data: data}

	hash1, _ := store.Put(ctx, obj1)
	hash2, _ := store.Put(ctx, obj2)

	// Should get same hash
	if hash1 != hash2 {
		t.Errorf("Expected same hash, got %s and %s", hash1, hash2)
	}

	// Verify ref count increased
	metadata, err := store.readMetadata(hash1)
	if err != nil {
		t.Fatalf("Read metadata failed: %v", err)
	}
	if metadata.RefCount != 2 {
		t.Errorf("Expected ref count 2, got %d", metadata.RefCount)
	}
}

func TestFSStoreGC(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFSStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFSStore failed: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Store objects
	hash1, _ := store.Put(ctx, &Object{Type: ObjectTypeArtifact, Data: []byte("keep1")})
	hash2, _ := store.Put(ctx, &Object{Type: ObjectTypeArtifact, Data: []byte("keep2")})
	hash3, _ := store.Put(ctx, &Object{Type: ObjectTypeArtifact, Data: []byte("remove1")})
	hash4, _ := store.Put(ctx, &Object{Type: ObjectTypeArtifact, Data: []byte("remove2")})

	// Mark hash1 and hash2 as referenced
	referenced := map[string]bool{
		hash1: true,
		hash2: true,
	}

	// Run GC
	removed, err := store.GC(ctx, referenced)
	if err != nil {
		t.Fatalf("GC failed: %v", err)
	}
	if removed != 2 {
		t.Errorf("Expected 2 removed, got %d", removed)
	}

	// Verify hash1 and hash2 still exist
	exists1, _ := store.Exists(ctx, hash1)
	exists2, _ := store.Exists(ctx, hash2)
	if !exists1 || !exists2 {
		t.Error("Referenced objects were removed")
	}

	// Verify hash3 and hash4 were removed
	exists3, _ := store.Exists(ctx, hash3)
	exists4, _ := store.Exists(ctx, hash4)
	if exists3 || exists4 {
		t.Error("Unreferenced objects were not removed")
	}
}

func TestFSStoreBuildRefs(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFSStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFSStore failed: %v", err)
	}
	defer store.Close()

	buildID := "build-123"
	hashes := []string{"hash1", "hash2", "hash3"}

	// Add build ref
	err = store.AddBuildRef(buildID, hashes)
	if err != nil {
		t.Fatalf("AddBuildRef failed: %v", err)
	}

	// Get build ref
	retrieved, err := store.GetBuildRef(buildID)
	if err != nil {
		t.Fatalf("GetBuildRef failed: %v", err)
	}

	if len(retrieved) != len(hashes) {
		t.Errorf("Expected %d hashes, got %d", len(hashes), len(retrieved))
	}

	for i, hash := range hashes {
		if retrieved[i] != hash {
			t.Errorf("Hash[%d]: got %s, want %s", i, retrieved[i], hash)
		}
	}
}

func TestFSStoreObjectPath(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFSStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFSStore failed: %v", err)
	}

	hash := "abcdef1234567890"
	expectedPath := filepath.Join(tmpDir, "objects", "ab", "cdef1234567890")
	actualPath := store.objectPath(hash)

	if actualPath != expectedPath {
		t.Errorf("Got path %s, want %s", actualPath, expectedPath)
	}

	// Test short hash
	shortHash := "a"
	expectedShortPath := filepath.Join(tmpDir, "objects", "a")
	actualShortPath := store.objectPath(shortHash)

	if actualShortPath != expectedShortPath {
		t.Errorf("Got short path %s, want %s", actualShortPath, expectedShortPath)
	}
}

func TestFSStoreGetNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFSStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFSStore failed: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	_, err = store.Get(ctx, "nonexistent")
	if !IsNotFound(err) {
		t.Errorf("Expected ErrNotFound, got %v", err)
	}
}
