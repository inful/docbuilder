package config

import (
	"strings"
	"testing"
)

// The chrome mode decides how the theme renders the navigation. An
// unknown value has to fall back to the default, loudly: the site
// would otherwise render a chrome mode nobody asked for.
func TestNormalizeHugoChrome(t *testing.T) {
	tests := []struct {
		name     string
		chrome   string
		want     string
		wantWarn bool
	}{
		{name: "unset keeps the default", chrome: "", want: ""},
		{name: "full is kept", chrome: ChromeFull, want: ChromeFull},
		{name: "shared is kept", chrome: ChromeShared, want: ChromeShared},
		{
			name:     "an unknown mode falls back to full",
			chrome:   "sidebar",
			want:     ChromeFull,
			wantWarn: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{Version: "2.0", Hugo: HugoConfig{Chrome: tt.chrome}}
			res, err := NormalizeConfig(cfg)
			if err != nil {
				t.Fatalf("NormalizeConfig error: %v", err)
			}
			if cfg.Hugo.Chrome != tt.want {
				t.Errorf("chrome: got %q want %q", cfg.Hugo.Chrome, tt.want)
			}
			// Only the chrome warnings are of interest here; the other
			// normalizers warn about their own fields.
			chromeWarn := false
			for _, w := range res.Warnings {
				if strings.Contains(w, "hugo.chrome") {
					chromeWarn = true
					break
				}
			}
			if chromeWarn != tt.wantWarn {
				t.Errorf("chrome warning: got %v want %v (warnings: %v)", chromeWarn, tt.wantWarn, res.Warnings)
			}
		})
	}
}
