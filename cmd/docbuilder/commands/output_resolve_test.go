package commands

import (
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// The command line, the daemon and the server have to publish the site
// in the same place. A base directory with an absolute directory in the
// configuration published the site in two places: the command line
// wrote it into the base directory, the daemon and the server used the
// absolute path.
func TestTheCommandLineUsesTheSameOutputDirectoryAsTheDaemon(t *testing.T) {
	cfg := &config.Config{}
	cfg.Output.BaseDirectory = "/data"
	cfg.Output.Directory = "/srv/site"

	if got := ResolveOutputDir("", cfg); got != "/srv/site" {
		t.Fatalf("the command line publishes the site in %s, want /srv/site", got)
	}
}

// The flag of the command line wins over the configuration.
func TestTheOutputFlagWinsOverTheConfiguration(t *testing.T) {
	cfg := &config.Config{}
	cfg.Output.BaseDirectory = "/data"
	cfg.Output.Directory = "site"

	if got := ResolveOutputDir("/tmp/elsewhere", cfg); got != "/tmp/elsewhere" {
		t.Fatalf("the flag was ignored: %s", got)
	}
}
