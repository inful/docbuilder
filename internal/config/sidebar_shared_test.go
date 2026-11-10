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

// TestSidebarConfig_SharedModeIsDefault pins that the shared sidebar is
// the default: an unset mode (including a nil config) and the explicit
// "auto"/"shared" modes all render the navigation once, and only
// "categories" and "default" opt out.
func TestSidebarConfig_SharedModeIsDefault(t *testing.T) {
	for _, m := range []SidebarMode{"", SidebarModeAuto, SidebarModeShared} {
		s := &SidebarConfig{Mode: m}
		if !s.IsShared() {
			t.Errorf("mode %q must enable the shared sidebar", m)
		}
	}

	for _, m := range []SidebarMode{SidebarModeCategories, SidebarModeDefault} {
		s := &SidebarConfig{Mode: m}
		if s.IsShared() {
			t.Errorf("mode %q must not enable the shared sidebar", m)
		}
	}

	var nilCfg *SidebarConfig
	if !nilCfg.IsShared() {
		t.Error("a nil sidebar config must enable the shared sidebar")
	}
}
