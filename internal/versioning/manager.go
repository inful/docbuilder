package versioning

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/git"
)

// DefaultVersionManager implements VersionManager using Git operations
type DefaultVersionManager struct {
	gitClient    *git.Client
	repositories map[string]*RepositoryVersions // In-memory cache
	mu           sync.RWMutex                   // Protects repositories map
}

// NewVersionManager creates a new version manager
func NewVersionManager(gitClient *git.Client) *DefaultVersionManager {
	return &DefaultVersionManager{
		gitClient:    gitClient,
		repositories: make(map[string]*RepositoryVersions),
	}
}

// DiscoverVersions discovers available versions for a repository
func (vm *DefaultVersionManager) DiscoverVersions(repoURL string, config *VersionConfig) (*VersionDiscoveryResult, error) {
	slog.Info("Discovering versions for repository", "repo_url", repoURL, "strategy", config.Strategy)

	// Get existing versions for comparison
	vm.mu.RLock()
	existing, exists := vm.repositories[repoURL]
	vm.mu.RUnlock()

	result := &VersionDiscoveryResult{
		Repository: &RepositoryVersions{
			RepositoryURL:    repoURL,
			Versions:         make([]*Version, 0),
			LastDiscovery:    time.Now(),
			MaxVersionsLimit: config.MaxVersions,
		},
	}

	// Get Git references from the repository
	refs, err := vm.getGitReferences(repoURL)
	if err != nil {
		return nil, fmt.Errorf("failed to get git references: %w", err)
	}

	// Determine default branch
	defaultBranch, err := vm.getDefaultBranch(repoURL, refs)
	if err != nil {
		return nil, fmt.Errorf("failed to determine default branch: %w", err)
	}
	result.Repository.DefaultBranch = defaultBranch

	// Filter and convert references to versions based on strategy
	versions := vm.filterAndConvertReferences(refs, config, defaultBranch)

	// Sort versions by creation time (newest first)
	sort.Slice(versions, func(i, j int) bool {
		return versions[i].CreatedAt.After(versions[j].CreatedAt)
	})

	// Apply version limits
	if config.MaxVersions > 0 && len(versions) > config.MaxVersions {
		versions = versions[:config.MaxVersions]
	}

	result.Repository.Versions = versions

	// Calculate changes compared to existing versions
	if exists {
		result.NewCount, result.UpdatedCount, result.RemovedCount = vm.calculateVersionChanges(existing.Versions, versions)
	} else {
		result.NewCount = len(versions)
	}

	slog.Info("Version discovery completed",
		"repo_url", repoURL,
		"total_versions", len(versions),
		"new", result.NewCount,
		"updated", result.UpdatedCount,
		"removed", result.RemovedCount)

	return result, nil
}

// GetRepositoryVersions returns all versions for a repository
func (vm *DefaultVersionManager) GetRepositoryVersions(repoURL string) (*RepositoryVersions, error) {
	vm.mu.RLock()
	defer vm.mu.RUnlock()

	if versions, exists := vm.repositories[repoURL]; exists {
		// Return a copy to prevent external modification
		return vm.copyRepositoryVersions(versions), nil
	}

	return nil, fmt.Errorf("repository not found: %s", repoURL)
}

// GetDefaultVersion returns the default version for a repository
func (vm *DefaultVersionManager) GetDefaultVersion(repoURL string) (*Version, error) {
	versions, err := vm.GetRepositoryVersions(repoURL)
	if err != nil {
		return nil, err
	}

	for _, version := range versions.Versions {
		if version.IsDefault {
			return version, nil
		}
	}

	return nil, fmt.Errorf("no default version found for repository: %s", repoURL)
}

// UpdateVersions updates the versions for a repository
func (vm *DefaultVersionManager) UpdateVersions(repoURL string, versions []*Version) error {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	if existing, exists := vm.repositories[repoURL]; exists {
		existing.Versions = versions
		existing.LastDiscovery = time.Now()
	} else {
		vm.repositories[repoURL] = &RepositoryVersions{
			RepositoryURL: repoURL,
			Versions:      versions,
			LastDiscovery: time.Now(),
		}
	}

	slog.Debug("Updated versions for repository", "repo_url", repoURL, "count", len(versions))
	return nil
}

