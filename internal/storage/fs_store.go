package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// FSStore is a filesystem-based implementation of ObjectStore.
// It stores objects in a content-addressable layout:
//
//	.docbuilder/
//	  objects/
//	    ab/
//	      cd1234... (first 2 chars = subdir, rest = filename)
//	  refs/
//	    builds/
//	      build-123 (file containing list of object hashes)
type FSStore struct {
	basePath string
	mu       sync.RWMutex
}

// NewFSStore creates a new filesystem-based object store.
func NewFSStore(basePath string) (*FSStore, error) {
	// Create directory structure
	dirs := []string{
		filepath.Join(basePath, "objects"),
		filepath.Join(basePath, "refs", "builds"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0750); err != nil {
			return nil, fmt.Errorf("create directory %s: %w", dir, err)
		}
	}

	return &FSStore{basePath: basePath}, nil
}

// Put stores an object and returns its content hash.
func (fs *FSStore) Put(ctx context.Context, obj *Object) (string, error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	// Compute hash if not provided
	hash := obj.Hash
	if hash == "" {
		h := sha256.Sum256(obj.Data)
		hash = hex.EncodeToString(h[:])
	}

	// Check if already exists
	objectPath := fs.objectPath(hash)
	if _, err := os.Stat(objectPath); err == nil {
		// Object exists, update ref count
		metadata, err := fs.readMetadata(hash)
		if err == nil {
			metadata.RefCount++
			metadata.LastAccessed = time.Now()
			if err := fs.writeMetadata(hash, metadata); err != nil {
				return hash, fmt.Errorf("update metadata: %w", err)
			}
		}
		return hash, nil
	}

	// Create object directory
	objectDir := filepath.Dir(objectPath)
	if err := os.MkdirAll(objectDir, 0750); err != nil {
		return "", fmt.Errorf("create object directory: %w", err)
	}

	// Write object data
	if err := os.WriteFile(objectPath, obj.Data, 0600); err != nil {
		return "", fmt.Errorf("write object: %w", err)
	}

	// Write metadata
	metadata := Metadata{
		CreatedAt:    time.Now(),
		LastAccessed: time.Now(),
		RefCount:     1,
		Custom:       make(map[string]string),
	}
	for k, v := range obj.Metadata.Custom {
		metadata.Custom[k] = v
	}

	// Store object type in metadata
	metadata.Custom["object_type"] = string(obj.Type)

	if err := fs.writeMetadata(hash, metadata); err != nil {
		return hash, fmt.Errorf("write metadata: %w", err)
	}

	return hash, nil
}

// Get retrieves an object by its content hash.
func (fs *FSStore) Get(ctx context.Context, hash string) (*Object, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	objectPath := fs.objectPath(hash)
	// #nosec G304 - objectPath is internal, constructed from sanitized hash
	data, err := os.ReadFile(objectPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound{Hash: hash}
		}
		return nil, fmt.Errorf("read object: %w", err)
	}

	// Read metadata
	metadata, err := fs.readMetadata(hash)
	if err != nil {
		// Create default metadata if missing
		metadata = Metadata{
			CreatedAt:    time.Now(),
			LastAccessed: time.Now(),
			RefCount:     1,
			Custom:       make(map[string]string),
		}
	}

	// Update last accessed
	metadata.LastAccessed = time.Now()
	if err := fs.writeMetadata(hash, metadata); err != nil {
		// Log but don't fail on metadata update
		fmt.Fprintf(os.Stderr, "Warning: failed to update metadata for %s: %v\n", hash, err)
	}

	// Extract object type from metadata
	objectType := ObjectType(metadata.Custom["object_type"])

	return &Object{
		Hash:     hash,
		Type:     objectType,
		Size:     int64(len(data)),
		Data:     data,
		Metadata: metadata,
	}, nil
}

// Exists checks if an object with the given hash exists.
func (fs *FSStore) Exists(ctx context.Context, hash string) (bool, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	objectPath := fs.objectPath(hash)
	_, err := os.Stat(objectPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("stat object: %w", err)
	}
	return true, nil
}

// Delete removes an object by its content hash.
func (fs *FSStore) Delete(ctx context.Context, hash string) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	return fs.deleteUnlocked(ctx, hash)
}

// List returns all object hashes matching the given type filter.
func (fs *FSStore) List(ctx context.Context, objectType ObjectType) ([]string, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	return fs.listUnlocked(ctx, objectType)
}

// Close releases resources.
func (fs *FSStore) Close() error {
	return nil
}

