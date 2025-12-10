package main

import (
	"fmt"
	"log/slog"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/git"
	"git.home.luguber.info/inful/docbuilder/internal/versioning"
)

// expandRepositoriesWithVersions takes the base repository configuration and expands
// it into multiple repositories if versioning is enabled, one per version.
func expandRepositoriesWithVersions(gitClient *git.Client, cfg *config.Config) ([]config.Repository, error) {
	// If no versioning config, return repos as-is
	if cfg.Versioning == nil || cfg.Versioning.DefaultBranchOnly {
		return cfg.Repositories, nil
	}

	versionManager := versioning.NewVersionManager(gitClient)
	var expandedRepos []config.Repository

	for _, repo := range cfg.Repositories {
		// Convert config.VersioningConfig to versioning.VersionConfig
		versionConfig := &versioning.VersionConfig{
			Strategy:    versioning.VersionStrategy(cfg.Versioning.Strategy),
			MaxVersions: cfg.Versioning.MaxVersionsPerRepo,
		}

		// Add patterns if specified
		if len(cfg.Versioning.BranchPatterns) > 0 {
			versionConfig.BranchPatterns = cfg.Versioning.BranchPatterns
		}
		if len(cfg.Versioning.TagPatterns) > 0 {
			versionConfig.TagPatterns = cfg.Versioning.TagPatterns
		}

		// Discover versions for this repository
		result, err := versionManager.DiscoverVersions(repo.URL, versionConfig)
		if err != nil {
			slog.Warn("Failed to discover versions for repository, using single version",
				"repo", repo.Name,
				"error", err)
			// Fallback to single version
			expandedRepos = append(expandedRepos, repo)
			continue
		}

		// Create a repository entry for each discovered version
		if len(result.Repository.Versions) == 0 {
			slog.Warn("No versions found for repository, using default branch",
				"repo", repo.Name)
			expandedRepos = append(expandedRepos, repo)
			continue
		}

		slog.Info("Discovered versions for repository",
			"repo", repo.Name,
			"versions", len(result.Repository.Versions))

		for _, version := range result.Repository.Versions {
			versionedRepo := repo // Copy base config

			// Set version-specific fields
			versionedRepo.Branch = version.Name // Use Name as branch/tag reference
			versionedRepo.Version = version.DisplayName
			versionedRepo.IsVersioned = true

			// Update name to include version for uniqueness
			versionedRepo.Name = fmt.Sprintf("%s-%s", repo.Name, version.DisplayName)

			// Add version metadata to tags
			if versionedRepo.Tags == nil {
				versionedRepo.Tags = make(map[string]string)
			}
			versionedRepo.Tags["version"] = version.DisplayName
			versionedRepo.Tags["version_type"] = string(version.Type)
			versionedRepo.Tags["base_repo"] = repo.Name

			expandedRepos = append(expandedRepos, versionedRepo)
		}
	}

	slog.Info("Repository expansion complete",
		"original", len(cfg.Repositories),
		"expanded", len(expandedRepos))

	return expandedRepos, nil
}