// CleanupOldVersions removes old versions based on retention policy
func (vm *DefaultVersionManager) CleanupOldVersions(repoURL string, config *VersionConfig) error {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	versions, exists := vm.repositories[repoURL]
	if !exists {
		return fmt.Errorf("repository not found: %s", repoURL)
	}

	originalCount := len(versions.Versions)

	// Sort by creation time (newest first)
	sort.Slice(versions.Versions, func(i, j int) bool {
		return versions.Versions[i].CreatedAt.After(versions.Versions[j].CreatedAt)
	})

	// Apply max versions limit
	if config.MaxVersions > 0 && len(versions.Versions) > config.MaxVersions {
		// Always keep the default version
		var defaultVersion *Version
		nonDefaultVersions := make([]*Version, 0)

		for _, v := range versions.Versions {
			if v.IsDefault {
				defaultVersion = v
			} else {
				nonDefaultVersions = append(nonDefaultVersions, v)
			}
		}

		// Keep the most recent non-default versions
		maxNonDefault := config.MaxVersions - 1 // Reserve one slot for default
		if maxNonDefault < 0 {
			maxNonDefault = 0
		}

		if len(nonDefaultVersions) > maxNonDefault {
			nonDefaultVersions = nonDefaultVersions[:maxNonDefault]
		}

		// Reconstruct the versions list
		newVersions := make([]*Version, 0, len(nonDefaultVersions)+1)
		if defaultVersion != nil {
			newVersions = append(newVersions, defaultVersion)
		}
		newVersions = append(newVersions, nonDefaultVersions...)

		versions.Versions = newVersions
	}

	removedCount := originalCount - len(versions.Versions)
	if removedCount > 0 {
		slog.Info("Cleaned up old versions", "repo_url", repoURL, "removed", removedCount)
	}

	return nil
}

// ListAllRepositories returns all repositories with versions
func (vm *DefaultVersionManager) ListAllRepositories() ([]*RepositoryVersions, error) {
	vm.mu.RLock()
	defer vm.mu.RUnlock()

	repositories := make([]*RepositoryVersions, 0, len(vm.repositories))
	for _, repo := range vm.repositories {
		repositories = append(repositories, vm.copyRepositoryVersions(repo))
	}

	return repositories, nil
}

// getGitReferences retrieves all Git references (branches and tags) from the repository
func (vm *DefaultVersionManager) getGitReferences(repoURL string) ([]*GitReference, error) {
	// This is a placeholder implementation
	// In a real implementation, this would use git ls-remote or similar
	// For now, we'll simulate some references

	references := []*GitReference{
		{
			Name:      "main",
			Type:      VersionTypeBranch,
			CommitSHA: "abc123",
			CreatedAt: time.Now().Add(-30 * 24 * time.Hour),
		},
		{
			Name:      "develop",
			Type:      VersionTypeBranch,
			CommitSHA: "def456",
			CreatedAt: time.Now().Add(-7 * 24 * time.Hour),
		},
		{
			Name:      "v1.0.0",
			Type:      VersionTypeTag,
			CommitSHA: "ghi789",
			CreatedAt: time.Now().Add(-14 * 24 * time.Hour),
		},
		{
			Name:      "v1.1.0",
			Type:      VersionTypeTag,
			CommitSHA: "jkl012",
			CreatedAt: time.Now().Add(-3 * 24 * time.Hour),
		},
	}

	return references, nil
}

// getDefaultBranch determines the default branch for the repository
func (vm *DefaultVersionManager) getDefaultBranch(repoURL string, refs []*GitReference) (string, error) {
	// Look for common default branch names
	defaultCandidates := []string{"main", "master", "trunk"}

	for _, candidate := range defaultCandidates {
		for _, ref := range refs {
			if ref.Type == VersionTypeBranch && ref.Name == candidate {
				return candidate, nil
			}
		}
	}

	// If no common default found, use the first branch
	for _, ref := range refs {
		if ref.Type == VersionTypeBranch {
			return ref.Name, nil
		}
	}

	return "", fmt.Errorf("no branches found in repository: %s", repoURL)
}

// filterAndConvertReferences filters Git references based on configuration and converts to versions
func (vm *DefaultVersionManager) filterAndConvertReferences(refs []*GitReference, config *VersionConfig, defaultBranch string) []*Version {
	versions := make([]*Version, 0)

	for _, ref := range refs {
		version := vm.convertReferenceToVersion(ref, config, defaultBranch)
		if version != nil {
			versions = append(versions, version)
		}
	}

	return versions
}

// convertReferenceToVersion converts a Git reference to a Version if it matches the configuration
func (vm *DefaultVersionManager) convertReferenceToVersion(ref *GitReference, config *VersionConfig, defaultBranch string) *Version {
	// Check if this reference should be included based on strategy
	include := false

	switch config.Strategy {
	case StrategyDefaultOnly:
		include = ref.Type == VersionTypeBranch && ref.Name == defaultBranch
	case StrategyBranches:
		include = ref.Type == VersionTypeBranch && vm.matchesPatterns(ref.Name, config.BranchPatterns)
	case StrategyTags:
		include = ref.Type == VersionTypeTag && vm.matchesPatterns(ref.Name, config.TagPatterns)
	case StrategyBranchesAndTags:
		include = (ref.Type == VersionTypeBranch && vm.matchesPatterns(ref.Name, config.BranchPatterns)) ||
			(ref.Type == VersionTypeTag && vm.matchesPatterns(ref.Name, config.TagPatterns))
	}

	if !include {
		return nil
	}

	// Create version
	version := &Version{
		Name:         ref.Name,
		Type:         ref.Type,
		DisplayName:  vm.generateDisplayName(ref.Name, string(ref.Type)),
		IsDefault:    ref.Type == VersionTypeBranch && ref.Name == defaultBranch,
		Path:         vm.generateVersionPath(ref.Name, string(ref.Type), defaultBranch),
		CommitSHA:    ref.CommitSHA,
		CreatedAt:    ref.CreatedAt,
		LastModified: time.Now(), // Would be updated during documentation processing
		DocsPath:     "docs",     // Default documentation path
	}

	return version
}

