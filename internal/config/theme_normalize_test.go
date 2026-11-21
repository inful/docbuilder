package config

import "testing"

func TestNormalizeHugoTheme(t *testing.T) {
	tests := []struct {
		name        string
		theme       string
		wantTheme   string
		wantWarning bool
	}{
		{name: "unset stays unset", theme: "", wantTheme: ""},
		{name: "relearn is kept", theme: ThemeRelearn, wantTheme: ThemeRelearn},
		{name: "docsy is kept", theme: ThemeDocsy, wantTheme: ThemeDocsy},
		{name: "unknown falls back", theme: "material", wantTheme: ThemeRelearn, wantWarning: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &HugoConfig{Theme: tt.theme}
			res := &NormalizationResult{}

			normalizeHugoConfig(h, res)

			if h.Theme != tt.wantTheme {
				t.Errorf("theme = %q, want %q", h.Theme, tt.wantTheme)
			}
			if got := len(res.Warnings) > 0; got != tt.wantWarning {
				t.Errorf("warnings = %v, want a warning: %v", res.Warnings, tt.wantWarning)
			}
		})
	}
}

func TestThemeNameDefaultsToRelearn(t *testing.T) {
	var nilConfig *HugoConfig
	if got := nilConfig.ThemeName(); got != ThemeRelearn {
		t.Errorf("ThemeName() = %q, want %q", got, ThemeRelearn)
	}
	if got := (&HugoConfig{}).ThemeName(); got != ThemeRelearn {
		t.Errorf("ThemeName() = %q, want %q", got, ThemeRelearn)
	}
	if got := (&HugoConfig{Theme: ThemeDocsy}).ThemeName(); got != ThemeDocsy {
		t.Errorf("ThemeName() = %q, want %q", got, ThemeDocsy)
	}
}
