package daemon

import (
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// TestRagabastConfigOrNil_AbsentReturnsNil verifies the helper returns nil
// when no outbound config is present (the default for every existing
// deployment). This is the load-bearing safety check: any nil-pointer
// regression here would panic on the daemon init path.
func TestRagabastConfigOrNil_AbsentReturnsNil(t *testing.T) {
	if got := ragabastConfigOrNil(&config.Config{}); got != nil {
		t.Errorf("absent = %v, want nil", got)
	}
}

// TestRagabastConfigOrNil_DisabledReturnsNil verifies Enabled: false is
// treated identically to absent — opt-in is strict.
func TestRagabastConfigOrNil_DisabledReturnsNil(t *testing.T) {
	cfg := &config.Config{
		Daemon: &config.DaemonConfig{
			Outbound: &config.OutboundConfig{
				Ragabast: &config.RagabastConfig{
					Enabled:   false,
					IngestURL: "https://ragabast/api/ingest/async",
				},
			},
		},
	}
	if got := ragabastConfigOrNil(cfg); got != nil {
		t.Errorf("disabled = %v, want nil", got)
	}
}

// TestRagabastConfigOrNil_EnabledReturnsConfig verifies the happy path: the
// section is present, enabled, and returns the config for construction.
func TestRagabastConfigOrNil_EnabledReturnsConfig(t *testing.T) {
	want := &config.RagabastConfig{
		Enabled:   true,
		IngestURL: "https://ragabast/api/ingest/async",
	}
	cfg := &config.Config{
		Daemon: &config.DaemonConfig{
			Outbound: &config.OutboundConfig{
				Ragabast: want,
			},
		},
	}
	if got := ragabastConfigOrNil(cfg); got != want {
		t.Errorf("enabled = %v, want %v", got, want)
	}
}

// TestRagabastConfigOrNil_NilDaemonSafe verifies the helper doesn't panic on
// edge inputs (nil cfg, nil Daemon, nil Outbound).
func TestRagabastConfigOrNil_NilDaemonSafe(t *testing.T) {
	cases := []struct {
		name string
		cfg  *config.Config
	}{
		{"nil cfg", nil},
		{"nil Daemon", &config.Config{}},
		{"nil Outbound", &config.Config{Daemon: &config.DaemonConfig{}}},
		{"nil Ragabast", &config.Config{Daemon: &config.DaemonConfig{Outbound: &config.OutboundConfig{}}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ragabastConfigOrNil(tc.cfg); got != nil {
				t.Errorf("%s: got %v, want nil", tc.name, got)
			}
		})
	}
}

// TestNewOutboundDispatcher_AppliesRagabastConfig verifies the wiring code
// can build a working dispatcher from a config.RagabastConfig. We use a
// placeholder URL here — full HTTP behavior is covered in
// outbound_dispatcher_test.go.
func TestNewOutboundDispatcher_AppliesRagabastConfig(t *testing.T) {
	d, err := NewOutboundDispatcher(DispatcherConfig{
		IngestURL: "http://127.0.0.1:1/api/ingest/async",
		Workers:   2,
		QueueSize: 8,
	})
	if err != nil {
		t.Fatalf("NewOutboundDispatcher: %v", err)
	}
	if d == nil {
		t.Fatal("dispatcher is nil")
	}
	if d.workers != 2 {
		t.Errorf("workers = %d, want 2", d.workers)
	}
	if d.queueCap != 8 {
		t.Errorf("queueCap = %d, want 8", d.queueCap)
	}
}
