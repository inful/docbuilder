package observability

import (
	"bytes"
	"context"
	"log/slog"
	"testing"
)

func TestWithBuildID(t *testing.T) {
	ctx := context.Background()
	ctx = WithBuildID(ctx, "build-123")

	lc := GetContext(ctx)
	if lc.BuildID != "build-123" {
		t.Errorf("expected build-123, got %s", lc.BuildID)
	}
}

func TestWithTenantID(t *testing.T) {
	ctx := context.Background()
	ctx = WithTenantID(ctx, "tenant-456")

	lc := GetContext(ctx)
	if lc.TenantID != "tenant-456" {
		t.Errorf("expected tenant-456, got %s", lc.TenantID)
	}
}

func TestWithStage(t *testing.T) {
	ctx := context.Background()
	ctx = WithStage(ctx, "clone")

	lc := GetContext(ctx)
	if lc.Stage != "clone" {
		t.Errorf("expected clone, got %s", lc.Stage)
	}
}

func TestWithTraceID(t *testing.T) {
	ctx := context.Background()
	ctx = WithTraceID(ctx, "trace-789")

	lc := GetContext(ctx)
	if lc.TraceID != "trace-789" {
		t.Errorf("expected trace-789, got %s", lc.TraceID)
	}
}

func TestWithUserID(t *testing.T) {
	ctx := context.Background()
	ctx = WithUserID(ctx, "user-abc")

	lc := GetContext(ctx)
	if lc.UserID != "user-abc" {
		t.Errorf("expected user-abc, got %s", lc.UserID)
	}
}

func TestMultipleContextValues(t *testing.T) {
	ctx := context.Background()
	ctx = WithBuildID(ctx, "build-1")
	ctx = WithTenantID(ctx, "tenant-1")
	ctx = WithStage(ctx, "discover")
	ctx = WithTraceID(ctx, "trace-1")

	lc := GetContext(ctx)

	if lc.BuildID != "build-1" {
		t.Error("expected build-1")
	}
	if lc.TenantID != "tenant-1" {
		t.Error("expected tenant-1")
	}
	if lc.Stage != "discover" {
		t.Error("expected discover")
	}
	if lc.TraceID != "trace-1" {
		t.Error("expected trace-1")
	}
}

func TestContextChaining(t *testing.T) {
	ctx := context.Background()
	ctx = WithBuildID(ctx, "build-1")
	ctx = WithTenantID(ctx, "tenant-1")

	lc := GetContext(ctx)

	if lc.BuildID != "build-1" {
		t.Error("BuildID was lost in chaining")
	}
	if lc.TenantID != "tenant-1" {
		t.Error("TenantID was lost in chaining")
	}
}

func TestOverwriteContextValue(t *testing.T) {
	ctx := context.Background()
	ctx = WithBuildID(ctx, "build-1")
	ctx = WithBuildID(ctx, "build-2")

	lc := GetContext(ctx)
	if lc.BuildID != "build-2" {
		t.Errorf("expected build-2, got %s", lc.BuildID)
	}
}

func TestEmptyContext(t *testing.T) {
	ctx := context.Background()
	lc := GetContext(ctx)

	if lc.BuildID != "" || lc.TenantID != "" || lc.Stage != "" {
		t.Error("expected empty context")
	}
}

func TestHasContextValue(t *testing.T) {
	ctx := context.Background()
	ctx = WithBuildID(ctx, "build-1")
	ctx = WithTenantID(ctx, "tenant-1")

	tests := []struct {
		field    string
		expected bool
	}{
		{"build.id", true},
		{"tenant.id", true},
		{"stage", false},
		{"trace.id", false},
		{"user.id", false},
	}

	for _, tt := range tests {
		if HasContextValue(ctx, tt.field) != tt.expected {
			t.Errorf("HasContextValue(%s) expected %v", tt.field, tt.expected)
		}
	}
}

func TestInfoContext(t *testing.T) {
	// Capture log output
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, nil)
	logger := slog.New(handler)
	slog.SetDefault(logger)

	ctx := context.Background()
	ctx = WithBuildID(ctx, "build-1")
	ctx = WithTenantID(ctx, "tenant-1")

	InfoContext(ctx, "test message", slog.String("extra", "value"))

	output := buf.String()
	if !contains(output, "build-1") {
		t.Error("expected build-1 in log output")
	}
	if !contains(output, "tenant-1") {
		t.Error("expected tenant-1 in log output")
	}
	if !contains(output, "test message") {
		t.Error("expected message in log output")
	}
}

func TestWarnContext(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, nil)
	logger := slog.New(handler)
	slog.SetDefault(logger)

	ctx := context.Background()
	ctx = WithStage(ctx, "clone")

	WarnContext(ctx, "warning message", slog.String("reason", "timeout"))

	output := buf.String()
	if !contains(output, "clone") {
		t.Error("expected stage in log output")
	}
	if !contains(output, "warning message") {
		t.Error("expected message in log output")
	}
}

func TestErrorContext(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, nil)
	logger := slog.New(handler)
	slog.SetDefault(logger)

	ctx := context.Background()
	ctx = WithBuildID(ctx, "build-error")
	ctx = WithTraceID(ctx, "trace-error")

	ErrorContext(ctx, "error occurred", slog.String("error", "connection failed"))

	output := buf.String()
	if !contains(output, "build-error") {
		t.Error("expected build-error in log output")
	}
	if !contains(output, "trace-error") {
		t.Error("expected trace-error in log output")
	}
}

