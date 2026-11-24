package config

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The Docker images are built by GoReleaser with their own
// configuration. The flags of that configuration named a variable
// that does not exist, so the image reported the version "unknown".
func TestTheDockerImageCarriesTheVersion(t *testing.T) {
	data, err := os.ReadFile("../../.goreleaser.docker.yaml")
	if err != nil {
		t.Fatalf("read the configuration: %v", err)
	}
	content := string(data)

	if strings.Contains(content, "-X main.version=") {
		t.Fatal("the Docker configuration injects the version into a variable that does not exist")
	}
	if !strings.Contains(content, "internal/version.Version={{.Version}}") {
		t.Fatal("the Docker configuration does not inject the version")
	}
}

// The flags of the release configuration name the variables of the
// version package.
func TestTheReleaseConfigurationCarriesTheVersion(t *testing.T) {
	data, err := os.ReadFile("../../.goreleaser.yaml")
	if err != nil {
		t.Fatalf("read the configuration: %v", err)
	}
	if !strings.Contains(string(data), "internal/version.Version={{.Version}}") {
		t.Fatal("the release configuration does not inject the version")
	}
}

// The version of the binary is the one of the version package.
func TestTheVersionOfTheBinary(t *testing.T) {
	if testing.Short() {
		t.Skip("the test builds the binary")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("no go toolchain")
	}
	dir := t.TempDir()
	binary := filepath.Join(dir, "docbuilder")
	cmd := exec.Command("go", "build",
		"-ldflags", "-X git.home.luguber.info/inful/docbuilder/internal/version.Version=9.9.9",
		"-o", binary, "./cmd/docbuilder")
	cmd.Dir = "../.."
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}

	out, err := exec.Command(binary, "--version").CombinedOutput()
	if err != nil {
		t.Fatalf("version: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "9.9.9") {
		t.Fatalf("the binary reports %q, want the injected version", strings.TrimSpace(string(out)))
	}
}
