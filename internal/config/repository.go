package config

// Repository represents a Git repository to process (shared between config and generator logic).
type Repository struct {
	URL         string            `yaml:"url"`
	Name        string            `yaml:"name"`
	DisplayName string            `yaml:"display_name,omitempty"` // Human-readable label used in category menus; falls back to Name.
	Branch      string            `yaml:"branch,omitempty"`
	Description string            `yaml:"description,omitempty"`
	Auth        *AuthConfig       `yaml:"auth,omitempty"`
	Paths       []string          `yaml:"paths,omitempty"`     // Specific paths to docs, defaults applied elsewhere
	Tags        map[string]string `yaml:"tags,omitempty"`      // Additional metadata (forge discovery, etc.)
	Version     string            `yaml:"version,omitempty"`   // Version label when expanded from versioning discovery
	Namespace   string            `yaml:"namespace,omitempty"` // Alias for forge_type (stable path resolution)
	Group       string            `yaml:"group,omitempty"`     // GitLab/GitHub group for collision resolution

	// PinnedCommit optionally pins the repository to a specific commit SHA for this run.
	//
	// This is intentionally not part of the on-disk YAML config schema; it is injected
	// by orchestration flows (ADR-021 snapshot builds).
	PinnedCommit string `json:"pinned_commit,omitempty" yaml:"-"`

	IsVersioned bool `yaml:"-"` // Internal flag indicating this repo was created from version expansion
	IsTag       bool `yaml:"-"` // Internal flag indicating this is a tag reference (not a branch)
}

// Label returns the human-readable label for this repository: DisplayName
// when set, otherwise Name. Callers in user-facing surfaces (sidebar menus,
// section indexes) should prefer Label over Name.
func (r *Repository) Label() string {
	if r.DisplayName != "" {
		return r.DisplayName
	}
	return r.Name
}
