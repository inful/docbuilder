package versioning

import (
	"time"
)

// VersionStrategy defines the versioning approach for repositories
type VersionStrategy string

const (
	// StrategyDefaultOnly includes only the default branch
	StrategyDefaultOnly VersionStrategy = "default_only"

	// StrategyBranches includes multiple branches
	StrategyBranches VersionStrategy = "branches"

	// StrategyTags includes tagged versions
	StrategyTags VersionStrategy = "tags"

	// StrategyBranchesAndTags includes both branches and tags
	StrategyBranchesAndTags VersionStrategy = "branches_and_tags"
)

// VersionConfig represents versioning configuration
type VersionConfig struct {
	Strategy          VersionStrategy `yaml:"strategy" json:"strategy"`
	DefaultBranchOnly bool            `yaml:"default_branch_only" json:"default_branch_only"`
	BranchPatterns    []string        `yaml:"branch_patterns" json:"branch_patterns"`
	TagPatterns       []string        `yaml:"tag_patterns" json:"tag_patterns"`
	MaxVersions       int             `yaml:"max_versions_per_repo" json:"max_versions_per_repo"`
}

// VersionType identifies the type of version (branch or tag)
type VersionType string

const (
	VersionTypeBranch VersionType = "branch"
	VersionTypeTag    VersionType = "tag"
)

// Version represents a single version of documentation
type Version struct {
	Name         string      `json:"name"`          // Branch name or tag name
	Type         VersionType `json:"type"`          // "branch" or "tag"
	DisplayName  string      `json:"display_name"`  // Human-readable version name
	IsDefault    bool        `json:"is_default"`    // Whether this is the default version
	Path         string      `json:"path"`          // Hugo content path (e.g., "v1.2.0", "latest")
	CommitSHA    string      `json:"commit_sha"`    // Git commit SHA
	CreatedAt    time.Time   `json:"created_at"`    // When version was created/tagged
	LastModified time.Time   `json:"last_modified"` // When docs were last updated
	DocsPath     string      `json:"docs_path"`     // Path to documentation in repo
}

// RepositoryVersions holds all versions for a single repository
type RepositoryVersions struct {
	RepositoryURL    string     `json:"repository_url"`
	DefaultBranch    string     `json:"default_branch"`
	Versions         []*Version `json:"versions"`
	LastDiscovery    time.Time  `json:"last_discovery"`
	MaxVersionsLimit int        `json:"max_versions_limit"`
}

// VersionDiscoveryResult holds the result of version discovery for a repository
type VersionDiscoveryResult struct {
	Repository   *RepositoryVersions `json:"repository"`
	NewCount     int                 `json:"new_count"`     // Number of new versions found
	UpdatedCount int                 `json:"updated_count"` // Number of versions updated
	RemovedCount int                 `json:"removed_count"` // Number of versions removed
	Errors       []string            `json:"errors,omitempty"`
}

// VersionManager interface defines version management operations
type VersionManager interface {
	// DiscoverVersions discovers available versions for a repository
	DiscoverVersions(repoURL string, config *VersionConfig) (*VersionDiscoveryResult, error)

	// GetRepositoryVersions returns all versions for a repository
	GetRepositoryVersions(repoURL string) (*RepositoryVersions, error)

	// GetDefaultVersion returns the default version for a repository
	GetDefaultVersion(repoURL string) (*Version, error)

	// UpdateVersions updates the versions for a repository based on discovery
	UpdateVersions(repoURL string, versions []*Version) error

	// CleanupOldVersions removes old versions based on retention policy
	CleanupOldVersions(repoURL string, config *VersionConfig) error

	// ListAllRepositories returns all repositories with versions
	ListAllRepositories() ([]*RepositoryVersions, error)
}

// VersionFilter represents criteria for filtering versions
type VersionFilter struct {
	IncludeTypes []VersionType  `json:"include_types,omitempty"` // Filter by version type
	NamePatterns []string       `json:"name_patterns,omitempty"` // Glob patterns for names
	ExcludeNames []string       `json:"exclude_names,omitempty"` // Specific names to exclude
	MaxAge       *time.Duration `json:"max_age,omitempty"`       // Maximum age for versions
	OnlyDefault  bool           `json:"only_default,omitempty"`  // Only include default version
}

// GitReference represents a Git branch or tag reference
type GitReference struct {
	Name      string    `json:"name"`       // Reference name (branch/tag)
	Type      string    `json:"type"`       // "branch" or "tag"
	CommitSHA string    `json:"commit_sha"` // Target commit SHA
	CreatedAt time.Time `json:"created_at"` // When ref was created
}
