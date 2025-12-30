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
	IsVersioned bool              `yaml:"-"`                 // Internal flag indicating this repo was created from version expansion
	IsTag       bool              `yaml:"-"`                 // Internal flag indicating this is a tag reference (not a branch)
}
