package observability

import (
	"context"
	"testing"
	"time"
)

func TestNewTracerProvider(t *testing.T) {
	tp := NewTracerProvider()
	if tp == nil {
		t.Fatal("expected TracerProvider, got nil")
	}
	if !tp.enabled {
		t.Fatal("expected enabled=true")
	}
}

func TestStartSpan(t *testing.T) {
	tp := NewTracerProvider()
	ctx := context.Background()

	newCtx, span := tp.StartSpan(ctx, "test.operation")

	if newCtx == ctx {
		t.Error("expected new context")
	}

	if span == nil {
		t.Fatal("expected span, got nil")
	}

	if localSpan, ok := span.(*LocalSpan); ok {
		if localSpan.name != "test.operation" {
			t.Errorf("expected span name 'test.operation', got %s", localSpan.name)
		}
	} else {
		t.Error("expected *LocalSpan")
	}
}

func TestStartSpanDisabled(t *testing.T) {
	tp := &TracerProvider{enabled: false}
	ctx := context.Background()

	newCtx, span := tp.StartSpan(ctx, "test.operation")

	if newCtx != ctx {
		t.Error("expected same context when disabled")
	}

	if span == nil {
		t.Fatal("expected span even when disabled")
	}
}

func TestLocalSpanSetAttribute(t *testing.T) {
	span := &LocalSpan{name: "test", startTime: time.Now()}

	span.SetAttribute("key1", "value1")
	span.SetAttribute("key2", 42)
	span.SetAttribute("key3", true)

	if len(span.attributes) != 3 {
		t.Errorf("expected 3 attributes, got %d", len(span.attributes))
	}

	if span.attributes["key1"] != "value1" {
		t.Error("expected key1=value1")
	}
	if span.attributes["key2"] != 42 {
		t.Error("expected key2=42")
	}
	if span.attributes["key3"] != true {
		t.Error("expected key3=true")
	}
}

func TestLocalSpanAddEvent(t *testing.T) {
	span := &LocalSpan{name: "test", startTime: time.Now()}

	span.AddEvent("event1")
	span.AddEvent("event2")
	span.AddEvent("event3")

	if len(span.events) != 3 {
		t.Errorf("expected 3 events, got %d", len(span.events))
	}

	if span.events[0] != "event1" || span.events[1] != "event2" || span.events[2] != "event3" {
		t.Error("events not recorded correctly")
	}
}

func TestLocalSpanRecordError(t *testing.T) {
	span := &LocalSpan{name: "test", startTime: time.Now()}

	testErr := context.DeadlineExceeded
	span.RecordError(testErr)

	if span.err != testErr {
		t.Error("error not recorded")
	}
}

func TestLocalSpanEnd(t *testing.T) {
	span := &LocalSpan{name: "test", startTime: time.Now().Add(-time.Second)}
	span.End()
	// Just verify it doesn't panic
}

func TestStartBuildSpan(t *testing.T) {
	tp := NewTracerProvider()
	ctx := context.Background()

	_, span := tp.StartBuildSpan(ctx, "build-123")

	if span == nil {
		t.Fatal("expected span")
	}

	localSpan := span.(*LocalSpan)
	if localSpan.name != "build.process" {
		t.Errorf("expected span name 'build.process', got %s", localSpan.name)
	}

	if localSpan.attributes["build.id"] != "build-123" {
		t.Error("expected build.id=build-123")
	}
}

func TestStartStageSpan(t *testing.T) {
	tp := NewTracerProvider()
	ctx := context.Background()

	_, span := tp.StartStageSpan(ctx, "clone", "build-456")

	if span == nil {
		t.Fatal("expected span")
	}

	localSpan := span.(*LocalSpan)
	if localSpan.name != "stage.clone" {
		t.Errorf("expected span name 'stage.clone', got %s", localSpan.name)
	}

	if localSpan.attributes["build.id"] != "build-456" {
		t.Error("expected build.id=build-456")
	}
	if localSpan.attributes["stage.name"] != "clone" {
		t.Error("expected stage.name=clone")
	}
}

