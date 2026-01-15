package observability

import (
	"bytes"
	"log/slog"
	"testing"
)

func TestWithBuildID(t *testing.T) {
	ctx := t.Context()
	ctx = WithBuildID(ctx, "build-123")

	lc := extractLogContext(ctx)
	if lc.BuildID != "build-123" {
		t.Errorf("expected build-123, got %s", lc.BuildID)
	}
}

func TestWithStage(t *testing.T) {
	ctx := t.Context()
	ctx = WithStage(ctx, "clone")

	lc := extractLogContext(ctx)
	if lc.Stage != "clone" {
		t.Errorf("expected clone, got %s", lc.Stage)
	}
}

func TestContextChaining(t *testing.T) {
	ctx := t.Context()
	ctx = WithBuildID(ctx, "build-1")
	ctx = WithStage(ctx, "discover")

	lc := extractLogContext(ctx)
	if lc.BuildID != "build-1" || lc.Stage != "discover" {
		t.Errorf("expected build-1 and discover, got %s and %s", lc.BuildID, lc.Stage)
	}
}

func TestOverwriteContextValue(t *testing.T) {
	ctx := t.Context()
	ctx = WithBuildID(ctx, "build-old")
	ctx = WithBuildID(ctx, "build-new")

	lc := extractLogContext(ctx)
	if lc.BuildID != "build-new" {
		t.Errorf("expected build-new, got %s", lc.BuildID)
	}
}

func TestEmptyContext(t *testing.T) {
	ctx := t.Context()

	lc := extractLogContext(ctx)
	if lc.BuildID != "" || lc.Stage != "" {
		t.Errorf("expected empty context, got %+v", lc)
	}
}

func TestInfoContext(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	ctx := t.Context()
	ctx = WithBuildID(ctx, "build-123")
	ctx = WithStage(ctx, "clone")

	InfoContext(ctx, "test info message", slog.String("extra", "value"))

	output := buf.String()
	if output == "" {
		t.Error("expected log output, got empty string")
	}
}

func TestWarnContext(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelWarn,
	}))
	slog.SetDefault(logger)

	ctx := t.Context()
	ctx = WithBuildID(ctx, "build-456")

	WarnContext(ctx, "test warning", slog.String("reason", "test"))

	output := buf.String()
	if output == "" {
		t.Error("expected log output, got empty string")
	}
}

func TestErrorContext(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))
	slog.SetDefault(logger)

	ctx := t.Context()
	ctx = WithStage(ctx, "hugo")

	ErrorContext(ctx, "test error", slog.String("error", "failed"))

	output := buf.String()
	if output == "" {
		t.Error("expected log output, got empty string")
	}
}

func TestDebugContext(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	slog.SetDefault(logger)

	ctx := t.Context()
	ctx = WithBuildID(ctx, "build-debug")

	DebugContext(ctx, "debug message", slog.Int("count", 42))

	output := buf.String()
	if output == "" {
		t.Error("expected log output, got empty string")
	}
}

func TestContextIsolation(t *testing.T) {
	ctx1 := t.Context()
	ctx1 = WithBuildID(ctx1, "build-1")

	ctx2 := t.Context()
	ctx2 = WithBuildID(ctx2, "build-2")

	lc1 := extractLogContext(ctx1)
	lc2 := extractLogContext(ctx2)

	if lc1.BuildID != "build-1" {
		t.Errorf("ctx1: expected build-1, got %s", lc1.BuildID)
	}
	if lc2.BuildID != "build-2" {
		t.Errorf("ctx2: expected build-2, got %s", lc2.BuildID)
	}
}

func TestGetLogAttrsWithMixedValues(t *testing.T) {
	ctx := t.Context()
	ctx = WithBuildID(ctx, "build-mixed")
	ctx = WithStage(ctx, "test-stage")

	attrs := getLogAttrs(ctx)

	if len(attrs) != 2 {
		t.Errorf("expected 2 attrs, got %d", len(attrs))
	}

	// Check that build.id and stage are both present
	foundBuildID := false
	foundStage := false
	for _, attr := range attrs {
		if attr.Key == "build.id" && attr.Value.String() == "build-mixed" {
			foundBuildID = true
		}
		if attr.Key == "stage" && attr.Value.String() == "test-stage" {
			foundStage = true
		}
	}

	if !foundBuildID {
		t.Error("expected to find build.id attr")
	}
	if !foundStage {
		t.Error("expected to find stage attr")
	}
}

func TestGetLogAttrsEmptyContext(t *testing.T) {
	ctx := t.Context()
	attrs := getLogAttrs(ctx)

	if len(attrs) != 0 {
		t.Errorf("expected 0 attrs, got %d", len(attrs))
	}
}
