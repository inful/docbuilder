package config

// OutputConfig represents output configuration.
type OutputConfig struct {
	BaseDirectory string `yaml:"base_directory"` // Optional base directory for output and staging, defaults to "."
	Directory     string `yaml:"directory"`      // Output directory (relative to base if base is set)
	Clean         bool   `yaml:"clean"`          // Clean output directory before build
}