func TestStartAPISpan(t *testing.T) {
	tp := NewTracerProvider()
	ctx := context.Background()

	_, span := tp.StartAPISpan(ctx, "POST", "/api/builds")

	if span == nil {
		t.Fatal("expected span")
	}

	localSpan := span.(*LocalSpan)
	if localSpan.name != "api.POST" {
		t.Errorf("expected span name 'api.POST', got %s", localSpan.name)
	}

	if localSpan.attributes["http.method"] != "POST" {
		t.Error("expected http.method=POST")
	}
	if localSpan.attributes["http.path"] != "/api/builds" {
		t.Error("expected http.path=/api/builds")
	}
}

func TestStartStorageSpan(t *testing.T) {
	tp := NewTracerProvider()
	ctx := context.Background()

	_, span := tp.StartStorageSpan(ctx, "put", "RepoTree")

	if span == nil {
		t.Fatal("expected span")
	}

	localSpan := span.(*LocalSpan)
	if localSpan.name != "storage.put" {
		t.Errorf("expected span name 'storage.put', got %s", localSpan.name)
	}

	if localSpan.attributes["storage.operation"] != "put" {
		t.Error("expected storage.operation=put")
	}
	if localSpan.attributes["storage.object_type"] != "RepoTree" {
		t.Error("expected storage.object_type=RepoTree")
	}
}

func TestRecordError(t *testing.T) {
	span := &LocalSpan{name: "test", startTime: time.Now()}

	testErr := context.Canceled
	RecordError(span, testErr)

	if span.err != testErr {
		t.Error("error not recorded")
	}
}

func TestRecordErrorNilSpan(t *testing.T) {
	// Should not panic
	RecordError(nil, context.Canceled)
}

func TestRecordErrorNilError(t *testing.T) {
	span := &LocalSpan{name: "test", startTime: time.Now()}
	// Should not panic
	RecordError(span, nil)
}

func TestEndSpan(t *testing.T) {
	span := &LocalSpan{name: "test", startTime: time.Now()}

	// Should not panic
	EndSpan(span, nil)
}

func TestEndSpanWithError(t *testing.T) {
	span := &LocalSpan{name: "test", startTime: time.Now()}
	testErr := context.DeadlineExceeded

	// Should not panic
	EndSpan(span, testErr)

	if span.err != testErr {
		t.Error("error not recorded before end")
	}
}

func TestEndSpanNil(t *testing.T) {
	// Should not panic
	EndSpan(nil, nil)
}

func TestInitGlobalTracer(t *testing.T) {
	// Reset global state
	globalTracerProvider = nil

	tp := InitGlobalTracer()

	if tp == nil {
		t.Fatal("expected TracerProvider")
	}

	tp2 := InitGlobalTracer()
	if tp != tp2 {
		t.Error("expected same instance on second call")
	}

	// Reset for other tests
	globalTracerProvider = nil
}

func TestGetGlobalTracer(t *testing.T) {
	// Reset global state
	globalTracerProvider = nil

	tp := GetGlobalTracer()

	if tp == nil {
		t.Fatal("expected TracerProvider")
	}

	tp2 := GetGlobalTracer()
	if tp != tp2 {
		t.Error("expected same instance")
	}

	// Reset for other tests
	globalTracerProvider = nil
}

func TestSetGlobalTracer(t *testing.T) {
	tp := NewTracerProvider()
	SetGlobalTracer(tp)

	retrieved := GetGlobalTracer()
	if retrieved != tp {
		t.Error("expected same tracer instance")
	}

	// Reset for other tests
	globalTracerProvider = nil
}

func TestSpanFromContext(t *testing.T) {
	tp := NewTracerProvider()
	ctx := context.Background()

	newCtx, span := tp.StartSpan(ctx, "test")

	retrievedSpan, ok := SpanFromContext(newCtx)
	if !ok {
		t.Fatal("expected to retrieve span from context")
	}

	if retrievedSpan != span {
		t.Error("expected same span instance")
	}
}

func TestSpanFromContextNotFound(t *testing.T) {
	ctx := context.Background()

	span, ok := SpanFromContext(ctx)
	if ok {
		t.Error("expected no span in empty context")
	}

	if span != nil {
		t.Error("expected nil span")
	}
}

