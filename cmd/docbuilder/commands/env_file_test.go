package commands

import (
	"os"
	"testing"
)

// Both files are loaded: the local one wins over the shared one, and the
// environment of the process wins over both.
func TestLoadEnvFileLoadsBothFiles(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	if err := os.WriteFile(".env", []byte("DOCBUILDER_TEST_TOKEN=shared\nDOCBUILDER_TEST_SHARED=shared\n"), 0o600); err != nil {
		t.Fatalf("write .env: %v", err)
	}
	if err := os.WriteFile(".env.local", []byte("DOCBUILDER_TEST_TOKEN=local\n"), 0o600); err != nil {
		t.Fatalf("write .env.local: %v", err)
	}

	if err := LoadEnvFile(); err != nil {
		t.Fatalf("load the env files: %v", err)
	}

	if got := os.Getenv("DOCBUILDER_TEST_TOKEN"); got != "local" {
		t.Errorf("the token is %q, want the one of the local file", got)
	}
	if got := os.Getenv("DOCBUILDER_TEST_SHARED"); got != "shared" {
		t.Errorf("the shared value is %q, want the one of the .env file", got)
	}
}

// The environment of the process wins over the files.
func TestLoadEnvFileKeepsTheEnvironmentOfTheProcess(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	t.Setenv("DOCBUILDER_TEST_TOKEN", "from-the-process")
	if err := os.WriteFile(".env", []byte("DOCBUILDER_TEST_TOKEN=shared\n"), 0o600); err != nil {
		t.Fatalf("write .env: %v", err)
	}

	if err := LoadEnvFile(); err != nil {
		t.Fatalf("load the env files: %v", err)
	}

	if got := os.Getenv("DOCBUILDER_TEST_TOKEN"); got != "from-the-process" {
		t.Errorf("the token is %q, want the one of the process", got)
	}
}

// Without a file the command reports it, but does not fail.
func TestLoadEnvFileWithoutAFile(t *testing.T) {
	t.Chdir(t.TempDir())

	if err := LoadEnvFile(); err == nil {
		t.Fatal("expected an error when there is no .env file")
	}
}
