package forge

import (
	"fmt"
	"strconv"
	"time"
)

// Repository Factory Functions
// These functions create pre-configured repository instances for testing

// CreateMockGitHubRepo creates a GitHub repository with realistic defaults.
func CreateMockGitHubRepo(owner, name string, hasDocs, isPrivate, isArchived, isFork bool) *Repository {
	return &Repository{
		ID:            fmt.Sprintf("github-%s-%s", owner, name),
		Name:          name,
		FullName:      fmt.Sprintf("%s/%s", owner, name),
		CloneURL:      fmt.Sprintf("https://github.com/%s/%s.git", owner, name),
		SSHURL:        fmt.Sprintf("git@github.com:%s/%s.git", owner, name),
		DefaultBranch: "main",
		Description:   fmt.Sprintf("Mock GitHub repository: %s", name),
		Private:       isPrivate,
		Archived:      isArchived,
		HasDocs:       hasDocs,
		HasDocIgnore:  false,
		LastUpdated:   time.Now().Add(-time.Hour * 24),
		Topics:        []string{"github", documentationToken, "mock"},
		Language:      languageMarkdown,
		Metadata: map[string]string{
			forgeTypeKey: "github",
			"owner":      owner,
			"fork":       strconv.FormatBool(isFork),
		},
	}
}

// CreateMockGitLabRepo creates a GitLab repository with realistic defaults.
func CreateMockGitLabRepo(group, name string, hasDocs, isPrivate, isArchived, isFork bool) *Repository {
	return &Repository{
		ID:            fmt.Sprintf("gitlab-%s-%s", group, name),
		Name:          name,
		FullName:      fmt.Sprintf("%s/%s", group, name),
		CloneURL:      fmt.Sprintf("https://gitlab.com/%s/%s.git", group, name),
		SSHURL:        fmt.Sprintf("git@gitlab.com:%s/%s.git", group, name),
		DefaultBranch: "main",
		Description:   fmt.Sprintf("Mock GitLab repository: %s", name),
		Private:       isPrivate,
		Archived:      isArchived,
		HasDocs:       hasDocs,
		HasDocIgnore:  false,
		LastUpdated:   time.Now().Add(-time.Hour * 48),
		Topics:        []string{"gitlab", documentationToken, "mock"},
		Language:      languageMarkdown,
		Metadata: map[string]string{
			forgeTypeKey: "gitlab",
			"group":      group,
			"fork":       strconv.FormatBool(isFork),
		},
	}
}

// CreateMockForgejoRepo creates a Forgejo repository with realistic defaults.
func CreateMockForgejoRepo(org, name string, hasDocs, isPrivate, isArchived, isFork bool) *Repository {
	return &Repository{
		ID:            fmt.Sprintf("forgejo-%s-%s", org, name),
		Name:          name,
		FullName:      fmt.Sprintf("%s/%s", org, name),
		CloneURL:      fmt.Sprintf("https://git.example.com/%s/%s.git", org, name),
		SSHURL:        fmt.Sprintf("git@git.example.com:%s/%s.git", org, name),
		DefaultBranch: "main",
		Description:   fmt.Sprintf("Mock Forgejo repository: %s", name),
		Private:       isPrivate,
		Archived:      isArchived,
		HasDocs:       hasDocs,
		HasDocIgnore:  false,
		LastUpdated:   time.Now().Add(-time.Hour * 12),
		Topics:        []string{"forgejo", "documentation", "mock"},
		Language:      languageMarkdown,
		Metadata: map[string]string{
			"forge_type": "forgejo",
			"org":        org,
			"fork":       strconv.FormatBool(isFork),
		},
	}
}

// Organization Factory Functions

// CreateMockGitHubOrg creates a GitHub organization with realistic defaults.
func CreateMockGitHubOrg(name string) *Organization {
	return &Organization{
		ID:          fmt.Sprintf("github-org-%s", name),
		Name:        name,
		DisplayName: fmt.Sprintf("%s Organization", name),
		Description: fmt.Sprintf("Mock GitHub organization: %s", name),
		Type:        "organization",
		Metadata: map[string]string{
			"forge_type": "github",
			"mock":       "true",
		},
	}
}

// CreateMockGitLabGroup creates a GitLab group with realistic defaults.
func CreateMockGitLabGroup(name string) *Organization {
	return &Organization{
		ID:          fmt.Sprintf("gitlab-group-%s", name),
		Name:        name,
		DisplayName: fmt.Sprintf("%s Group", name),
		Description: fmt.Sprintf("Mock GitLab group: %s", name),
		Type:        "group",
		Metadata: map[string]string{
			"forge_type": "gitlab",
			"mock":       "true",
		},
	}
}

// CreateMockForgejoOrg creates a Forgejo organization with realistic defaults.
func CreateMockForgejoOrg(name string) *Organization {
	return &Organization{
		ID:          fmt.Sprintf("forgejo-org-%s", name),
		Name:        name,
		DisplayName: fmt.Sprintf("%s Organization", name),
		Description: fmt.Sprintf("Mock Forgejo organization: %s", name),
		Type:        "organization",
		Metadata: map[string]string{
			"forge_type": "forgejo",
			"mock":       "true",
		},
	}
}

// Bulk Repository Creation Functions

// CreateMockRepositorySet creates a set of repositories for testing.
func CreateMockRepositorySet(forgeType Type, orgName string, count int) []*Repository {
	repos := make([]*Repository, count)

	for i := range count {
		repoName := fmt.Sprintf("repo-%d", i)
		hasDocs := i%2 == 0   // Half have docs
		isPrivate := i%5 == 0 // Every 5th is private

		switch forgeType {
		case TypeGitHub:
			repos[i] = CreateMockGitHubRepo(orgName, repoName, hasDocs, isPrivate, false, false)
		case TypeGitLab:
			repos[i] = CreateMockGitLabRepo(orgName, repoName, hasDocs, isPrivate, false, false)
		case TypeForgejo:
			repos[i] = CreateMockForgejoRepo(orgName, repoName, hasDocs, isPrivate, false, false)
		default:
			// Generic repository
			repos[i] = &Repository{
				ID:            fmt.Sprintf("generic-%s-%s", orgName, repoName),
				Name:          repoName,
				FullName:      fmt.Sprintf("%s/%s", orgName, repoName),
				CloneURL:      fmt.Sprintf("https://forge.example.com/%s/%s.git", orgName, repoName),
				DefaultBranch: "main",
				HasDocs:       hasDocs,
				Private:       isPrivate,
				Topics:        []string{"documentation", "mock"},
				Language:      languageMarkdown,
			}
		}
	}

	return repos
}

// Enhanced Mock Builder Pattern moved to enhanced_mock.go to break the
// call-graph cycle between this file and enhanced_mock.go. The factory
// funcs in this file are leaves (no calls into EnhancedMockForgeClient),
// and the builder + CreateRealistic*Mock now live next to the type they
// construct.
