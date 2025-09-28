package hugo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// TestPathHelpers_Contract validates buildRoot/finalRoot behavior across staging lifecycle.
func TestPathHelpers_Contract(t *testing.T) {
	base := t.TempDir()
	out := filepath.Join(base, "site")
	cfg := &config.Config{Hugo: config.HugoConfig{Theme: "hextra"}}
	gen := NewGenerator(cfg, out)

	// Initial (no staging)
	if gen.buildRoot() != out {
		t.Fatalf("expected buildRoot initial=%s got %s", out, gen.buildRoot())
	}
	if gen.finalRoot() != out {
		t.Fatalf("expected finalRoot=%s got %s", out, gen.finalRoot())
	}

	// Begin staging
	if err := gen.beginStaging(); err != nil {
		t.Fatalf("beginStaging failed: %v", err)
	}
	stage1 := gen.stageDir
	if stage1 == "" {
		t.Fatalf("stageDir empty after beginStaging")
	}
	if !strings.Contains(stage1, ".staging-") {
		t.Fatalf("stageDir does not contain .staging- marker: %s", stage1)
	}
	if gen.buildRoot() != stage1 {
		t.Fatalf("buildRoot should point to staging dir")
	}
	if gen.finalRoot() != out {
		t.Fatalf("finalRoot changed unexpectedly")
	}
	if _, err := os.Stat(stage1); err != nil {
		t.Fatalf("staging dir missing on disk: %v", err)
	}

	// Abort staging
	gen.abortStaging()
	if gen.stageDir != "" {
		t.Fatalf("stageDir not cleared after abort")
	}
	if gen.buildRoot() != out {
		t.Fatalf("buildRoot should revert to outputDir after abort")
	}
	if _, err := os.Stat(stage1); !os.IsNotExist(err) {
		t.Fatalf("staging dir still exists after abort")
	}

	// Begin again and finalize
	if err := gen.beginStaging(); err != nil {
		t.Fatalf("second beginStaging failed: %v", err)
	}
	stage2 := gen.stageDir
	if stage2 == "" || stage2 == stage1 {
		t.Fatalf("expected new staging dir, got %s (old %s)", stage2, stage1)
	}
	if gen.buildRoot() != stage2 {
		t.Fatalf("buildRoot should equal new staging dir before finalize")
	}
	if err := gen.finalizeStaging(); err != nil {
		t.Fatalf("finalizeStaging failed: %v", err)
	}
	if gen.stageDir != "" {
		t.Fatalf("stageDir not cleared after finalize")
	}
	if gen.buildRoot() != out || gen.finalRoot() != out {
		t.Fatalf("roots not pointing to final output after finalize")
	}
	if _, err := os.Stat(out); err != nil {
		t.Fatalf("final output dir missing after finalize: %v", err)
	}
	if _, err := os.Stat(stage2); !os.IsNotExist(err) {
		t.Fatalf("old staging dir still present after finalize")
	}
}
