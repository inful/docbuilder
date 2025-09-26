package hugo

import (
	"context"
	"strings"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// TestGenerationCancelledEarly ensures that a canceled context aborts before running stages.
func TestGenerationCancelledEarly(t *testing.T) {
	cfg := &config.V2Config{}
	gen := NewGenerator(cfg, t.TempDir())

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := gen.GenerateSiteWithReportContext(ctx, nil)
	if err == nil {
		t.Fatalf("expected cancellation error, got nil")
	}
	if !strings.Contains(err.Error(), "canceled") {
		t.Fatalf("expected error to mention canceled, got %v", err)
	}
}
