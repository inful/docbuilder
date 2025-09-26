package forge

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// DiscoveryService handles repository discovery across multiple forges
type DiscoveryService struct {
	forgeManager *ForgeManager
	filtering    *config.FilteringConfig
}

// NewDiscoveryService creates a new discovery service
func NewDiscoveryService(manager *ForgeManager, filtering *config.FilteringConfig) *DiscoveryService {
	return &DiscoveryService{
		forgeManager: manager,
		filtering:    filtering,
	}
}

// DiscoveryResult contains the results of a discovery operation
type DiscoveryResult struct {
	Repositories  []*Repository              `json:"repositories"`
	Organizations map[string][]*Organization `json:"organizations"` // Keyed by forge name
	Filtered      []*Repository              `json:"filtered"`      // Repositories that were filtered out
	Errors        map[string]error           `json:"errors"`        // Errors by forge name
	Timestamp     time.Time                  `json:"timestamp"`
	Duration      time.Duration              `json:"duration"`
}

// DiscoverAll discovers repositories from all configured forges
func (ds *DiscoveryService) DiscoverAll(ctx context.Context) (*DiscoveryResult, error) {
	startTime := time.Now()
	result := &DiscoveryResult{
		Repositories:  make([]*Repository, 0),
		Organizations: make(map[string][]*Organization),
		Filtered:      make([]*Repository, 0),
		Errors:        make(map[string]error),
		Timestamp:     startTime,
	}

	for forgeName, client := range ds.forgeManager.GetAllForges() {
		slog.Info("Starting discovery", "forge", forgeName)

		// Discover repositories for this forge
		repos, orgs, filtered, err := ds.discoverForge(ctx, client)
		if err != nil {
			result.Errors[forgeName] = err
			slog.Error("Discovery failed", "forge", forgeName, "error", err)
			continue
		}

		result.Repositories = append(result.Repositories, repos...)
		result.Organizations[forgeName] = orgs
		result.Filtered = append(result.Filtered, filtered...)

		slog.Info("Discovery completed",
			"forge", forgeName,
			"repositories", len(repos),
			"organizations", len(orgs),
			"filtered", len(filtered))
	}

	result.Duration = time.Since(startTime)

	slog.Info("Discovery summary",
		"total_repositories", len(result.Repositories),
		"total_filtered", len(result.Filtered),
		"duration", result.Duration,
		"errors", len(result.Errors))

	return result, nil
}

// discoverForge discovers repositories from a single forge
func (ds *DiscoveryService) discoverForge(ctx context.Context, client ForgeClient) ([]*Repository, []*Organization, []*Repository, error) {
	forgeConfig := ds.forgeManager.GetForgeConfigs()[client.GetName()]
	if forgeConfig == nil {
		return nil, nil, nil, fmt.Errorf("forge configuration not found for %s", client.GetName())
	}

	// Determine which organizations/groups to scan
	var targetOrgs []string
	targetOrgs = append(targetOrgs, forgeConfig.Organizations...)
	targetOrgs = append(targetOrgs, forgeConfig.Groups...)

	if len(targetOrgs) == 0 {
		// If no specific orgs/groups configured, enter auto-discovery mode and enumerate all accessible organizations
		slog.Info("Entering auto-discovery mode (no organizations/groups configured)", "forge", client.GetName())
		// If no specific orgs configured, discover all accessible ones
		orgs, err := client.ListOrganizations(ctx)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to list organizations: %w", err)
		}
		for _, org := range orgs {
			targetOrgs = append(targetOrgs, org.Name)
		}
		slog.Info("Auto-discovered organizations", "forge", client.GetName(), "count", len(orgs))
	}

	// Get all organizations (for metadata)
	organizations, err := client.ListOrganizations(ctx)
	if err != nil {
		slog.Warn("Failed to get organization metadata", "forge", client.GetName(), "error", err)
		organizations = make([]*Organization, 0)
	}

	// Discover repositories
	repositories, err := client.ListRepositories(ctx, targetOrgs)
	if err != nil {
		return nil, organizations, nil, fmt.Errorf("failed to list repositories: %w", err)
	}

	// Check documentation status and apply filtering
	originalCount := len(repositories)
	var validRepos []*Repository
	var filteredRepos []*Repository

	for _, repo := range repositories {
		// Check if repository has documentation
		if err := client.CheckDocumentation(ctx, repo); err != nil {
			slog.Warn("Failed to check documentation status",
				"forge", client.GetName(),
				"repository", repo.FullName,
				"error", err)
			// Continue processing, but assume no docs
			repo.HasDocs = false
			repo.HasDocIgnore = false
		}

		// Apply filtering logic
		if ds.shouldIncludeRepository(repo) {
			validRepos = append(validRepos, repo)
		} else {
			filteredRepos = append(filteredRepos, repo)
			slog.Debug("Repository filtered out",
				"forge", client.GetName(),
				"repository", repo.FullName,
				"reason", ds.getFilterReason(repo))
		}
	}

	if originalCount > 0 && len(validRepos) == 0 {
		if len(ds.filtering.IncludePatterns) > 0 {
			for _, p := range ds.filtering.IncludePatterns {
				if strings.Contains(p, "/") {
					slog.Warn("All repositories filtered: include_patterns contains path-like pattern which won't match repository names", "pattern", p)
					break
				}
			}
		}
		slog.Warn("All repositories filtered out by configuration",
			"forge", client.GetName(),
			"total_before", originalCount,
			"required_paths", ds.filtering.RequiredPaths,
			"include_patterns", ds.filtering.IncludePatterns,
			"exclude_patterns", ds.filtering.ExcludePatterns)
	}

	return validRepos, organizations, filteredRepos, nil
}

