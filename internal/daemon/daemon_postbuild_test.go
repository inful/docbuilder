package daemon

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolvePublicDirForVerification_Primary(t *testing.T) {
	base := t.TempDir()
	out := filepath.Join(base, "site")
	primary := filepath.Join(out, "public")
	if err := os.MkdirAll(primary, 0o750); err != nil {
		t.Fatalf("mkdir primary: %v", err)
	}

	got, ok := resolvePublicDirForVerification(out)
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if got != primary {
		t.Fatalf("expected %q got %q", primary, got)
	}
}

func TestResolvePublicDirForVerification_BackupPrev(t *testing.T) {
	base := t.TempDir()
	out := filepath.Join(base, "site")
	backupPublic := filepath.Join(out+".prev", "public")
	if err := os.MkdirAll(backupPublic, 0o750); err != nil {
		t.Fatalf("mkdir backup public: %v", err)
	}

	got, ok := resolvePublicDirForVerification(out)
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if got != backupPublic {
		t.Fatalf("expected %q got %q", backupPublic, got)
	}
}

func TestResolvePublicDirForVerification_Missing(t *testing.T) {
	base := t.TempDir()
	out := filepath.Join(base, "site")

	got, ok := resolvePublicDirForVerification(out)
	if ok {
		t.Fatalf("expected ok=false, got true with %q", got)
	}
	if got != "" {
		t.Fatalf("expected empty path, got %q", got)
	}
}
