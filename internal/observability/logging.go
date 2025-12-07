package observability

import (
	"context"
	"log/slog"
)

// LogContext holds structured logging context information.
type LogContext struct {
	BuildID  string
	TenantID string
	Stage    string
	TraceID  string
	UserID   string
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

// WithTenantID adds a tenant ID to the context.
func WithTenantID(ctx context.Context, tenantID string) context.Context {
	lc := extractLogContext(ctx)
	lc.TenantID = tenantID
	return context.WithValue(ctx, logContextKey, lc)
}

// WithStage adds a stage name to the context.
func WithStage(ctx context.Context, stage string) context.Context {
	lc := extractLogContext(ctx)
	lc.Stage = stage
	return context.WithValue(ctx, logContextKey, lc)
}

// WithTraceID adds a trace ID to the context.
func WithTraceID(ctx context.Context, traceID string) context.Context {
	lc := extractLogContext(ctx)
	lc.TraceID = traceID
	return context.WithValue(ctx, logContextKey, lc)
}

// WithUserID adds a user ID to the context.
func WithUserID(ctx context.Context, userID string) context.Context {
	lc := extractLogContext(ctx)
	lc.UserID = userID
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
	if lc.TenantID != "" {
		attrs = append(attrs, slog.String("tenant.id", lc.TenantID))
	}
	if lc.Stage != "" {
		attrs = append(attrs, slog.String("stage", lc.Stage))
	}
	if lc.TraceID != "" {
		attrs = append(attrs, slog.String("trace.id", lc.TraceID))
	}
	if lc.UserID != "" {
		attrs = append(attrs, slog.String("user.id", lc.UserID))
	}

	return attrs
}

// InfoContext logs an info message with context information.
func InfoContext(ctx context.Context, msg string, attrs ...slog.Attr) {
	contextAttrs := getLogAttrs(ctx)
	allAttrs := append(contextAttrs, attrs...)
	slog.LogAttrs(ctx, slog.LevelInfo, msg, allAttrs...)
}

// WarnContext logs a warning message with context information.
func WarnContext(ctx context.Context, msg string, attrs ...slog.Attr) {
	contextAttrs := getLogAttrs(ctx)
	allAttrs := append(contextAttrs, attrs...)
	slog.LogAttrs(ctx, slog.LevelWarn, msg, allAttrs...)
}

// ErrorContext logs an error message with context information.
func ErrorContext(ctx context.Context, msg string, attrs ...slog.Attr) {
	contextAttrs := getLogAttrs(ctx)
	allAttrs := append(contextAttrs, attrs...)
	slog.LogAttrs(ctx, slog.LevelError, msg, allAttrs...)
}

// DebugContext logs a debug message with context information.
func DebugContext(ctx context.Context, msg string, attrs ...slog.Attr) {
	contextAttrs := getLogAttrs(ctx)
	allAttrs := append(contextAttrs, attrs...)
	slog.LogAttrs(ctx, slog.LevelDebug, msg, allAttrs...)
}

// LogBuilder is a helper for building log messages with context.
type LogBuilder struct {
	ctx   context.Context
	attrs []slog.Attr
}

// NewLogBuilder creates a new log builder with context.
func NewLogBuilder(ctx context.Context) *LogBuilder {
	return &LogBuilder{
		ctx:   ctx,
		attrs: getLogAttrs(ctx),
	}
}

// With adds an attribute to the log builder.
func (lb *LogBuilder) With(key string, value interface{}) *LogBuilder {
	switch v := value.(type) {
	case string:
		lb.attrs = append(lb.attrs, slog.String(key, v))
	case int:
		lb.attrs = append(lb.attrs, slog.Int(key, v))
	case int64:
		lb.attrs = append(lb.attrs, slog.Int64(key, v))
	case float64:
		lb.attrs = append(lb.attrs, slog.Float64(key, v))
	case bool:
		lb.attrs = append(lb.attrs, slog.Bool(key, v))
	default:
		lb.attrs = append(lb.attrs, slog.Any(key, v))
	}
	return lb
}

// Info logs an info message with accumulated attributes.
func (lb *LogBuilder) Info(msg string) {
	slog.LogAttrs(lb.ctx, slog.LevelInfo, msg, lb.attrs...)
}

// Warn logs a warning message with accumulated attributes.
func (lb *LogBuilder) Warn(msg string) {
	slog.LogAttrs(lb.ctx, slog.LevelWarn, msg, lb.attrs...)
}

// Error logs an error message with accumulated attributes.
func (lb *LogBuilder) Error(msg string) {
	slog.LogAttrs(lb.ctx, slog.LevelError, msg, lb.attrs...)
}

// Debug logs a debug message with accumulated attributes.
func (lb *LogBuilder) Debug(msg string) {
	slog.LogAttrs(lb.ctx, slog.LevelDebug, msg, lb.attrs...)
}

// GetContext returns the structured log context from the provided context.
func GetContext(ctx context.Context) LogContext {
	return extractLogContext(ctx)
}

// HasContextValue checks if a specific context value is set.
func HasContextValue(ctx context.Context, field string) bool {
	lc := extractLogContext(ctx)
	switch field {
	case "build.id":
		return lc.BuildID != ""
	case "tenant.id":
		return lc.TenantID != ""
	case "stage":
		return lc.Stage != ""
	case "trace.id":
		return lc.TraceID != ""
	case "user.id":
		return lc.UserID != ""
	default:
		return false
	}
}