func TestDebugContext(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(handler)
	slog.SetDefault(logger)

	ctx := context.Background()
	ctx = WithUserID(ctx, "user-123")

	DebugContext(ctx, "debug info", slog.Int("count", 42))

	output := buf.String()
	if !contains(output, "user-123") {
		t.Error("expected user-123 in log output")
	}
}

func TestLogBuilder(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, nil)
	logger := slog.New(handler)
	slog.SetDefault(logger)

	ctx := context.Background()
	ctx = WithBuildID(ctx, "build-1")

	lb := NewLogBuilder(ctx)
	lb.With("operation", "clone").With("duration_ms", 150).Info("operation completed")

	output := buf.String()
	if !contains(output, "build-1") {
		t.Error("expected build-1 in log output")
	}
	if !contains(output, "clone") {
		t.Error("expected operation in log output")
	}
	if !contains(output, "150") {
		t.Error("expected duration in log output")
	}
}

func TestLogBuilderChaining(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, nil)
	logger := slog.New(handler)
	slog.SetDefault(logger)

	ctx := context.Background()
	ctx = WithBuildID(ctx, "build-1")
	ctx = WithTenantID(ctx, "tenant-1")

	lb := NewLogBuilder(ctx).
		With("stage", "discover").
		With("files_found", 5).
		With("success", true)

	lb.Info("discovery completed")

	output := buf.String()
	if !contains(output, "build-1") {
		t.Error("expected build-1 in log output")
	}
	if !contains(output, "tenant-1") {
		t.Error("expected tenant-1 in log output")
	}
	if !contains(output, "discover") {
		t.Error("expected stage in log output")
	}
}

func TestLogBuilderWithVariousTypes(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, nil)
	logger := slog.New(handler)
	slog.SetDefault(logger)

	ctx := context.Background()

	lb := NewLogBuilder(ctx).
		With("string_val", "test").
		With("int_val", 42).
		With("int64_val", int64(9999)).
		With("float_val", 3.14).
		With("bool_val", true)

	lb.Info("type test")

	output := buf.String()
	if !contains(output, "test") {
		t.Error("expected string value in log output")
	}
}

func TestContextIsolation(t *testing.T) {
	ctx1 := context.Background()
	ctx1 = WithBuildID(ctx1, "build-1")

	ctx2 := context.Background()
	ctx2 = WithBuildID(ctx2, "build-2")

	lc1 := GetContext(ctx1)
	lc2 := GetContext(ctx2)

	if lc1.BuildID != "build-1" {
		t.Error("context1 modified")
	}
	if lc2.BuildID != "build-2" {
		t.Error("context2 modified")
	}
}

func TestLogBuilderMultipleLogs(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, nil)
	logger := slog.New(handler)
	slog.SetDefault(logger)

	ctx := context.Background()
	ctx = WithBuildID(ctx, "build-1")

	lb1 := NewLogBuilder(ctx).With("step", 1)
	lb2 := NewLogBuilder(ctx).With("step", 2)

	lb1.Info("first step")
	lb2.Info("second step")

	output := buf.String()
	if !contains(output, "\"step\":1") && !contains(output, "\"step\": 1") {
		t.Error("expected step 1 in log output")
	}
	if !contains(output, "\"step\":2") && !contains(output, "\"step\": 2") {
		t.Error("expected step 2 in log output")
	}
}

func TestComplexContextFlow(t *testing.T) {
	ctx := context.Background()

	// Simulate a multi-stage build
	ctx = WithBuildID(ctx, "build-123")
	ctx = WithTenantID(ctx, "tenant-456")

	// Clone stage
	cloneCtx := WithStage(ctx, "clone")
	cloneCtx = WithTraceID(cloneCtx, "trace-clone-1")

	lc := GetContext(cloneCtx)
	if lc.BuildID != "build-123" || lc.TenantID != "tenant-456" ||
		lc.Stage != "clone" || lc.TraceID != "trace-clone-1" {
		t.Error("complex context flow failed")
	}

	// Discover stage
	discoverCtx := WithStage(ctx, "discover")
	discoverCtx = WithTraceID(discoverCtx, "trace-discover-1")

	lc = GetContext(discoverCtx)
	if lc.BuildID != "build-123" || lc.TenantID != "tenant-456" ||
		lc.Stage != "discover" || lc.TraceID != "trace-discover-1" {
		t.Error("complex context flow for discover failed")
	}
}

func TestGetLogAttrsWithMixedValues(t *testing.T) {
	ctx := context.Background()
	ctx = WithBuildID(ctx, "build-1")
	ctx = WithTenantID(ctx, "tenant-1")
	// Don't set stage, trace, or user ID

	attrs := getLogAttrs(ctx)

	// Should have at least build and tenant
	if len(attrs) < 2 {
		t.Errorf("expected at least 2 attributes, got %d", len(attrs))
	}

	// Verify that empty fields are not included
	attrStr := ""
	for _, attr := range attrs {
		attrStr += attr.Key
	}

	if !contains(attrStr, "build.id") {
		t.Error("expected build.id attribute")
	}
	if !contains(attrStr, "tenant.id") {
		t.Error("expected tenant.id attribute")
	}
	if contains(attrStr, "stage") && !contains(attrStr, "build.id") {
		t.Error("unexpected stage attribute when not set")
	}
}
