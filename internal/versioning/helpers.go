package versioning

import (
	"fmt"
	"log/slog"
	"maps"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/git"
)

// ExpandRepositoriesWithVersions takes the base repository configuration and expands
// it into multiple repositories if versioning is enabled, one per version.
func ExpandRepositoriesWithVersions(gitClient *git.Client, cfg *config.Config) ([]config.Repository, error) {
	// If versioning is disabled or not configured, return repos as-is
	if cfg.Versioning == nil || !cfg.Versioning.Enabled || cfg.Versioning.DefaultBranchOnly {
		return cfg.Repositories, nil
	}

	versionManager := NewVersionManager(gitClient)
	var expandedRepos []config.Repository

	for i := range cfg.Repositories {
		repo := &cfg.Repositories[i]
		// Convert config.VersioningConfig to versioning.VersionConfig
		versionConfig := &VersionConfig{
			Strategy:    VersionStrategy(cfg.Versioning.Strategy),
			MaxVersions: cfg.Versioning.MaxVersionsPerRepo,
		}

		// Add patterns if specified
		if len(cfg.Versioning.BranchPatterns) > 0 {
			versionConfig.BranchPatterns = cfg.Versioning.BranchPatterns
		}
		if len(cfg.Versioning.TagPatterns) > 0 {
			versionConfig.TagPatterns = cfg.Versioning.TagPatterns
		}

		// Discover versions for this repository (pass repo for auth)
		result, err := versionManager.DiscoverVersionsWithAuth(repo.URL, versionConfig, repo.Auth)
		if err != nil {
			slog.Warn("Failed to discover versions for repository, using single version",
				"repo", repo.Name,
				"error", err)
			// Fallback to single version
			expandedRepos = append(expandedRepos, *repo)
			continue
		}

		// The names of the versions must be unique: the path of a version
		// is a lossy version of its name, and two versions can share it
		// ("release/1.0" and "release-1.0"). The second clone of a name
		// deletes the workspace of the first one, and the pages of the
		// first version vanish from the site.
		usedNames := make(map[string]int)

		// Create a repository entry for each discovered version
		if len(result.Repository.Versions) == 0 {
			slog.Warn("No versions found for repository, using default branch",
				"repo", repo.Name)
			expandedRepos = append(expandedRepos, *repo)
			continue
		}

		slog.Info("Discovered versions for repository",
			"repo", repo.Name,
			"versions", len(result.Repository.Versions))

		for _, version := range result.Repository.Versions {
			versionedRepo := *repo // Copy base config by dereferencing pointer

			// Set version-specific fields
			versionedRepo.Branch = version.Name // Use Name as branch/tag reference
			versionedRepo.Version = version.DisplayName
			versionedRepo.IsVersioned = true
			versionedRepo.IsTag = (version.Type == VersionTypeTag)

			slog.Debug("Creating versioned repository",
				"name", repo.Name,
				"version", version.Name,
				"type", string(version.Type),
				"is_tag", versionedRepo.IsTag)

			// Update name to include version for uniqueness. The path of
			// the version is used and not its display name: the display
			// name holds spaces, and the name of a branch may hold a
			// slash. The name of a repository is a path, and two
			// versions that share one overwrote each other.
			versionedRepo.Name = fmt.Sprintf("%s-%s", repo.Name, version.Path)
			if count, taken := usedNames[versionedRepo.Name]; taken {
				versionedRepo.Name = fmt.Sprintf("%s-%d", versionedRepo.Name, count+1)
			}
			usedNames[versionedRepo.Name]++

			// Add version metadata to tags. The map is copied: the base
			// repository and every version would otherwise share it, and
			// the version of the last one would be the version of all of
			// them.
			versionedRepo.Tags = maps.Clone(repo.Tags)
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
