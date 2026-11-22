package hugo

import (
	"os"
	"path/filepath"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// A failed promotion must not take the site away: the output directory is
// renamed to the backup before the promotion, so without a restore the
// daemon would serve nothing until the next build.
func TestFinalizeStagingRestoresTheOutputWhenPromotionFails(t *testing.T) {
	out := t.TempDir()
	outputDir := filepath.Join(out, "site")
	if err := os.MkdirAll(filepath.Join(outputDir, "public"), 0o750); err != nil {
		t.Fatalf("mkdir output: %v", err)
	}
	if err := os.WriteFile(filepath.Join(outputDir, "public", "index.html"), []byte("old"), 0o600); err != nil {
		t.Fatalf("write index: %v", err)
	}

	// The staging directory sits in a read-only directory, so the
	// promotion cannot succeed while the backup can.
	readOnly := filepath.Join(out, "ro")
	if err := os.MkdirAll(readOnly, 0o750); err != nil {
		t.Fatalf("mkdir ro: %v", err)
	}
	stageDir := filepath.Join(readOnly, "site_stage")
	if err := os.MkdirAll(stageDir, 0o750); err != nil {
		t.Fatalf("mkdir staging: %v", err)
	}
	//nolint:gosec // the test needs a directory that cannot be written to
	if err := os.Chmod(readOnly, 0o500); err != nil {
		t.Fatalf("chmod ro: %v", err)
	}
	//nolint:gosec // the test needs to make the directory writable again
	t.Cleanup(func() { _ = os.Chmod(readOnly, 0o700) })

	cfg := &config.Config{Output: config.OutputConfig{Directory: outputDir}}
	g := NewGenerator(cfg, outputDir)
	g.stageDir = stageDir

	if err := g.finalizeStaging(); err == nil {
		t.Fatal("expected the promotion to fail")
	}

	// The previous site is back where the server looks for it.
	restored := filepath.Join(outputDir, "public", "index.html")
	data, err := os.ReadFile(restored) //nolint:gosec // test path
	if err != nil {
		t.Fatalf("the previous output must be restored: %v", err)
	}
	if string(data) != "old" {
		t.Fatalf("unexpected content %q", data)
	}
}
