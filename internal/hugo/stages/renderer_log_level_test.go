package stages

import (
	"bytes"
	"log/slog"
	"testing"
)

// Hugo writes a lot at debug level - tens of megabytes for a big site
// - and the daemon drops those lines at the usual log level anyway.
// So it is only asked for them when the daemon logs at debug level.
func TestHugoLogLevelArg(t *testing.T) {
	old := slog.Default()
	defer slog.SetDefault(old)

	var sink bytes.Buffer

	slog.SetDefault(slog.New(slog.NewTextHandler(&sink, &slog.HandlerOptions{Level: slog.LevelInfo})))
	if got := hugoLogLevelArg(t.Context()); got != "info" {
		t.Errorf("hugoLogLevelArg = %q, want %q", got, "info")
	}

	slog.SetDefault(slog.New(slog.NewTextHandler(&sink, &slog.HandlerOptions{Level: slog.LevelDebug})))
	if got := hugoLogLevelArg(t.Context()); got != "debug" {
		t.Errorf("hugoLogLevelArg = %q, want %q", got, "debug")
	}
}
