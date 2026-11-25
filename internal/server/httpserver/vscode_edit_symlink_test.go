package httpserver

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// A directory of the docs that is a link to another place takes the
// file out of the docs directory: the check of the file alone does not
// see the link in the directories above it, and the preview server
// opened a file anywhere on the machine.
func TestTheEditPathRefusesALinkThatLeavesTheDocsDirectory(t *testing.T) {
	docsDir := t.TempDir()
	outside := t.TempDir()

	secret := filepath.Join(outside, "secret.md")
	if err := os.WriteFile(secret, []byte("# Secret"), 0o600); err != nil {
		t.Fatalf("write the file: %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(docsDir, "link")); err != nil {
		t.Fatalf("create the link: %v", err)
	}

	srv := &Server{cfg: &config.Config{
		Repositories: []config.Repository{{URL: docsDir}},
	}}

	_, err := srv.validateAndResolveEditPath("/_edit/link/secret.md")
	if err == nil {
		t.Fatal("the link out of the docs directory was accepted")
	}

	var editErr *editError
	if !errors.As(err, &editErr) {
		t.Fatalf("expected an edit error, got: %T", err)
	}
	if editErr.statusCode != http.StatusBadRequest {
		t.Fatalf("expected the status 400, got %d", editErr.statusCode)
	}
}
