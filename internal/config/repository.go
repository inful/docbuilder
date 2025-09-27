package config

// Repository represents a Git repository to process (shared between config and generator logic)
type Repository struct {
    URL    string            `yaml:"url"`
    Name   string            `yaml:"name"`
    Branch string            `yaml:"branch,omitempty"`
    Auth   *AuthConfig       `yaml:"auth,omitempty"`
    Paths  []string          `yaml:"paths,omitempty"` // Specific paths to docs, defaults applied elsewhere
    Tags   map[string]string `yaml:"tags,omitempty"`  // Additional metadata (forge discovery, etc.)
}