// GC performs garbage collection, removing unreferenced objects.
func (fs *FSStore) GC(ctx context.Context, referencedHashes map[string]bool) (int, error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	// List all objects without acquiring lock (we already have it)
	allHashes, err := fs.listUnlocked(ctx, "")
	if err != nil {
		return 0, fmt.Errorf("list objects: %w", err)
	}

	removed := 0
	for _, hash := range allHashes {
		if !referencedHashes[hash] {
			if err := fs.deleteUnlocked(ctx, hash); err != nil && !IsNotFound(err) {
				return removed, fmt.Errorf("delete object %s: %w", hash, err)
			}
			removed++
		}
	}

	return removed, nil
}

// listUnlocked is an internal version of List that doesn't acquire locks.
func (fs *FSStore) listUnlocked(ctx context.Context, objectType ObjectType) ([]string, error) {
	var hashes []string
	objectsDir := filepath.Join(fs.basePath, "objects")

	// Walk the objects directory
	err := filepath.Walk(objectsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories and metadata files
		if info.IsDir() || strings.HasSuffix(path, ".meta.json") {
			return nil
		}

		// Extract hash from path
		relPath, err := filepath.Rel(objectsDir, path)
		if err != nil {
			return nil
		}

		// Remove directory separator to get full hash
		hash := strings.ReplaceAll(relPath, string(filepath.Separator), "")

		// Filter by type if specified
		if objectType != "" {
			metadata, err := fs.readMetadata(hash)
			if err == nil {
				if ObjectType(metadata.Custom["object_type"]) != objectType {
					return nil
				}
			}
		}

		hashes = append(hashes, hash)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("walk objects: %w", err)
	}

	return hashes, nil
}

// deleteUnlocked is an internal version of Delete that doesn't acquire locks.
func (fs *FSStore) deleteUnlocked(ctx context.Context, hash string) error {
	objectPath := fs.objectPath(hash)
	if err := os.Remove(objectPath); err != nil {
		if os.IsNotExist(err) {
			return ErrNotFound{Hash: hash}
		}
		return fmt.Errorf("delete object: %w", err)
	}

	// Delete metadata
	metadataPath := fs.metadataPath(hash)
	os.Remove(metadataPath) // Best effort

	// Try to remove empty directory
	objectDir := filepath.Dir(objectPath)
	os.Remove(objectDir) // Best effort

	return nil
}

// objectPath returns the filesystem path for an object.
func (fs *FSStore) objectPath(hash string) string {
	if len(hash) < 2 {
		return filepath.Join(fs.basePath, "objects", hash)
	}
	// Use first 2 chars as directory, rest as filename
	return filepath.Join(fs.basePath, "objects", hash[:2], hash[2:])
}

// metadataPath returns the filesystem path for object metadata.
func (fs *FSStore) metadataPath(hash string) string {
	return fs.objectPath(hash) + ".meta.json"
}

// readMetadata reads object metadata from disk.
func (fs *FSStore) readMetadata(hash string) (Metadata, error) {
	metadataPath := fs.metadataPath(hash)
	// #nosec G304 - metadataPath is internal, constructed from sanitized hash
	data, err := os.ReadFile(metadataPath)
	if err != nil {
		return Metadata{}, fmt.Errorf("read metadata: %w", err)
	}

	var metadata Metadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return Metadata{}, fmt.Errorf("unmarshal metadata: %w", err)
	}

	return metadata, nil
}

// writeMetadata writes object metadata to disk.
func (fs *FSStore) writeMetadata(hash string, metadata Metadata) error {
	metadataPath := fs.metadataPath(hash)

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(metadataPath), 0750); err != nil {
		return fmt.Errorf("create metadata directory: %w", err)
	}

	data, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}

	if err := os.WriteFile(metadataPath, data, 0600); err != nil {
		return fmt.Errorf("write metadata: %w", err)
	}

	return nil
}

// AddBuildRef associates a build ID with a set of object hashes.
func (fs *FSStore) AddBuildRef(buildID string, hashes []string) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	refPath := filepath.Join(fs.basePath, "refs", "builds", buildID)
	data := strings.Join(hashes, "\n")

	return os.WriteFile(refPath, []byte(data), 0600)
}

// GetBuildRef retrieves object hashes for a build ID.
func (fs *FSStore) GetBuildRef(buildID string) ([]string, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	refPath := filepath.Join(fs.basePath, "refs", "builds", buildID)
	// #nosec G304 - refPath is internal, buildID is sanitized
	data, err := os.ReadFile(refPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read build ref: %w", err)
	}

	lines := strings.Split(string(data), "\n")
	var hashes []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			hashes = append(hashes, line)
		}
	}

	return hashes, nil
}
