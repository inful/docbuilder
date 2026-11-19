package commands

import "testing"

// The admin API has no password and can start builds, so the preview
// server keeps it on the loopback interface - the same as the daemon.
func TestPreviewHTTPConfig_KeepsTheAdminAPIOnLoopback(t *testing.T) {
	cfg := previewHTTPConfig(1316, 1319)

	if cfg.AdminBind != "127.0.0.1" {
		t.Errorf("AdminBind = %q, want %q", cfg.AdminBind, "127.0.0.1")
	}
	if cfg.DocsPort != 1316 {
		t.Errorf("DocsPort = %d, want %d", cfg.DocsPort, 1316)
	}
	if cfg.WebhookPort != 1317 {
		t.Errorf("WebhookPort = %d, want %d", cfg.WebhookPort, 1317)
	}
	if cfg.AdminPort != 1318 {
		t.Errorf("AdminPort = %d, want %d", cfg.AdminPort, 1318)
	}
	if cfg.LiveReloadPort != 1319 {
		t.Errorf("LiveReloadPort = %d, want %d", cfg.LiveReloadPort, 1319)
	}
}
