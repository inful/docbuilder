package observability

import (
	"context"
	"log/slog"
)

// LogContext holds structured logging context information.
type LogContext struct {
	BuildID string
	Stage   string
}

// contextKey is used for context values.
type logContextKeyType string

const logContextKey logContextKeyType = "log-context"

// WithBuildID adds a build ID to the context.
func WithBuildID(ctx context.Context, buildID string) context.Context {
	lc := extractLogContext(ctx)
	lc.BuildID = buildID
	return context.WithValue(ctx, logContextKey, lc)
}

// WithStage adds a stage name to the context.
func WithStage(ctx context.Context, stage string) context.Context {
	lc := extractLogContext(ctx)
	lc.Stage = stage
	return context.WithValue(ctx, logContextKey, lc)
}

// extractLogContext retrieves or creates a LogContext from the context.
func extractLogContext(ctx context.Context) LogContext {
	if lc, ok := ctx.Value(logContextKey).(LogContext); ok {
		return lc
	}
	return LogContext{}
}

// getLogAttrs returns slog attributes from the context's LogContext.
func getLogAttrs(ctx context.Context) []slog.Attr {
	lc := extractLogContext(ctx)
	attrs := []slog.Attr{}

	if lc.BuildID != "" {
		attrs = append(attrs, slog.String("build.id", lc.BuildID))
	}
	if lc.Stage != "" {
		attrs = append(attrs, slog.String("stage", lc.Stage))
	}

	return attrs
}

// InfoContext logs an info message with context information.
func InfoContext(ctx context.Context, msg string, attrs ...slog.Attr) {
	logAttrs(ctx, slog.LevelInfo, msg, attrs)
}

// WarnContext logs a warning message with context information.
func WarnContext(ctx context.Context, msg string, attrs ...slog.Attr) {
	logAttrs(ctx, slog.LevelWarn, msg, attrs)
}

// ErrorContext logs an error message with context information.
func ErrorContext(ctx context.Context, msg string, attrs ...slog.Attr) {
	logAttrs(ctx, slog.LevelError, msg, attrs)
}

// DebugContext logs a debug message with context information.
func DebugContext(ctx context.Context, msg string, attrs ...slog.Attr) {
	logAttrs(ctx, slog.LevelDebug, msg, attrs)
}

func logAttrs(ctx context.Context, level slog.Level, msg string, attrs []slog.Attr) {
	slog.LogAttrs(ctx, level, msg, append(getLogAttrs(ctx), attrs...)...)
}
