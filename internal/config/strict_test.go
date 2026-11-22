package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUnmarshalStrict_RejectsUnknownKey(t *testing.T) {
	raw := "version: \"2.0\"\nhugo:\n  theme: relearn\n"
	_, err := unmarshalStrict([]byte(raw))
	if err == nil {
		t.Fatal("expected an error for the unknown hugo.theme key")
	}
	if !strings.Contains(err.Error(), "theme") {
		t.Errorf("error should name the offending key, got: %v", err)
	}
}

// A key that the schema knows at another path has to say where it
// belongs: the operator moved it to the wrong level, and the error is
// the only place that can tell them.
func TestUnmarshalStrict_UnknownKeyNamesWhereItBelongs(t *testing.T) {
	raw := "version: \"2.0\"\noutbound:\n  ragabast:\n    enabled: true\n"
	_, err := unmarshalStrict([]byte(raw))
	if err == nil {
		t.Fatal("expected an error for the misplaced outbound key")
	}
	if !strings.Contains(err.Error(), "daemon.outbound") {
		t.Errorf("error should point at daemon.outbound, got: %v", err)
	}
}

// The hint must not fire for a key that is unknown everywhere.
func TestUnmarshalStrict_UnknownKeyWithoutAPath(t *testing.T) {
	raw := "version: \"2.0\"\nnot_a_key: 1\n"
	_, err := unmarshalStrict([]byte(raw))
	if err == nil {
		t.Fatal("expected an error for the unknown not_a_key")
	}
	if strings.Contains(err.Error(), "is accepted at") {
		t.Errorf("error should not name a path for an unknown key, got: %v", err)
	}
}

// The key the hint points at has to be the one the schema reads.
func TestUnmarshalStrict_OutboundIsReadUnderDaemon(t *testing.T) {
	raw := "version: \"2.0\"\ndaemon:\n  outbound:\n    ragabast:\n      enabled: true\n      ragabast_base_url: \"https://ragabast.example.com\"\n"
	cfg, err := unmarshalStrict([]byte(raw))
	if err != nil {
		t.Fatalf("daemon.outbound must load: %v", err)
	}
	if cfg.Daemon == nil || cfg.Daemon.Outbound == nil || cfg.Daemon.Outbound.Ragabast == nil {
		t.Fatal("daemon.outbound.ragabast was not read")
	}
	if !cfg.Daemon.Outbound.Ragabast.Enabled {
		t.Error("ragabast should be enabled")
	}
}

func TestUnmarshalStrict_RejectsUnknownNestedKey(t *testing.T) {
	raw := "version: \"2.0\"\ndaemon:\n  sync:\n    interval_minutes: 5\n"
	_, err := unmarshalStrict([]byte(raw))
	if err == nil {
		t.Fatal("expected an error for the unknown daemon.sync.interval_minutes key")
	}
	if !strings.Contains(err.Error(), "interval_minutes") {
		t.Errorf("error should name the offending key, got: %v", err)
	}
}

// The transform pipeline is fixed (ADR-003), so there is no
// hugo.transforms key. It used to be accepted and silently ignored;
// now it has to be rejected so the user learns it does nothing.
func TestUnmarshalStrict_RejectsTransformsKey(t *testing.T) {
	raw := "version: \"2.0\"\nhugo:\n  transforms:\n    disable:\n      - some_transform\n"
	_, err := unmarshalStrict([]byte(raw))
	if err == nil {
		t.Fatal("expected an error for the unknown hugo.transforms key")
	}
	if !strings.Contains(err.Error(), "transforms") {
		t.Errorf("error should name the offending key, got: %v", err)
	}
}

func TestUnmarshalStrict_AcceptsKnownKeys(t *testing.T) {
	raw := "version: \"2.0\"\nhugo:\n  title: T\n  base_url: https://example.com\n" +
		"repositories:\n  - url: https://example.com/a.git\n    name: a\n"
	cfg, err := unmarshalStrict([]byte(raw))
	if err != nil {
		t.Fatalf("known keys must load: %v", err)
	}
	if cfg.Hugo.Title != "T" {
		t.Errorf("title: got %q want %q", cfg.Hugo.Title, "T")
	}
}

func TestUnmarshalStrict_AllowsArbitraryKeysInsideMaps(t *testing.T) {
	raw := "version: \"2.0\"\nhugo:\n  params:\n    anything: 1\n    nested:\n      deep: 2\n" +
		"forges:\n  - name: g\n    type: github\n    options:\n      custom: 3\n"
	if _, err := unmarshalStrict([]byte(raw)); err != nil {
		t.Fatalf("map values accept any key: %v", err)
	}
}

func TestLoadWithResult_RejectsUnknownKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	raw := "version: \"2.0\"\nhugo:\n  theme: relearn\n" +
		"repositories:\n  - url: https://example.com/a.git\n    name: a\n"
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	_, _, err := LoadWithResult(path)
	if err == nil {
		t.Fatal("expected LoadWithResult to reject the unknown key")
	}
	if !strings.Contains(err.Error(), "theme") {
		t.Errorf("error should name the offending key, got: %v", err)
	}
}
