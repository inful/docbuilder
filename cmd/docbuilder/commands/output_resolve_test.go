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

	if got := ResolveOutputDir(nil, cfg); got != "/srv/site" {
		t.Fatalf("the command line publishes the site in %s, want /srv/site", got)
	}
}

// The flag of the command line wins over the configuration.
func TestTheOutputFlagWinsOverTheConfiguration(t *testing.T) {
	cfg := &config.Config{}
	cfg.Output.BaseDirectory = "/data"
	cfg.Output.Directory = "site"

	elsewhere := "/tmp/elsewhere"
	if got := ResolveOutputDir(&elsewhere, cfg); got != "/tmp/elsewhere" {
		t.Fatalf("the flag was ignored: %s", got)
	}
}

// The flag wins over the configuration, even when the operator typed
// the default value: they typed it.
func TestTheOutputFlagWinsEvenWithTheDefaultValue(t *testing.T) {
	cfg := &config.Config{}
	cfg.Output.Directory = "public"

	site := "./site"
	if got := ResolveOutputDir(&site, cfg); got != "./site" {
		t.Fatalf("the site is published in %q, want ./site", got)
	}
}
