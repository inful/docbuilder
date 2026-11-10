package stages

import (
	"log/slog"
	"testing"
)

// TestHugoLogLevel pins how Hugo's own log lines map to DocBuilder's log
// levels. Hugo writes its progress output to stderr, so INFO lines (the
// per-step timings included) must not be reported as warnings.
func TestHugoLogLevel(t *testing.T) {
	cases := []struct {
		line string
		want slog.Level
	}{
		{"INFO  build:  step render pages 1587 duration 3m17s", slog.LevelInfo},
		{"INFO  static: syncing static files to /", slog.LevelInfo},
		{"WARN  deprecated: .Language.LanguageCode was deprecated", slog.LevelWarn},
		{"ERROR error building site: boom", slog.LevelError},
		{"Error: error building site: boom", slog.LevelError},
		{"DEBUG build:  step process", slog.LevelDebug},
		{"TRACE  something", slog.LevelDebug},
		{"", slog.LevelInfo},
		{"plain output without a level", slog.LevelInfo},
	}

	for _, tc := range cases {
		if got := hugoLogLevel(tc.line); got != tc.want {
			t.Errorf("hugoLogLevel(%q) = %v, want %v", tc.line, got, tc.want)
		}
	}
}
