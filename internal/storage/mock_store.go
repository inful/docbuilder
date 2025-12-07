package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// MockStore is an in-memory implementation of ObjectStore for testing.
type MockStore struct {
	mu      sync.RWMutex
	objects map[string]*Object
	calls   MockCalls
}

// MockCalls tracks method invocations for test verification.
type MockCalls struct {
	Put    int
	Get    int
	Exists int
	Delete int
	List   int
}

// NewMockStore creates a new in-memory object store.
func NewMockStore() *MockStore {
	return &MockStore{
		objects: make(map[string]*Object),
	}
}

// Put stores an object and returns its content hash.
func (m *MockStore) Put(ctx context.Context, obj *Object) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls.Put++

	// Compute hash if not provided
	hash := obj.Hash
	if hash == "" {
		h := sha256.Sum256(obj.Data)
		hash = hex.EncodeToString(h[:])
	}

	// Check if object already exists
	if existing, ok := m.objects[hash]; ok {
		// Update ref count
		existing.Metadata.RefCount++
		existing.Metadata.LastAccessed = time.Now()
		return hash, nil
	}

	// Store new object
	stored := &Object{
		Hash: hash,
		Type: obj.Type,
		Size: int64(len(obj.Data)),
		Data: make([]byte, len(obj.Data)),
		Metadata: Metadata{
			CreatedAt:    time.Now(),
			LastAccessed: time.Now(),
			RefCount:     1,
			Custom:       make(map[string]string),
		},
	}
	copy(stored.Data, obj.Data)

	// Copy custom metadata
	for k, v := range obj.Metadata.Custom {
		stored.Metadata.Custom[k] = v
	}

	m.objects[hash] = stored
	return hash, nil
}

// Get retrieves an object by its content hash.
func (m *MockStore) Get(ctx context.Context, hash string) (*Object, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	m.calls.Get++

	obj, ok := m.objects[hash]
	if !ok {
		return nil, ErrNotFound{Hash: hash}
	}

	// Update last accessed time
	obj.Metadata.LastAccessed = time.Now()

	// Return a copy to prevent external modification
	result := &Object{
		Hash: obj.Hash,
		Type: obj.Type,
		Size: obj.Size,
		Data: make([]byte, len(obj.Data)),
		Metadata: Metadata{
			CreatedAt:    obj.Metadata.CreatedAt,
			LastAccessed: obj.Metadata.LastAccessed,
			RefCount:     obj.Metadata.RefCount,
			Custom:       make(map[string]string),
		},
	}
	copy(result.Data, obj.Data)
	for k, v := range obj.Metadata.Custom {
		result.Metadata.Custom[k] = v
	}

	return result, nil
}

// Exists checks if an object with the given hash exists.
func (m *MockStore) Exists(ctx context.Context, hash string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	m.calls.Exists++

	_, ok := m.objects[hash]
	return ok, nil
}

// Delete removes an object by its content hash.
func (m *MockStore) Delete(ctx context.Context, hash string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls.Delete++

	if _, ok := m.objects[hash]; !ok {
		return ErrNotFound{Hash: hash}
	}

	delete(m.objects, hash)
	return nil
}

// List returns all object hashes matching the given type filter.
func (m *MockStore) List(ctx context.Context, objectType ObjectType) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	m.calls.List++

	var hashes []string
	for hash, obj := range m.objects {
		if objectType == "" || obj.Type == objectType {
			hashes = append(hashes, hash)
		}
	}
	return hashes, nil
}

// Close releases resources (no-op for mock).
func (m *MockStore) Close() error {
	return nil
}

// GetCalls returns the number of times each method was called.
func (m *MockStore) GetCalls() MockCalls {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.calls
}

// Reset clears all stored objects and call counts.
func (m *MockStore) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.objects = make(map[string]*Object)
	m.calls = MockCalls{}
}

// GetObject returns a stored object (for testing).
func (m *MockStore) GetObject(hash string) (*Object, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	obj, ok := m.objects[hash]
	return obj, ok
}

// Size returns the number of stored objects.
func (m *MockStore) Size() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.objects)
}

// String returns a string representation for debugging.
func (m *MockStore) String() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return fmt.Sprintf("MockStore{objects: %d, calls: %+v}", len(m.objects), m.calls)
}
