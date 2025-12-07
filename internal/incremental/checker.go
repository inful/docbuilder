package incremental

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/manifest"
	"git.home.luguber.info/inful/docbuilder/internal/storage"
)

// BuildCache tracks previous builds and their signatures for incremental checking.
type BuildCache struct {
	store    storage.ObjectStore
	logger   *slog.Logger
	cacheDir string
}

// NewBuildCache creates a new build cache.
func NewBuildCache(store storage.ObjectStore, cacheDir string) *BuildCache {
	return &BuildCache{
		store:    store,
		logger:   slog.Default(),
		cacheDir: cacheDir,
	}
}

// WithLogger sets a custom logger.
func (c *BuildCache) WithLogger(logger *slog.Logger) *BuildCache {
	c.logger = logger
	return c
}

// BuildCacheEntry represents a cached build with its signature and manifest.
type BuildCacheEntry struct {
	BuildID    string                  `json:"build_id"`
	Signature  *BuildSignature         `json:"signature"`
	Manifest   *manifest.BuildManifest `json:"manifest"`
	Timestamp  time.Time               `json:"timestamp"`
	OutputPath string                  `json:"output_path"`
}

// ShouldSkipBuild checks if a build with the given signature already exists.
// Returns true if the build can be skipped, along with the cached manifest.
func (c *BuildCache) ShouldSkipBuild(sig *BuildSignature) (bool, *BuildCacheEntry, error) {
	if sig == nil || sig.BuildHash == "" {
		return false, nil, fmt.Errorf("invalid signature")
	}

	ctx := context.Background()

	// Query store for builds with matching signature
	hashes, err := c.store.List(ctx, storage.ObjectTypeBuildManifest)
	if err != nil {
		return false, nil, fmt.Errorf("failed to list builds: %w", err)
	}

	// Load objects and sort by creation time descending to find most recent match
	type objWithTime struct {
		hash string
		obj  *storage.Object
	}
	var objects []objWithTime

	for _, hash := range hashes {
		obj, err := c.store.Get(ctx, hash)
		if err != nil {
			c.logger.Warn("Failed to get object", "hash", hash, "error", err)
			continue
		}
		objects = append(objects, objWithTime{hash: hash, obj: obj})
	}

	sort.Slice(objects, func(i, j int) bool {
		return objects[i].obj.Metadata.CreatedAt.After(objects[j].obj.Metadata.CreatedAt)
	})

	// Look for a build with matching signature
	for _, item := range objects {
		// Check if signature matches in metadata
		if storedSig, ok := item.obj.Metadata.Custom["signature"]; ok {
			if storedSig == sig.BuildHash {
				c.logger.Info("Found matching build signature",
					"build_hash", sig.BuildHash,
					"object_hash", item.hash)

				// Load the full manifest
				entry, err := c.loadCacheEntry(item.obj)
				if err != nil {
					c.logger.Warn("Failed to load cached build entry",
						"error", err,
						"object_hash", item.hash)
					continue
				}

				return true, entry, nil
			}
		}
	}

	c.logger.Debug("No matching build found", "signature", sig.BuildHash)
	return false, nil, nil
}

// loadCacheEntry loads a build cache entry from storage.
func (c *BuildCache) loadCacheEntry(obj *storage.Object) (*BuildCacheEntry, error) {
	// Parse the manifest from object data
	m, err := manifest.FromJSON(obj.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse manifest: %w", err)
	}

	// Extract signature from metadata
	sigHash, ok := obj.Metadata.Custom["signature"]
	if !ok {
		return nil, fmt.Errorf("signature not found in metadata")
	}

	// Create a signature object (we only have the hash, not full details)
	sig := &BuildSignature{
		BuildHash: sigHash,
	}

	entry := &BuildCacheEntry{
		BuildID:    m.ID,
		Signature:  sig,
		Manifest:   m,
		Timestamp:  obj.Metadata.CreatedAt,
		OutputPath: obj.Metadata.Custom["output_path"],
	}

	return entry, nil
}

// SaveBuild stores a completed build's signature and manifest.
func (c *BuildCache) SaveBuild(sig *BuildSignature, m *manifest.BuildManifest, outputPath string) error {
	if sig == nil || sig.BuildHash == "" {
		return fmt.Errorf("invalid signature")
	}
	if m == nil {
		return fmt.Errorf("manifest cannot be nil")
	}

	// Serialize manifest
	data, err := m.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize manifest: %w", err)
	}

	// Compute manifest hash
	manifestHash, err := m.Hash()
	if err != nil {
		return fmt.Errorf("failed to compute manifest hash: %w", err)
	}

	// Store manifest with signature in metadata
	metadata := storage.Metadata{
		CreatedAt: time.Now(),
		Custom: map[string]string{
			"signature":   sig.BuildHash,
			"build_id":    m.ID,
			"output_path": outputPath,
		},
	}

	obj := &storage.Object{
		Hash:     manifestHash,
		Type:     storage.ObjectTypeBuildManifest,
		Data:     data,
		Size:     int64(len(data)),
		Metadata: metadata,
	}

	ctx := context.Background()
	hash, err := c.store.Put(ctx, obj)
	if err != nil {
		return fmt.Errorf("failed to store build manifest: %w", err)
	}

	c.logger.Info("Cached build manifest",
		"build_id", m.ID,
		"signature", sig.BuildHash,
		"manifest_hash", hash)

	return nil
}

// CanReuseOutput checks if the output directory from a cached build still exists and is valid.
func (c *BuildCache) CanReuseOutput(entry *BuildCacheEntry) bool {
	if entry == nil || entry.OutputPath == "" {
		return false
	}

	// Check if output directory exists
	if _, err := os.Stat(entry.OutputPath); os.IsNotExist(err) {
		c.logger.Debug("Cached output directory does not exist", "path", entry.OutputPath)
		return false
	}

	// Check if it contains expected Hugo structure
	requiredPaths := []string{
		filepath.Join(entry.OutputPath, "hugo.yaml"),
		filepath.Join(entry.OutputPath, "content"),
	}

	for _, path := range requiredPaths {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			c.logger.Debug("Required path missing in cached output", "path", path)
			return false
		}
	}

	c.logger.Info("Cached output is reusable", "path", entry.OutputPath)
	return true
}

// InvalidateBuild removes a cached build from the store.
func (c *BuildCache) InvalidateBuild(buildID string) error {
	ctx := context.Background()

	hashes, err := c.store.List(ctx, storage.ObjectTypeBuildManifest)
	if err != nil {
		return fmt.Errorf("failed to list builds: %w", err)
	}

	for _, hash := range hashes {
		obj, err := c.store.Get(ctx, hash)
		if err != nil {
			c.logger.Warn("Failed to get object", "hash", hash, "error", err)
			continue
		}

		if id, ok := obj.Metadata.Custom["build_id"]; ok && id == buildID {
			if err := c.store.Delete(ctx, hash); err != nil {
				return fmt.Errorf("failed to delete build: %w", err)
			}
			c.logger.Info("Invalidated cached build", "build_id", buildID)
			return nil
		}
	}

	return fmt.Errorf("build not found: %s", buildID)
}
