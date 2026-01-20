package forge

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/foundation/errors"
)

// DiscoveryService handles repository discovery across multiple forges.
type DiscoveryService struct {
	forgeManager *Manager
	filtering    *config.FilteringConfig
}

type repoFilterDecision struct {
	include bool
	reason  string // stable reason code
	detail  string // optional detail (e.g. matched pattern)
}

// NewDiscoveryService creates a new discovery service.
func NewDiscoveryService(manager *Manager, filtering *config.FilteringConfig) *DiscoveryService {
	return &DiscoveryService{
		forgeManager: manager,
		filtering:    filtering,
	}
}

// DiscoveryResult contains the results of a discovery operation.
type DiscoveryResult struct {
	Repositories  []*Repository              `json:"repositories"`
	Organizations map[string][]*Organization `json:"organizations"` // Keyed by forge name
	Filtered      []*Repository              `json:"filtered"`      // Repositories that were filtered out
	Errors        map[string]error           `json:"errors"`        // Errors by forge name
	Timestamp     time.Time                  `json:"timestamp"`
	Duration      time.Duration              `json:"duration"`
}

// DiscoverAll discovers repositories from all configured forges.
func (ds *DiscoveryService) DiscoverAll(ctx context.Context) (*DiscoveryResult, error) {
	startTime := time.Now()
	result := &DiscoveryResult{
		Repositories:  make([]*Repository, 0),
		Organizations: make(map[string][]*Organization),
		Filtered:      make([]*Repository, 0),
		Errors:        make(map[string]error),
		Timestamp:     startTime,
	}

	forges := ds.forgeManager.GetAllForges()
	if len(forges) == 0 {
		result.Duration = time.Since(startTime)
		return result, nil
	}

	// Discover each forge concurrently. This can significantly reduce end-to-end time
	// when multiple forges are configured or when a single forge has high-latency APIs.
	const maxForgeConcurrency = 4
	sem := make(chan struct{}, maxForgeConcurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for forgeName, client := range forges {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			slog.Info("Starting discovery", "forge", forgeName)

			// Discover repositories for this forge
			repos, orgs, filtered, err := ds.discoverForge(ctx, client)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				result.Errors[forgeName] = err
				slog.Error("Discovery failed", "forge", forgeName, "error", err)
				return
			}

			result.Repositories = append(result.Repositories, repos...)
			result.Organizations[forgeName] = orgs
			result.Filtered = append(result.Filtered, filtered...)

			slog.Info("Discovery completed",
				"forge", forgeName,
				"repositories", len(repos),
				"organizations", len(orgs),
				"filtered", len(filtered))
		}()
	}

	wg.Wait()

	result.Duration = time.Since(startTime)

	slog.Info("Discovery summary",
		"total_repositories", len(result.Repositories),
		"total_filtered", len(result.Filtered),
		"duration", result.Duration,
		"errors", len(result.Errors))

	return result, nil
}

