package config

import (
	"testing"
)

func TestResolveEffectiveRenderMode(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *Config
		expected RenderMode
	}{
		{
			name:     "nil config returns auto",
			cfg:      nil,
			expected: RenderModeAuto,
		},
		{
			name: "explicit always returns always",
			cfg: &Config{
				Build: BuildConfig{
					RenderMode: RenderModeAlways,
				},
			},
			expected: RenderModeAlways,
		},
		{
			name: "explicit never returns never",
			cfg: &Config{
				Build: BuildConfig{
					RenderMode: RenderModeNever,
				},
			},
			expected: RenderModeNever,
		},
		{
			name: "explicit auto returns auto",
			cfg: &Config{
				Build: BuildConfig{
					RenderMode: RenderModeAuto,
				},
			},
			expected: RenderModeAuto,
		},
		{
			name: "empty render mode returns auto",
			cfg: &Config{
				Build: BuildConfig{
					RenderMode: "",
				},
			},
			expected: RenderModeAuto,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ResolveEffectiveRenderMode(tt.cfg)
			if result != tt.expected {
				t.Errorf("ResolveEffectiveRenderMode() = %v, expected %v", result, tt.expected)
			}
		})
	}
}
