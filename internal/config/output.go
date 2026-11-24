package config

import "path/filepath"

// DefaultSiteDir is where the site is published when nothing else says
// otherwise.
const DefaultSiteDir = "./site"

// OutputConfig represents output configuration.
type OutputConfig struct {
	BaseDirectory string `yaml:"base_directory"` // Optional base directory for output and staging, defaults to "."
	Directory     string `yaml:"directory"`      // Output directory (relative to base if base is set)

	// Clean is accepted for the configurations that set it. The build
	// replaces the output directory with the site it built, so the
	// output never holds a file of an older build: there is nothing
	// to clean.
	Clean bool `yaml:"clean"`
}

// ResolveOutputDir returns the directory the site is published in.
//
// The base directory holds the output and the staging directory. A
// directory that is absolute is a path of its own: the base directory
// is ignored for it. Every part of DocBuilder that looks for the site
// - the build, the daemon, the server that serves it - uses this one,
// or they look in different places.
func (c *Config) ResolveOutputDir() string {
	if c == nil {
		return DefaultSiteDir
	}
	dir := c.Output.Directory
	if dir == "" {
		dir = DefaultSiteDir
	}
	if c.Output.BaseDirectory != "" && !filepath.IsAbs(dir) {
		dir = filepath.Join(c.Output.BaseDirectory, dir)
	}
	return dir
}
