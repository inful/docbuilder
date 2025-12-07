package observability

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// Span represents a tracing span.
type Span interface {
	SetAttribute(key string, value interface{})
	AddEvent(name string)
	RecordError(err error)
	End()
}

// LocalSpan is a lightweight span implementation for local tracing.
type LocalSpan struct {
	name       string
	startTime  time.Time
	attributes map[string]interface{}
	events     []string
	err        error
}

// SetAttribute sets an attribute on the span.
func (s *LocalSpan) SetAttribute(key string, value interface{}) {
	if s.attributes == nil {
		s.attributes = make(map[string]interface{})
	}
	s.attributes[key] = value
}

// AddEvent adds an event to the span.
func (s *LocalSpan) AddEvent(name string) {
	s.events = append(s.events, name)
}

// RecordError records an error in the span.
func (s *LocalSpan) RecordError(err error) {
	if err != nil {
		s.err = err
		slog.Error("Span error", "span", s.name, "error", err)
	}
}

// End ends the span and logs duration.
func (s *LocalSpan) End() {
	duration := time.Since(s.startTime)
	slog.Debug("Span ended", "span", s.name, "duration_ms", duration.Milliseconds())
}

// TracerProvider manages span creation.
type TracerProvider struct {
	enabled bool
}

// NewTracerProvider creates a new tracer provider.
func NewTracerProvider() *TracerProvider {
	return &TracerProvider{enabled: true}
}

// StartSpan creates a new span for a given operation.
func (tp *TracerProvider) StartSpan(ctx context.Context, spanName string) (context.Context, Span) {
	if !tp.enabled {
		return ctx, &LocalSpan{name: spanName, startTime: time.Now()}
	}

	span := &LocalSpan{
		name:       spanName,
		startTime:  time.Now(),
		attributes: make(map[string]interface{}),
	}

	slog.Debug("Span started", "span", spanName)
	return context.WithValue(ctx, spanContextKey, span), span
}

// StartBuildSpan creates a span for a build operation.
func (tp *TracerProvider) StartBuildSpan(ctx context.Context, buildID string) (context.Context, Span) {
	ctx, span := tp.StartSpan(ctx, "build.process")
	span.SetAttribute("build.id", buildID)
	return ctx, span
}

// StartStageSpan creates a span for a pipeline stage.
func (tp *TracerProvider) StartStageSpan(ctx context.Context, stageName, buildID string) (context.Context, Span) {
	ctx, span := tp.StartSpan(ctx, fmt.Sprintf("stage.%s", stageName))
	span.SetAttribute("build.id", buildID)
	span.SetAttribute("stage.name", stageName)
	return ctx, span
}

// StartAPISpan creates a span for an API handler.
func (tp *TracerProvider) StartAPISpan(ctx context.Context, method, path string) (context.Context, Span) {
	ctx, span := tp.StartSpan(ctx, fmt.Sprintf("api.%s", method))
	span.SetAttribute("http.method", method)
	span.SetAttribute("http.path", path)
	return ctx, span
}

// StartStorageSpan creates a span for a storage operation.
func (tp *TracerProvider) StartStorageSpan(ctx context.Context, operation, objectType string) (context.Context, Span) {
	ctx, span := tp.StartSpan(ctx, fmt.Sprintf("storage.%s", operation))
	span.SetAttribute("storage.operation", operation)
	span.SetAttribute("storage.object_type", objectType)
	return ctx, span
}

// RecordError records an error in a span.
func RecordError(span Span, err error) {
	if err != nil && span != nil {
		span.RecordError(err)
	}
}

// EndSpan ends a span and logs if there was an error.
func EndSpan(span Span, err error) {
	if span != nil {
		if err != nil {
			RecordError(span, err)
		}
		span.End()
	}
}

// GlobalTracerProvider holds the singleton tracer provider.
var globalTracerProvider *TracerProvider

// InitGlobalTracer initializes the global tracer provider.
func InitGlobalTracer() *TracerProvider {
	if globalTracerProvider == nil {
		globalTracerProvider = NewTracerProvider()
	}
	return globalTracerProvider
}

// GetGlobalTracer returns the global tracer provider.
func GetGlobalTracer() *TracerProvider {
	if globalTracerProvider == nil {
		return InitGlobalTracer()
	}
	return globalTracerProvider
}

// SetGlobalTracer sets the global tracer provider (for testing).
func SetGlobalTracer(tp *TracerProvider) {
	globalTracerProvider = tp
}

// Context key for storing span context.
type contextKey string

const spanContextKey contextKey = "span"

// SpanFromContext extracts span from context.
func SpanFromContext(ctx context.Context) (Span, bool) {
	span, ok := ctx.Value(spanContextKey).(Span)
	return span, ok
}
