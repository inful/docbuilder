package config

import "testing"

// TestSidebarConfig_SharedMode pins the shared sidebar mode: it keeps the
// categories navigation (only the delivery changes) and is the only mode
// that opts into the shared fragment.
func TestSidebarConfig_SharedMode(t *testing.T) {
	s := &SidebarConfig{Mode: SidebarModeShared}

	if !s.IsCategories() {
		t.Error("shared mode must keep the categories navigation")
	}
	if !s.IsShared() {
		t.Error("shared mode must report IsShared")
	}

	res := &NormalizationResult{}
	s.Normalize(res)
	if s.Mode != SidebarModeShared {
		t.Errorf("Normalize changed the shared mode to %q", s.Mode)
	}
	if len(res.Warnings) != 0 {
		t.Errorf("shared mode produced warnings: %v", res.Warnings)
	}
}

// TestSidebarConfig_SharedModeIsOptIn pins that no other mode renders the
// navigation as a shared fragment.
func TestSidebarConfig_SharedModeIsOptIn(t *testing.T) {
	modes := []SidebarMode{"", SidebarModeAuto, SidebarModeCategories, SidebarModeDefault}
	for _, m := range modes {
		s := &SidebarConfig{Mode: m}
		if s.IsShared() {
			t.Errorf("mode %q must not enable the shared sidebar", m)
		}
	}

	var nilCfg *SidebarConfig
	if nilCfg.IsShared() {
		t.Error("a nil sidebar config must not enable the shared sidebar")
	}
}