// shouldIncludeRepository determines if a repository should be included based on filtering config
func (ds *DiscoveryService) shouldIncludeRepository(repo *Repository) bool {
	// Skip archived repositories
	if repo.Archived {
		return false
	}

	// Check for .docignore file
	if repo.HasDocIgnore {
		return false
	}

	// Check if repository has required paths (e.g., docs folder)
	if !repo.HasDocs && len(ds.filtering.RequiredPaths) > 0 {
		return false
	}

	// Check include patterns
	if len(ds.filtering.IncludePatterns) > 0 {
		included := false
		for _, pattern := range ds.filtering.IncludePatterns {
			if matchesPattern(repo.Name, pattern) || matchesPattern(repo.FullName, pattern) {
				included = true
				break
			}
		}
		if !included {
			return false
		}
	}

	// Check exclude patterns
	for _, pattern := range ds.filtering.ExcludePatterns {
		if matchesPattern(repo.Name, pattern) || matchesPattern(repo.FullName, pattern) {
			return false
		}
	}

	return true
}

// getFilterReason returns a human-readable reason why a repository was filtered out
func (ds *DiscoveryService) getFilterReason(repo *Repository) string {
	if repo.Archived {
		return "archived"
	}
	if repo.HasDocIgnore {
		return "has .docignore"
	}
	if !repo.HasDocs && len(ds.filtering.RequiredPaths) > 0 {
		return "missing required docs paths"
	}

	// Check include patterns
	if len(ds.filtering.IncludePatterns) > 0 {
		included := false
		for _, pattern := range ds.filtering.IncludePatterns {
			if matchesPattern(repo.Name, pattern) || matchesPattern(repo.FullName, pattern) {
				included = true
				break
			}
		}
		if !included {
			return "doesn't match include patterns"
		}
	}

	// Check exclude patterns
	for _, pattern := range ds.filtering.ExcludePatterns {
		if matchesPattern(repo.Name, pattern) || matchesPattern(repo.FullName, pattern) {
			return "matches exclude pattern: " + pattern
		}
	}

	return "unknown"
}

// matchesPattern checks if a string matches a simple glob pattern
// This is a basic implementation - could be enhanced with proper glob matching
func matchesPattern(str, pattern string) bool {
	// Simple wildcard matching
	if pattern == "*" {
		return true
	}

	// Exact match
	if pattern == str {
		return true
	}

	// Prefix match with *
	if len(pattern) > 1 && pattern[len(pattern)-1] == '*' {
		prefix := pattern[:len(pattern)-1]
		return len(str) >= len(prefix) && str[:len(prefix)] == prefix
	}

	// Suffix match with *
	if len(pattern) > 1 && pattern[0] == '*' {
		suffix := pattern[1:]
		return len(str) >= len(suffix) && str[len(str)-len(suffix):] == suffix
	}

	// Contains match with *pattern*
	if len(pattern) > 2 && pattern[0] == '*' && pattern[len(pattern)-1] == '*' {
		substring := pattern[1 : len(pattern)-1]
		return contains(str, substring)
	}

	return false
}

// contains checks if a string contains a substring (case-insensitive)
func contains(str, substr string) bool {
	if len(substr) > len(str) {
		return false
	}
	for i := 0; i <= len(str)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			if toLowerCase(str[i+j]) != toLowerCase(substr[j]) {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// toLowerCase converts a byte to lowercase
func toLowerCase(b byte) byte {
	if b >= 'A' && b <= 'Z' {
		return b + ('a' - 'A')
	}
	return b
}

// ConvertToConfigRepositories converts discovered repositories to config.Repository format
func (ds *DiscoveryService) ConvertToConfigRepositories(repos []*Repository, forgeManager *ForgeManager) []config.Repository {
	var configRepos []config.Repository

	for _, repo := range repos {
		// Find the forge config for this repository
		var auth *config.AuthConfig
		for forgeName, forgeConfig := range forgeManager.GetForgeConfigs() {
			if forgeName == repo.Metadata["forge_name"] ||
				forgeConfig.Name == repo.Metadata["forge_name"] {
				auth = forgeConfig.Auth
				break
			}
		}

		configRepo := repo.ToConfigRepository(auth)
		configRepos = append(configRepos, configRepo)
	}

	return configRepos
}