// discoverForge discovers repositories from a single forge.
func (ds *DiscoveryService) discoverForge(ctx context.Context, client Client) ([]*Repository, []*Organization, []*Repository, error) {
	forgeConfig := ds.forgeManager.GetForgeConfigs()[client.GetName()]
	if forgeConfig == nil {
		return nil, nil, nil, errors.ConfigError("forge configuration not found").
			WithContext("name", client.GetName()).
			Build()
	}

	// Determine which organizations/groups to scan.
	// If none are configured, enter auto-discovery mode and enumerate all accessible organizations.
	var (
		targetOrgs       []string
		organizations    []*Organization
		hasPrelistedOrgs bool
		organizationsErr error
		repositories     []*Repository
		repositoriesErr  error
	)

	targetOrgs = append(targetOrgs, forgeConfig.Organizations...)
	targetOrgs = append(targetOrgs, forgeConfig.Groups...)

	if len(targetOrgs) == 0 {
		slog.Info("Entering auto-discovery mode (no organizations/groups configured)", "forge", client.GetName())
		orgs, err := client.ListOrganizations(ctx)
		if err != nil {
			return nil, nil, nil, errors.ForgeError("failed to list organizations during auto-discovery").
				WithCause(err).
				WithContext("forge", client.GetName()).
				Build()
		}
		organizations = orgs
		hasPrelistedOrgs = true
		for _, org := range orgs {
			targetOrgs = append(targetOrgs, org.Name)
		}
		slog.Info("Auto-discovered organizations", "forge", client.GetName(), "count", len(orgs))
	}

	// Fetch org metadata and repositories concurrently where possible.
	// If we already listed orgs for auto-discovery, reuse that result.
	var fetchWG sync.WaitGroup
	if !hasPrelistedOrgs {
		fetchWG.Add(1)
		go func() {
			defer fetchWG.Done()
			organizations, organizationsErr = client.ListOrganizations(ctx)
		}()
	}

	fetchWG.Add(1)
	go func() {
		defer fetchWG.Done()
		repositories, repositoriesErr = client.ListRepositories(ctx, targetOrgs)
	}()

	fetchWG.Wait()

	if organizationsErr != nil {
		slog.Warn("Failed to get organization metadata", "forge", client.GetName(), "error", organizationsErr)
		organizations = make([]*Organization, 0)
	}
	if repositoriesErr != nil {
		return nil, organizations, nil, errors.ForgeError("failed to list repositories for forge").
			WithCause(repositoriesErr).
			WithContext("forge", client.GetName()).
			Build()
	}

	// Ensure repository metadata includes forge identity for downstream conversion (auth, namespacing, edit links).
	forgeName := client.GetName()
	forgeType := strings.ToLower(string(client.GetType()))
	for _, repo := range repositories {
		if repo.Metadata == nil {
			repo.Metadata = make(map[string]string)
		}
		if repo.Metadata["forge_name"] == "" {
			repo.Metadata["forge_name"] = forgeName
		}
		if repo.Metadata["forge_type"] == "" {
			repo.Metadata["forge_type"] = forgeType
		}
	}

	// Check documentation status and apply filtering
	originalCount := len(repositories)
	var validRepos []*Repository
	var filteredRepos []*Repository

	// Check documentation status concurrently (max 20 at a time)
	const maxConcurrency = 20
	sem := make(chan struct{}, maxConcurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex
	checkedCount := 0

	for _, repo := range repositories {
		wg.Add(1)
		go func(r *Repository) {
			defer wg.Done()
			sem <- struct{}{}        // Acquire semaphore
			defer func() { <-sem }() // Release semaphore

			// Check if repository has documentation
			if err := client.CheckDocumentation(ctx, r); err != nil {
				slog.Warn("Failed to check documentation status",
					"forge", client.GetName(),
					"repository", r.FullName,
					"error", err)
				// Continue processing, but assume no docs
				r.HasDocs = false
				r.HasDocIgnore = false
			}

			// Log first few checks for debugging
			mu.Lock()
			checkedCount++
			if checkedCount <= 5 {
				slog.Info("Documentation check sample",
					"forge", client.GetName(),
					"repository", r.FullName,
					"has_docs", r.HasDocs,
					"has_docignore", r.HasDocIgnore,
					"default_branch", r.DefaultBranch,
					"project_id", r.ID)
			}
			mu.Unlock()

			// Apply filtering logic with mutex protection
			mu.Lock()
			defer mu.Unlock()
			decision := ds.filterDecision(r)
			if decision.include {
				validRepos = append(validRepos, r)
			} else {
				filteredRepos = append(filteredRepos, r)
				attrs := []any{
					"forge", client.GetName(),
					"repository", r.FullName,
					"reason", decision.reason,
				}
				if decision.detail != "" {
					attrs = append(attrs, "detail", decision.detail)
				}
				slog.Info("Repository filtered", attrs...)
			}
		}(repo)
	}

	// Wait for all checks to complete
	wg.Wait()

	// Log statistics about documentation checks
	docsFound := 0
	docsIgnored := 0
	for _, repo := range repositories {
		if repo.HasDocs {
			docsFound++
		}
		if repo.HasDocIgnore {
			docsIgnored++
		}
	}
	slog.Info("Documentation check completed",
		"forge", client.GetName(),
		"total_repos", originalCount,
		"repos_with_docs", docsFound,
		"repos_with_docignore", docsIgnored)

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

// shouldIncludeRepository determines if a repository should be included based on filtering config.

func (ds *DiscoveryService) filterDecision(repo *Repository) repoFilterDecision {
	// Skip archived repositories
	if repo.Archived {
		return repoFilterDecision{include: false, reason: "archived"}
	}

	// Check for .docignore file
	if repo.HasDocIgnore {
		return repoFilterDecision{include: false, reason: "docignore_present"}
	}

	// Check if repository has required paths (e.g., docs folder)
	if !repo.HasDocs && len(ds.filtering.RequiredPaths) > 0 {
		return repoFilterDecision{include: false, reason: "missing_required_paths"}
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
			return repoFilterDecision{include: false, reason: "include_patterns_miss"}
		}
	}

	// Check exclude patterns
	for _, pattern := range ds.filtering.ExcludePatterns {
		if matchesPattern(repo.Name, pattern) || matchesPattern(repo.FullName, pattern) {
			return repoFilterDecision{include: false, reason: "exclude_patterns_match", detail: pattern}
		}
	}

	return repoFilterDecision{include: true, reason: "included"}
}

// matchesPattern checks if a string matches a simple glob pattern
// This is a basic implementation - could be enhanced with proper glob matching.
func matchesPattern(str, pattern string) bool {
	// Simple wildcard matching
	if pattern == "*" {
		return true
	}

	// Exact match
	if pattern == str {
		return true
	}

	// Contains match with *pattern* (must be checked before suffix match!)
	if len(pattern) > 2 && pattern[0] == '*' && pattern[len(pattern)-1] == '*' {
		substring := pattern[1 : len(pattern)-1]
		return contains(str, substring)
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

	return false
}

// contains checks if a string contains a substring (case-insensitive).
func contains(str, substr string) bool {
	if len(substr) > len(str) {
		return false
	}
	for i := 0; i <= len(str)-len(substr); i++ {
		match := true
		for j := range len(substr) {
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

// toLowerCase converts a byte to lowercase.
func toLowerCase(b byte) byte {
	if b >= 'A' && b <= 'Z' {
		return b + ('a' - 'A')
	}
	return b
}

// ConvertToConfigRepositories converts discovered repositories to config.Repository format.
func (ds *DiscoveryService) ConvertToConfigRepositories(repos []*Repository, forgeManager *Manager) []config.Repository {
	configRepos := make([]config.Repository, 0, len(repos))

	for _, repo := range repos {
		// Find the forge config for this repository
		var auth *config.AuthConfig
		forgeNameMeta := repo.Metadata["forge_name"]
		for forgeName, forgeConfig := range forgeManager.GetForgeConfigs() {
			if forgeName == forgeNameMeta || forgeConfig.Name == forgeNameMeta {
				auth = forgeConfig.Auth
				break
			}
		}

		// Fallback: if forge_name metadata is missing, try matching by BaseURL.
		if auth == nil {
			for _, forgeConfig := range forgeManager.GetForgeConfigs() {
				base := strings.TrimRight(forgeConfig.BaseURL, "/")
				if base == "" {
					continue
				}
				if strings.HasPrefix(repo.CloneURL, base+"/") || repo.CloneURL == base {
					auth = forgeConfig.Auth
					break
				}
			}
		}

		configRepo := repo.ToConfigRepository(auth)
		configRepos = append(configRepos, configRepo)
	}

	return configRepos
}
