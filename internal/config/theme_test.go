package config

import "testing"

func TestThemeTypeNormalization(t *testing.T) {
	cases := []struct{ in string; want Theme }{
		{"Hextra", ThemeHextra},
		{"hextra", ThemeHextra},
		{"  HExtra  ", ThemeHextra},
		{"DOCSY", ThemeDocsy},
		{"docsy", ThemeDocsy},
		{"unknown", ""},
		{"", ""},
	}
	for _, c := range cases {
		h := HugoConfig{Theme: c.in}
		if got := h.ThemeType(); got != c.want {
				t.Errorf("ThemeType(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
