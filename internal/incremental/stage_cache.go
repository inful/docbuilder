package incremental

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"

	"git.home.luguber.info/inful/docbuilder/internal/docs"
	"git.home.luguber.info/inful/docbuilder/internal/storage"
)

// StageCache provides per-stage caching for incremental builds.
type StageCache struct {
	store  storage.ObjectStore
	logger *slog.Logger
}

// NewStageCache creates a new stage cache.
func NewStageCache(store storage.ObjectStore) *StageCache {
	return &StageCache{
		store:  store,
		logger: slog.Default(),
	}
}

// WithLogger sets a custom logger.
func (c *StageCache) WithLogger(logger *slog.Logger) *StageCache {
	c.logger = logger
	return c
}

// RepoStageResult represents the cached result of a stage for a repository.
type RepoStageResult struct {
	RepoName  string   `json:"repo_name"`
	RepoHash  string   `json:"repo_hash"`
	Stage     string   `json:"stage"`
	ClonePath string   `json:"clone_path,omitempty"`
	DocFiles  []string `json:"doc_files,omitempty"`
	Success   bool     `json:"success"`
}

// CanSkipClone checks if a repository at a specific commit was already cloned.
// Returns true and the cached clone path if the repo can be skipped.
func (c *StageCache) CanSkipClone(repoName, repoHash string) (bool, string, error) {
	if repoName == "" || repoHash == "" {
		return false, "", fmt.Errorf("repoName and repoHash are required")
	}

	ctx := context.Background()

	// Query for repo tree objects matching this hash
	hashes, err := c.store.List(ctx, storage.ObjectTypeRepoTree)
	if err != nil {
		return false, "", fmt.Errorf("failed to list repo trees: %w", err)
	}

	// Look for matching repo tree
	for _, hash := range hashes {
		obj, err := c.store.Get(ctx, hash)
		if err != nil {
			c.logger.Warn("Failed to get repo tree", "hash", hash, "error", err)
			continue
		}

		// Check if this is the repo we're looking for
		if name, ok := obj.Metadata.Custom["repo_name"]; ok && name == repoName {
			if storedHash, ok := obj.Metadata.Custom["repo_hash"]; ok && storedHash == repoHash {
				clonePath := obj.Metadata.Custom["clone_path"]
				c.logger.Info("Found cached clone",
					"repo", repoName,
					"hash", repoHash,
					"path", clonePath)
				return true, clonePath, nil
			}
		}
	}

	c.logger.Debug("No cached clone found", "repo", repoName, "hash", repoHash)
	return false, "", nil
}

// SaveClone stores a cloned repository for future reuse.
func (c *StageCache) SaveClone(repoName, repoHash, clonePath string, treeData []byte) error {
	if repoName == "" || repoHash == "" {
		return fmt.Errorf("repoName and repoHash are required")
	}

	ctx := context.Background()

	metadata := storage.Metadata{
		Custom: map[string]string{
			"repo_name":  repoName,
			"repo_hash":  repoHash,
			"clone_path": clonePath,
			"stage":      "clone",
		},
	}

	obj := &storage.Object{
		Hash:     repoHash,
		Type:     storage.ObjectTypeRepoTree,
		Data:     treeData,
		Size:     int64(len(treeData)),
		Metadata: metadata,
	}

	hash, err := c.store.Put(ctx, obj)
	if err != nil {
		return fmt.Errorf("failed to store clone: %w", err)
	}

	c.logger.Info("Cached clone", "repo", repoName, "hash", hash)
	return nil
}

// CanSkipDiscovery checks if documentation was already discovered for a repo at this commit.
// Returns true and the cached doc files if discovery can be skipped.
func (c *StageCache) CanSkipDiscovery(repoName, repoHash string) (bool, []docs.DocFile, error) {
	if repoName == "" || repoHash == "" {
		return false, nil, fmt.Errorf("repoName and repoHash are required")
	}

	ctx := context.Background()

	// Query for docs manifests
	hashes, err := c.store.List(ctx, storage.ObjectTypeDocsManifest)
	if err != nil {
		return false, nil, fmt.Errorf("failed to list docs manifests: %w", err)
	}

	// Look for matching docs manifest
	for _, hash := range hashes {
		obj, err := c.store.Get(ctx, hash)
		if err != nil {
			c.logger.Warn("Failed to get docs manifest", "hash", hash, "error", err)
			continue
		}

		// Check if this manifest is for our repo
		if name, ok := obj.Metadata.Custom["repo_name"]; ok && name == repoName {
			if storedHash, ok := obj.Metadata.Custom["repo_hash"]; ok && storedHash == repoHash {
				// Parse the docs manifest
				manifest, err := docs.FromJSON(obj.Data)
				if err != nil {
					c.logger.Warn("Failed to parse docs manifest",
						"hash", hash,
						"error", err)
					continue
				}

				// Filter for this repo's files
				manifestFiles := manifest.FilterByRepository(repoName)

				// Convert manifest files back to DocFile (without content)
				docFiles := make([]docs.DocFile, len(manifestFiles))
				for i, mf := range manifestFiles {
					docFiles[i] = docs.DocFile{
						Path:         mf.Path,
						RelativePath: mf.RelativePath,
						Repository:   mf.Repository,
						Forge:        mf.Forge,
						Section:      mf.Section,
						Metadata:     mf.Metadata,
					}
				}

				c.logger.Info("Found cached discovery",
					"repo", repoName,
					"hash", repoHash,
					"files", len(docFiles))
				return true, docFiles, nil
			}
		}
	}

	c.logger.Debug("No cached discovery found", "repo", repoName, "hash", repoHash)
	return false, nil, nil
}