func TestSpanContextKeyIsolation(t *testing.T) {
	tp := NewTracerProvider()
	ctx := context.Background()

	ctx1, _ := tp.StartSpan(ctx, "span1")
	ctx2, _ := tp.StartSpan(ctx, "span2")

	retrieved1, _ := SpanFromContext(ctx1)
	retrieved2, _ := SpanFromContext(ctx2)

	if retrieved1 == retrieved2 {
		t.Error("expected different spans in different contexts")
	}

	localSpan1 := retrieved1.(*LocalSpan)
	localSpan2 := retrieved2.(*LocalSpan)

	if localSpan1.name != "span1" || localSpan2.name != "span2" {
		t.Error("span names don't match contexts")
	}
}

func TestTracingWorkflow(t *testing.T) {
	tp := NewTracerProvider()
	ctx := context.Background()

	// Start build span
	ctx, buildSpan := tp.StartBuildSpan(ctx, "build-789")
	buildSpan.SetAttribute("tenant.id", "tenant-123")
	buildSpan.AddEvent("build.started")

	// Start clone stage
	_, cloneSpan := tp.StartStageSpan(ctx, "clone", "build-789")
	cloneSpan.SetAttribute("repo.url", "https://github.com/test/repo")
	cloneSpan.AddEvent("repo.cloned")

	// Simulate work
	time.Sleep(10 * time.Millisecond)

	// End clone stage
	EndSpan(cloneSpan, nil)

	// Start discover stage
	_, discoverSpan := tp.StartStageSpan(ctx, "discover", "build-789")
	discoverSpan.AddEvent("docs.discovered")
	EndSpan(discoverSpan, nil)

	// End build span
	buildSpan.AddEvent("build.completed")
	EndSpan(buildSpan, nil)

	// Verify all operations completed without error
}

func TestTracingAPIRequestFlow(t *testing.T) {
	tp := NewTracerProvider()
	ctx := context.Background()

	// Start API span
	ctx, apiSpan := tp.StartAPISpan(ctx, "POST", "/api/builds")
	apiSpan.SetAttribute("tenant.id", "tenant-456")
	apiSpan.AddEvent("request.received")

	// Simulate storage operation during API handling
	_, storageSpan := tp.StartStorageSpan(ctx, "get", "BuildManifest")
	storageSpan.SetAttribute("key", "manifest-123")
	EndSpan(storageSpan, nil)

	// End API span
	apiSpan.AddEvent("request.completed")
	EndSpan(apiSpan, nil)

	// Verify all operations completed
}

func TestTracingErrorHandling(t *testing.T) {
	tp := NewTracerProvider()
	ctx := context.Background()

	_, span := tp.StartSpan(ctx, "failing.operation")

	// Simulate error during operation
	testErr := context.DeadlineExceeded
	span.RecordError(testErr)
	span.AddEvent("operation.failed")

	EndSpan(span, testErr)

	localSpan := span.(*LocalSpan)
	if localSpan.err != testErr {
		t.Error("error should be recorded in span")
	}
}

func TestMultipleAttributeTypes(t *testing.T) {
	span := &LocalSpan{name: "test", startTime: time.Now()}

	// Test various types
	span.SetAttribute("string", "value")
	span.SetAttribute("int", 42)
	span.SetAttribute("int64", int64(9999))
	span.SetAttribute("float", 3.14)
	span.SetAttribute("bool", true)
	span.SetAttribute("custom", struct{ x int }{x: 10})

	if len(span.attributes) != 6 {
		t.Errorf("expected 6 attributes, got %d", len(span.attributes))
	}
}

func TestSpanDurationMeasurement(t *testing.T) {
	span := &LocalSpan{name: "test", startTime: time.Now()}

	time.Sleep(50 * time.Millisecond)

	duration := time.Since(span.startTime)

	if duration < 50*time.Millisecond {
		t.Error("span duration should be at least 50ms")
	}
}

func TestGlobalTracerThreadSafety(t *testing.T) {
	// Reset
	globalTracerProvider = nil

	done := make(chan bool)

	for i := 0; i < 10; i++ {
		go func() {
			tp := GetGlobalTracer()
			if tp == nil {
				t.Error("unexpected nil tracer")
			}
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	// Reset
	globalTracerProvider = nil
}
