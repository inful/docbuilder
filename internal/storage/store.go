// Package storage provides content-addressable storage for DocBuilder artifacts.
package storage

import (
	"context"
	"io"
	"time"
)

// ObjectStore provides content-addressable storage for build artifacts.
// Objects are stored by their content hash, enabling deduplication and
// efficient cache invalidation.
type ObjectStore interface {
	// Put stores an object and returns its content hash.
	// If the object already exists, it returns the existing hash without writing.
	Put(ctx context.Context, obj *Object) (hash string, err error)

	// Get retrieves an object by its content hash.
	// Returns ErrNotFound if the object doesn't exist.
	Get(ctx context.Context, hash string) (*Object, error)

	// Exists checks if an object with the given hash exists.
	Exists(ctx context.Context, hash string) (bool, error)

	// Delete removes an object by its content hash.
	// Returns ErrNotFound if the object doesn't exist.
	Delete(ctx context.Context, hash string) error

	// List returns all object hashes matching the given type filter.
	// If objectType is empty, returns all objects.
	List(ctx context.Context, objectType ObjectType) ([]string, error)

	// Close releases any resources held by the store.
	Close() error
}

// Object represents a stored artifact with its metadata.
type Object struct {
	// Hash is the content hash (SHA256) of the data.
	Hash string

	// Type identifies the kind of object.
	Type ObjectType

	// Size is the size of the data in bytes.
	Size int64

	// Data is the object content.
	// For large objects, this may be nil and should be read via GetReader.
	Data []byte

	// Metadata stores additional key-value pairs.
	Metadata Metadata

	// Reader provides streaming access to large objects (optional).
	Reader io.ReadCloser
}

// Metadata stores object metadata.
type Metadata struct {
	// CreatedAt is when the object was first stored.
	CreatedAt time.Time

	// LastAccessed is when the object was last retrieved.
	LastAccessed time.Time

	// RefCount tracks how many builds reference this object.
	// Used for garbage collection.
	RefCount int

	// Custom allows storage-specific metadata.
	Custom map[string]string
}

// ObjectType identifies the kind of stored object.
type ObjectType string

const (
	// ObjectTypeRepoTree represents a Merkle tree hash of a repository state.
	ObjectTypeRepoTree ObjectType = "repo_tree"

	// ObjectTypeDocsManifest represents a hash of discovered documentation files.
	ObjectTypeDocsManifest ObjectType = "docs_manifest"

	// ObjectTypeTransformedContent represents transformed documentation content.
	ObjectTypeTransformedContent ObjectType = "transformed_content"

	// ObjectTypeArtifact represents a final build artifact (Hugo config, generated page, etc.).
	ObjectTypeArtifact ObjectType = "artifact"

	// ObjectTypeBuildManifest represents a complete build manifest.
	ObjectTypeBuildManifest ObjectType = "build_manifest"
)

// ErrNotFound is returned when an object doesn't exist.
type ErrNotFound struct {
	Hash string
}

func (e ErrNotFound) Error() string {
	return "object not found: " + e.Hash
}

// IsNotFound returns true if the error is ErrNotFound.
func IsNotFound(err error) bool {
	_, ok := err.(ErrNotFound)
	return ok
}
