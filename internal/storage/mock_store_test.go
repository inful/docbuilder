package storage

import (
	"context"
	"testing"
)

func TestMockStorePutAndGet(t *testing.T) {
	store := NewMockStore()
	ctx := context.Background()

	data := []byte("test content for repo tree")
	obj := &Object{
		Type: ObjectTypeRepoTree,
		Data: data,
		Metadata: Metadata{
			Custom: map[string]string{"repo": "test-repo"},
		},
	}

	hash, err := store.Put(ctx, obj)
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}
	if hash == "" {
		t.Fatal("Put returned empty hash")
	}

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
	if retrieved.Metadata.Custom["repo"] != "test-repo" {
		t.Error("Custom metadata lost")
	}
}

func TestMockStoreExists(t *testing.T) {
	store := NewMockStore()
	ctx := context.Background()

	exists, err := store.Exists(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if exists {
		t.Error("Exists returned true for non-existent object")
	}

	obj := &Object{Type: ObjectTypeDocsManifest, Data: []byte("docs manifest data")}
	hash, _ := store.Put(ctx, obj)

	exists, err = store.Exists(ctx, hash)
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if !exists {
		t.Error("Exists returned false for existing object")
	}
}

func TestMockStoreDelete(t *testing.T) {
	store := NewMockStore()
	ctx := context.Background()

	obj := &Object{Type: ObjectTypeArtifact, Data: []byte("artifact data")}
	hash, _ := store.Put(ctx, obj)

	err := store.Delete(ctx, hash)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	exists, _ := store.Exists(ctx, hash)
	if exists {
		t.Error("Object still exists after Delete")
	}

	err = store.Delete(ctx, hash)
	if !IsNotFound(err) {
		t.Errorf("Expected ErrNotFound, got %v", err)
	}
}

func TestMockStoreList(t *testing.T) {
	store := NewMockStore()
	ctx := context.Background()

	store.Put(ctx, &Object{Type: ObjectTypeRepoTree, Data: []byte("tree1")})
	store.Put(ctx, &Object{Type: ObjectTypeRepoTree, Data: []byte("tree2")})
	store.Put(ctx, &Object{Type: ObjectTypeDocsManifest, Data: []byte("manifest1")})
	store.Put(ctx, &Object{Type: ObjectTypeArtifact, Data: []byte("artifact1")})

	allHashes, err := store.List(ctx, "")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(allHashes) != 4 {
		t.Errorf("Expected 4 objects, got %d", len(allHashes))
	}

	trees, err := store.List(ctx, ObjectTypeRepoTree)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(trees) != 2 {
		t.Errorf("Expected 2 repo trees, got %d", len(trees))
	}

	manifests, err := store.List(ctx, ObjectTypeDocsManifest)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(manifests) != 1 {
		t.Errorf("Expected 1 manifest, got %d", len(manifests))
	}
}

func TestMockStoreDeduplication(t *testing.T) {
	store := NewMockStore()
	ctx := context.Background()

	data := []byte("duplicate content")
	obj1 := &Object{Type: ObjectTypeRepoTree, Data: data}
	obj2 := &Object{Type: ObjectTypeRepoTree, Data: data}

	hash1, _ := store.Put(ctx, obj1)
	hash2, _ := store.Put(ctx, obj2)

	if hash1 != hash2 {
		t.Errorf("Expected same hash for duplicate content, got %s and %s", hash1, hash2)
	}

	if store.Size() != 1 {
		t.Errorf("Expected 1 object, got %d", store.Size())
	}

	obj, _ := store.GetObject(hash1)
	if obj.Metadata.RefCount != 2 {
		t.Errorf("Expected ref count 2, got %d", obj.Metadata.RefCount)
	}
}

func TestMockStoreCallTracking(t *testing.T) {
	store := NewMockStore()
	ctx := context.Background()

	obj := &Object{Type: ObjectTypeArtifact, Data: []byte("test")}
	hash, _ := store.Put(ctx, obj)
	store.Get(ctx, hash)
	store.Exists(ctx, hash)
	store.List(ctx, "")
	store.Delete(ctx, hash)

	calls := store.GetCalls()
	if calls.Put != 1 {
		t.Errorf("Expected 1 Put call, got %d", calls.Put)
	}
	if calls.Get != 1 {
		t.Errorf("Expected 1 Get call, got %d", calls.Get)
	}
	if calls.Exists != 1 {
		t.Errorf("Expected 1 Exists call, got %d", calls.Exists)
	}
	if calls.List != 1 {
		t.Errorf("Expected 1 List call, got %d", calls.List)
	}
	if calls.Delete != 1 {
		t.Errorf("Expected 1 Delete call, got %d", calls.Delete)
	}
}

func TestMockStoreReset(t *testing.T) {
	store := NewMockStore()
	ctx := context.Background()

	store.Put(ctx, &Object{Type: ObjectTypeArtifact, Data: []byte("test1")})
	store.Put(ctx, &Object{Type: ObjectTypeArtifact, Data: []byte("test2")})

	if store.Size() != 2 {
		t.Errorf("Expected 2 objects before reset, got %d", store.Size())
	}

	store.Reset()

	if store.Size() != 0 {
		t.Errorf("Expected 0 objects after reset, got %d", store.Size())
	}

	calls := store.GetCalls()
	if calls.Put != 0 {
		t.Errorf("Expected call count reset, got Put=%d", calls.Put)
	}
}
