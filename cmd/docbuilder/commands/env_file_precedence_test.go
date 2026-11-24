package commands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/alecthomas/kong"
)

// The .env file is loaded before the flags are parsed: the variables
// in it name the configuration file and the log level, and kong reads
// them while it parses. Loaded afterwards, they had no effect.
func TestTheEnvFileIsLoadedBeforeTheFlagsAreParsed(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, ".env")
	if err := os.WriteFile(envFile, []byte("DOCBUILDER_CONFIG=other.yaml\n"), 0o600); err != nil {
		t.Fatalf("write the .env file: %v", err)
	}

	// The working directory of the test is the directory of the .env.
	t.Chdir(dir)

	// The variable must not exist: the .env file never overwrites a
	// variable that exists.
	if err := os.Unsetenv("DOCBUILDER_CONFIG"); err != nil {
		t.Fatalf("unset the variable: %v", err)
	}
	if err := LoadEnvFile(); err != nil {
		t.Fatalf("load the .env file: %v", err)
	}

	if got := os.Getenv("DOCBUILDER_CONFIG"); got != "other.yaml" {
		t.Fatalf("the configuration file is %q, want other.yaml", got)
	}
}

// The configuration file comes from the environment: kong reads the
// variable while it parses the flags, so the .env file has to be
// loaded before that.
func TestTheConfigurationFileComesFromTheEnvironment(t *testing.T) {
	t.Setenv("DOCBUILDER_CONFIG", "other.yaml")

	cli := &CLI{}
	parser, err := kong.New(cli)
	if err != nil {
		t.Fatalf("the parser: %v", err)
	}
	if _, err := parser.Parse([]string{"lint"}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if cli.Config != "other.yaml" {
		t.Fatalf("the configuration file is %q, want other.yaml", cli.Config)
	}
}
