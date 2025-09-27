package config

// OutputConfig represents output configuration
type OutputConfig struct {
	Directory string `yaml:"directory"`
	Clean     bool   `yaml:"clean"` // Clean output directory before build
}
