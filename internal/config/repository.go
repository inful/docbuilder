package config

// Repository represents a Git repository to process (shared between config and generator logic).
type Repository struct {
	URL         string            `yaml:"url"`
	Name        string            `yaml:"name"`
	Branch      string            `yaml:"branch,omitempty"`
	Description string            `yaml:"description,omitempty"`
	Auth        *AuthConfig       `yaml:"auth,omitempty"`
	Paths       []string          `yaml:"paths,omitempty"`   // Specific paths to docs, defaults applied elsewhere
	Tags        map[string]string `yaml:"tags,omitempty"`    // Additional metadata (forge discovery, etc.)
	Version     string            `yaml:"version,omitempty"` // Version label when expanded from versioning discovery

	// PinnedCommit optionally pins the repository to a specific commit SHA for this run.
	//
	// This is intentionally not part of the on-disk YAML config schema; it is injected
	// by orchestration flows (ADR-021 snapshot builds).
	PinnedCommit string `json:"pinned_commit,omitempty" yaml:"-"`

	IsVersioned bool `yaml:"-"` // Internal flag indicating this repo was created from version expansion
	IsTag       bool `yaml:"-"` // Internal flag indicating this is a tag reference (not a branch)
}
