package versioning

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// VersionService integrates version management with DocBuilder
type VersionService struct {
	manager VersionManager
	config  *VersionConfig
}

// NewVersionService creates a new version service
func NewVersionService(manager VersionManager, config *VersionConfig) *VersionService {
	return &VersionService{
		manager: manager,
		config:  config,
	}
}

// DiscoverRepositoryVersions discovers versions for a repository
func (vs *VersionService) DiscoverRepositoryVersions(repoURL string) (*VersionDiscoveryResult, error) {
	slog.Info("Discovering versions for repository", "repo_url", repoURL)

	result, err := vs.manager.DiscoverVersions(repoURL, vs.config)
	if err != nil {
		return nil, fmt.Errorf("failed to discover versions: %w", err)
	}

	// Update the manager's internal state
	if err := vs.manager.UpdateVersions(repoURL, result.Repository.Versions); err != nil {
		slog.Error("Failed to update versions", "repo_url", repoURL, "error", err)
	}

	// Cleanup old versions if needed
	if vs.config.MaxVersions > 0 {
		if err := vs.manager.CleanupOldVersions(repoURL, vs.config); err != nil {
			slog.Error("Failed to cleanup old versions", "repo_url", repoURL, "error", err)
		}
	}

	return result, nil
}

// GetVersioningConfig creates a VersionConfig from V2Config
func GetVersioningConfig(v2Config *config.Config) *VersionConfig {
	if v2Config.Versioning == nil {
		// Return default configuration
		return &VersionConfig{
			Strategy:          StrategyDefaultOnly,
			DefaultBranchOnly: true,
			BranchPatterns:    []string{"main", "master"},
			TagPatterns:       []string{},
			MaxVersions:       5,
		}
	}

	strategy := StrategyDefaultOnly
	switch v2Config.Versioning.Strategy {
	case "branches":
		strategy = StrategyBranches
	case "tags":
		strategy = StrategyTags
	case "branches_and_tags":
		strategy = StrategyBranchesAndTags
	}

	return &VersionConfig{
		Strategy:          strategy,
		DefaultBranchOnly: v2Config.Versioning.DefaultBranchOnly,
		BranchPatterns:    v2Config.Versioning.BranchPatterns,
		TagPatterns:       v2Config.Versioning.TagPatterns,
		MaxVersions:       v2Config.Versioning.MaxVersionsPerRepo,
	}
}

// CreateVersionAwareContentPath creates a content path that includes version information
func CreateVersionAwareContentPath(repoName, versionPath, originalPath string) string {
	// Remove any existing docs prefix
	cleanPath := strings.TrimPrefix(originalPath, "docs/")
	cleanPath = strings.TrimPrefix(cleanPath, "/")

	// Create version-aware path: content/{repo}/{version}/{path}
	return filepath.Join(repoName, versionPath, cleanPath)
}

// CreateVersionMetadata creates metadata for a versioned document
type VersionMetadata struct {
	Version         string `json:"version"`
	VersionDisplay  string `json:"version_display"`
	VersionType     string `json:"version_type"`
	IsDefault       bool   `json:"is_default_version"`
	VersionPath     string `json:"version_path"`
	Repository      string `json:"repository"`
	RepositoryURL   string `json:"repository_url"`
	CommitSHA       string `json:"commit_sha"`
	CreatedAt       string `json:"created_at"`
	LastModified    string `json:"last_modified"`
}

func CreateVersionMetadata(version *Version, repoVersions *RepositoryVersions) VersionMetadata {
	repoName := sanitizeRepoName(repoVersions.RepositoryURL)
	return VersionMetadata{
		Version:        version.Name,
		VersionDisplay: version.DisplayName,
		VersionType:    string(version.Type),
		IsDefault:      version.IsDefault,
		VersionPath:    version.Path,
		Repository:     repoName,
		RepositoryURL:  repoVersions.RepositoryURL,
		CommitSHA:      version.CommitSHA,
		CreatedAt:      version.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		LastModified:   version.LastModified.Format("2006-01-02T15:04:05Z07:00"),
	}
}

// sanitizeRepoName creates a filesystem-safe repository name
func sanitizeRepoName(repoURL string) string {
	// Extract repository name from URL
	parts := strings.Split(repoURL, "/")
	name := parts[len(parts)-1]
	name = strings.TrimSuffix(name, ".git")

	// Sanitize for filesystem/URL use
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, " ", "-")
	name = strings.ReplaceAll(name, "_", "-")

	return name
}
