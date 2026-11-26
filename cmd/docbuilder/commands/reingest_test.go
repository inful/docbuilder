package commands

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// The command asks the daemon for a reingestion.
func TestTheReingestCommandAsksTheDaemon(t *testing.T) {
	var gotMethod, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"triggered","job_id":"manual-1"}`))
	}))
	defer srv.Close()

	cmd := &ReingestCmd{AdminURL: srv.URL}
	if err := cmd.Run(&Global{}, &CLI{Config: "config.yaml"}); err != nil {
		t.Fatalf("the reingestion: %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Fatalf("the daemon was asked with %s, want POST", gotMethod)
	}
	if gotPath != "/api/ragabast/reingest" {
		t.Fatalf("the daemon was asked at %s, want /api/ragabast/reingest", gotPath)
	}
}

// The address of the daemon comes from the configuration: the port and
// the bind address of the admin API.
func TestTheReingestCommandReadsTheAdminAddressFromTheConfiguration(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	content := `version: "2.0"
repositories:
  - name: repo
    url: https://example.com/repo.git
daemon:
  http:
    admin_port: 9999
`
	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write the config: %v", err)
	}

	cmd := &ReingestCmd{}
	url := cmd.resolveAdminURL(&CLI{Config: configPath})
	if url != "http://127.0.0.1:9999" {
		t.Fatalf("the address of the daemon is %s, want http://127.0.0.1:9999", url)
	}
}

// A daemon that listens on every interface is reached through the
// loopback.
func TestTheReingestCommandReachesADaemonOnEveryInterface(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	content := `version: "2.0"
repositories:
  - name: repo
    url: https://example.com/repo.git
daemon:
  http:
    admin_port: 9999
    admin_bind: "0.0.0.0"
`
	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write the config: %v", err)
	}

	cmd := &ReingestCmd{}
	url := cmd.resolveAdminURL(&CLI{Config: configPath})
	if url != "http://127.0.0.1:9999" {
		t.Fatalf("the address of the daemon is %s, want http://127.0.0.1:9999", url)
	}
}

// The command reads the environment variables of the .env file: the
// configuration holds ${VAR} references, and they name the variables of
// the .env files.
func TestTheReingestCommandReadsTheEnvironmentOfTheEnvFile(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	content := `version: "2.0"
repositories:
  - name: repo
    url: https://example.com/repo.git
daemon:
  http:
    admin_port: ${DOCBUILDER_TEST_ADMIN_PORT}
`
	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write the config: %v", err)
	}
	// The .env file sits next to the command: the command reads it
	// from the directory it runs in.
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("DOCBUILDER_TEST_ADMIN_PORT=9999\n"), 0o600); err != nil {
		t.Fatalf("write the .env file: %v", err)
	}
	t.Chdir(dir)

	cmd := &ReingestCmd{}
	url := cmd.resolveAdminURL(&CLI{Config: configPath})
	if url != "http://127.0.0.1:9999" {
		t.Fatalf("the address of the daemon is %s, want http://127.0.0.1:9999", url)
	}
}

// A configuration that is not there is not an error: the daemon that
// runs the site may be named by the flag or by the environment, and the
// default address is where a daemon listens when nothing says
// otherwise.
func TestTheReingestCommandFallsBackToTheDefaultAddress(t *testing.T) {
	cmd := &ReingestCmd{}
	url := cmd.resolveAdminURL(&CLI{Config: filepath.Join(t.TempDir(), "there-is-no-config.yaml")})
	if url != defaultAdminURL {
		t.Fatalf("the address of the daemon is %s, want %s", url, defaultAdminURL)
	}
}

// A configuration without a daemon block says nothing about where the
// daemon listens: the default address is used.
func TestTheReingestCommandFallsBackWhenTheConfigurationHasNoDaemon(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	content := `version: "2.0"
repositories:
  - name: repo
    url: https://example.com/repo.git
`
	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write the config: %v", err)
	}

	cmd := &ReingestCmd{}
	url := cmd.resolveAdminURL(&CLI{Config: configPath})
	if url != defaultAdminURL {
		t.Fatalf("the address of the daemon is %s, want %s", url, defaultAdminURL)
	}
}

// A daemon that refuses the reingestion is an error: the operator must
// know that the corpus was not sent again.
func TestTheReingestCommandFailsWhenTheDaemonRefuses(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("the ragabast ingest is not configured"))
	}))
	defer srv.Close()

	cmd := &ReingestCmd{AdminURL: srv.URL}
	if err := cmd.Run(&Global{}, &CLI{Config: "config.yaml"}); err == nil {
		t.Fatal("the command must fail when the daemon refuses the reingestion")
	}
}

// A daemon that is not there is an error, and the answer says where the
// command looked for it.
func TestTheReingestCommandFailsWhenTheDaemonIsNotThere(t *testing.T) {
	cmd := &ReingestCmd{AdminURL: "http://127.0.0.1:1"}
	err := cmd.Run(&Global{}, &CLI{Config: "config.yaml"})
	if err == nil {
		t.Fatal("the command must fail when the daemon does not answer")
	}
}