// SaveDiscovery stores discovered documentation for future reuse.
func (c *StageCache) SaveDiscovery(repoName, repoHash string, docFiles []docs.DocFile) error {
	if repoName == "" || repoHash == "" {
		return fmt.Errorf("repoName and repoHash are required")
	}

	ctx := context.Background()

	// Create a docs manifest for this repo
	manifest, err := docs.CreateDocsManifest(docFiles)
	if err != nil {
		return fmt.Errorf("failed to create docs manifest: %w", err)
	}

	data, err := manifest.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize docs manifest: %w", err)
	}

	hash, err := docs.ComputeDocsHash(docFiles)
	if err != nil {
		return fmt.Errorf("failed to compute docs hash: %w", err)
	}

	metadata := storage.Metadata{
		Custom: map[string]string{
			"repo_name": repoName,
			"repo_hash": repoHash,
			"stage":     "discovery",
		},
	}

	obj := &storage.Object{
		Hash:     hash,
		Type:     storage.ObjectTypeDocsManifest,
		Data:     data,
		Size:     int64(len(data)),
		Metadata: metadata,
	}

	storedHash, err := c.store.Put(ctx, obj)
	if err != nil {
		return fmt.Errorf("failed to store discovery: %w", err)
	}

	c.logger.Info("Cached discovery",
		"repo", repoName,
		"files", len(docFiles),
		"hash", storedHash)
	return nil
}

// CanSkipTransform checks if transforms were already applied to content at this state.
// Returns true if transforms can be skipped.
func (c *StageCache) CanSkipTransform(repoName, repoHash, transformName string) (bool, error) {
	if repoName == "" || repoHash == "" || transformName == "" {
		return false, fmt.Errorf("repoName, repoHash, and transformName are required")
	}

	ctx := context.Background()

	// Query for transformed content
	hashes, err := c.store.List(ctx, storage.ObjectTypeTransformedContent)
	if err != nil {
		return false, fmt.Errorf("failed to list transformed content: %w", err)
	}

	// Look for matching transformed content
	for _, hash := range hashes {
		obj, err := c.store.Get(ctx, hash)
		if err != nil {
			c.logger.Warn("Failed to get transformed content", "hash", hash, "error", err)
			continue
		}

		// Check if this transform matches
		if name, ok := obj.Metadata.Custom["repo_name"]; ok && name == repoName {
			if storedHash, ok := obj.Metadata.Custom["repo_hash"]; ok && storedHash == repoHash {
				if transform, ok := obj.Metadata.Custom["transform"]; ok && transform == transformName {
					c.logger.Info("Found cached transform",
						"repo", repoName,
						"hash", repoHash,
						"transform", transformName)
					return true, nil
				}
			}
		}
	}

	c.logger.Debug("No cached transform found",
		"repo", repoName,
		"hash", repoHash,
		"transform", transformName)
	return false, nil
}

// SaveTransform stores transformed content for future reuse.
func (c *StageCache) SaveTransform(repoName, repoHash, transformName string, content []byte) error {
	if repoName == "" || repoHash == "" || transformName == "" {
		return fmt.Errorf("repoName, repoHash, and transformName are required")
	}

	ctx := context.Background()

	// Compute content hash
	emptyHash, err := docs.ComputeDocsHash(nil)
	if err != nil {
		return fmt.Errorf("failed to compute empty hash: %w", err)
	}
	hash := fmt.Sprintf("%s-%s-%s", repoHash, transformName, emptyHash)

	metadata := storage.Metadata{
		Custom: map[string]string{
			"repo_name": repoName,
			"repo_hash": repoHash,
			"transform": transformName,
			"stage":     "transform",
		},
	}

	obj := &storage.Object{
		Hash:     hash,
		Type:     storage.ObjectTypeTransformedContent,
		Data:     content,
		Size:     int64(len(content)),
		Metadata: metadata,
	}

	storedHash, err := c.store.Put(ctx, obj)
	if err != nil {
		return fmt.Errorf("failed to store transform: %w", err)
	}

	c.logger.Info("Cached transform",
		"repo", repoName,
		"transform", transformName,
		"hash", storedHash)
	return nil
}

// GetCachedClonePath returns the cached clone path for a repo, or empty string if not cached.
func (c *StageCache) GetCachedClonePath(repoName, repoHash, workspaceDir string) string {
	canSkip, path, err := c.CanSkipClone(repoName, repoHash)
	if err != nil {
		c.logger.Warn("Error checking clone cache", "error", err)
		return ""
	}

	if canSkip && path != "" {
		return path
	}

	// Return expected path based on workspace
	return filepath.Join(workspaceDir, repoName)
}