// matchesPatterns checks if a name matches any of the given patterns
func (vm *DefaultVersionManager) matchesPatterns(name string, patterns []string) bool {
	if len(patterns) == 0 {
		return true // No patterns means match all
	}

	for _, pattern := range patterns {
		matched, err := filepath.Match(pattern, name)
		if err != nil {
			slog.Warn("Invalid pattern", "pattern", pattern, "error", err)
			continue
		}
		if matched {
			return true
		}

		// Also try regex matching for more complex patterns
		if strings.Contains(pattern, "*") {
			// Convert glob pattern to regex
			regexPattern := strings.ReplaceAll(pattern, "*", ".*")
			if matched, _ := regexp.MatchString("^"+regexPattern+"$", name); matched {
				return true
			}
		}
	}

	return false
}

// generateDisplayName creates a human-readable display name for the version
func (vm *DefaultVersionManager) generateDisplayName(name, refType string) string {
	if refType == "tag" {
		// For tags, use the tag name as-is (it's usually semantic version)
		return name
	}

	// For branches, make it more readable
	switch name {
	case "main", "master":
		return "Latest"
	case "develop", "development":
		return "Development"
	default:
		// Capitalize first letter and replace dashes/underscores with spaces
		displayName := strings.ReplaceAll(name, "-", " ")
		displayName = strings.ReplaceAll(displayName, "_", " ")
		if len(displayName) > 0 {
			displayName = strings.ToUpper(displayName[:1]) + displayName[1:]
		}
		return displayName
	}
}

// generateVersionPath creates a Hugo-compatible path for the version
func (vm *DefaultVersionManager) generateVersionPath(name, refType, defaultBranch string) string {
	if refType == "branch" && name == defaultBranch {
		return "latest" // Default branch uses "latest" path
	}

	// Sanitize the name for use in URLs
	path := strings.ToLower(name)
	path = regexp.MustCompile(`[^a-z0-9.-]`).ReplaceAllString(path, "-")
	path = regexp.MustCompile(`-+`).ReplaceAllString(path, "-")
	path = strings.Trim(path, "-")

	return path
}

// calculateVersionChanges compares old and new versions to determine changes
func (vm *DefaultVersionManager) calculateVersionChanges(oldVersions, newVersions []*Version) (int, int, int) {
	oldMap := make(map[string]*Version)
	for _, v := range oldVersions {
		oldMap[v.Name] = v
	}

	newMap := make(map[string]*Version)
	for _, v := range newVersions {
		newMap[v.Name] = v
	}

	newCount := 0
	updatedCount := 0

	// Count new and updated versions
	for name, newVersion := range newMap {
		if oldVersion, exists := oldMap[name]; exists {
			// Check if version was updated (different commit SHA)
			if oldVersion.CommitSHA != newVersion.CommitSHA {
				updatedCount++
			}
		} else {
			newCount++
		}
	}

	// Count removed versions
	removedCount := 0
	for name := range oldMap {
		if _, exists := newMap[name]; !exists {
			removedCount++
		}
	}

	return newCount, updatedCount, removedCount
}

// copyRepositoryVersions creates a deep copy of RepositoryVersions
func (vm *DefaultVersionManager) copyRepositoryVersions(src *RepositoryVersions) *RepositoryVersions {
	dst := &RepositoryVersions{
		RepositoryURL:    src.RepositoryURL,
		DefaultBranch:    src.DefaultBranch,
		LastDiscovery:    src.LastDiscovery,
		MaxVersionsLimit: src.MaxVersionsLimit,
		Versions:         make([]*Version, len(src.Versions)),
	}

	for i, v := range src.Versions {
		dst.Versions[i] = &Version{
			Name:         v.Name,
			Type:         v.Type,
			DisplayName:  v.DisplayName,
			IsDefault:    v.IsDefault,
			Path:         v.Path,
			CommitSHA:    v.CommitSHA,
			CreatedAt:    v.CreatedAt,
			LastModified: v.LastModified,
			DocsPath:     v.DocsPath,
		}
	}

	return dst
}
