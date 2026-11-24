package config

import "testing"

// Every part of DocBuilder that looks for the site has to look in the
// same place: the build, the daemon and the server that serves it.
func TestTheOutputDirectoryIsTheSameForEverybody(t *testing.T) {
	tests := []struct {
		name string
		base string
		dir  string
		want string
	}{
		{"base with a relative directory", "/data", "site", "/data/site"},
		{"base with an absolute directory", "/data", "/custom/site", "/custom/site"},
		{"no base with a relative directory", "", "site", "site"},
		{"no base with an absolute directory", "", "/var/site", "/var/site"},
		{"no base and no directory", "", "", DefaultSiteDir},
		{"base and no directory", "/data", "", "/data/site"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{}
			cfg.Output.BaseDirectory = tt.base
			cfg.Output.Directory = tt.dir

			if got := cfg.ResolveOutputDir(); got != tt.want {
				t.Fatalf("the output directory is %s, want %s", got, tt.want)
			}
		})
	}
}
