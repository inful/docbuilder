package observability

import (
	"testing"
	"time"
)

func TestNewMetricsCollector(t *testing.T) {
	mc := NewMetricsCollector()
	if mc == nil {
		t.Fatal("expected MetricsCollector")
	}

	if mc.buildCount != 0 {
		t.Error("expected buildCount=0")
	}
	if mc.cacheHits != 0 {
		t.Error("expected cacheHits=0")
	}
}

func TestRecordBuildStart(t *testing.T) {
	mc := NewMetricsCollector()

	mc.RecordBuildStart("build-1", "tenant-1")

	if mc.buildCount != 1 {
		t.Errorf("expected buildCount=1, got %d", mc.buildCount)
	}
	if mc.currentConcurrent != 1 {
		t.Errorf("expected concurrent=1, got %d", mc.currentConcurrent)
	}
	if mc.buildsByStatus["started"] != 1 {
		t.Error("expected started status")
	}
}

func TestRecordBuildEnd(t *testing.T) {
	mc := NewMetricsCollector()

	mc.RecordBuildStart("build-1", "tenant-1")
	mc.RecordBuildEnd(100*time.Millisecond, true, "tenant-1")

	if mc.currentConcurrent != 0 {
		t.Errorf("expected concurrent=0, got %d", mc.currentConcurrent)
	}
	if mc.buildsByStatus["completed"] != 1 {
		t.Error("expected completed status")
	}
	if len(mc.buildDurations) != 1 {
		t.Error("expected duration recorded")
	}
}

func TestRecordBuildEndFailure(t *testing.T) {
	mc := NewMetricsCollector()

	mc.RecordBuildStart("build-1", "tenant-1")
	mc.RecordBuildEnd(50*time.Millisecond, false, "tenant-1")

	if mc.buildErrors != 1 {
		t.Errorf("expected 1 error, got %d", mc.buildErrors)
	}
	if mc.buildsByStatus["failed"] != 1 {
		t.Error("expected failed status")
	}
}

func TestRecordCacheHit(t *testing.T) {
	mc := NewMetricsCollector()

	mc.RecordCacheHit("build-signature")
	mc.RecordCacheHit("repo-tree")

	if mc.cacheHits != 2 {
		t.Errorf("expected 2 hits, got %d", mc.cacheHits)
	}
}

func TestRecordCacheMiss(t *testing.T) {
	mc := NewMetricsCollector()

	mc.RecordCacheMiss("build-signature")

	if mc.cacheMisses != 1 {
		t.Errorf("expected 1 miss, got %d", mc.cacheMisses)
	}
}

func TestRecordStage(t *testing.T) {
	mc := NewMetricsCollector()

	mc.RecordStage("clone", 100*time.Millisecond, true)
	mc.RecordStage("discover", 50*time.Millisecond, true)

	if mc.stageCount["clone"] != 1 {
		t.Error("expected clone stage count")
	}
	if mc.stageCount["discover"] != 1 {
		t.Error("expected discover stage count")
	}
	if len(mc.stageDurations["clone"]) != 1 {
		t.Error("expected clone duration recorded")
	}
}

func TestRecordStorageOperation(t *testing.T) {
	mc := NewMetricsCollector()

	mc.RecordStorageOperation("put", 1024)
	mc.RecordStorageOperation("get", 512)
	mc.RecordStorageOperation("put", 2048)

	if mc.storageOperations["put"] != 2 {
		t.Error("expected 2 put operations")
	}
	if mc.storageOperations["get"] != 1 {
		t.Error("expected 1 get operation")
	}
	if mc.storageSize != 3584 {
		t.Errorf("expected total size 3584, got %d", mc.storageSize)
	}
}

func TestRecordDLQEvent(t *testing.T) {
	mc := NewMetricsCollector()

	mc.RecordDLQEvent()
	mc.RecordDLQEvent()
	mc.RecordDLQEvent()

	if mc.dlqSize != 3 {
		t.Errorf("expected dlqSize=3, got %d", mc.dlqSize)
	}
}

func TestRemoveDLQEvent(t *testing.T) {
	mc := NewMetricsCollector()

	mc.RecordDLQEvent()
	mc.RecordDLQEvent()
	mc.RecordDLQEvent()
	mc.RemoveDLQEvent(2)

	if mc.dlqSize != 1 {
		t.Errorf("expected dlqSize=1, got %d", mc.dlqSize)
	}
}

func TestRemoveDLQEventUnderflow(t *testing.T) {
	mc := NewMetricsCollector()

	mc.RecordDLQEvent()
	mc.RemoveDLQEvent(5) // More than exists

	if mc.dlqSize != 0 {
		t.Error("expected dlqSize=0 (should not go negative)")
	}
}

func TestGetSnapshot(t *testing.T) {
	mc := NewMetricsCollector()

	mc.RecordBuildStart("build-1", "tenant-1")
	mc.RecordBuildStart("build-2", "tenant-2")
	mc.RecordCacheHit("test")
	mc.RecordCacheHit("test")
	mc.RecordCacheMiss("test")
	mc.RecordStorageOperation("put", 1024)

	snapshot := mc.GetSnapshot()

	if snapshot.TotalBuilds != 2 {
		t.Errorf("expected 2 builds in snapshot, got %d", snapshot.TotalBuilds)
	}
	if snapshot.CurrentConcurrent != 2 {
		t.Errorf("expected 2 concurrent, got %d", snapshot.CurrentConcurrent)
	}
	if snapshot.CacheHits != 2 {
		t.Errorf("expected 2 cache hits, got %d", snapshot.CacheHits)
	}
	if snapshot.CacheMisses != 1 {
		t.Errorf("expected 1 cache miss, got %d", snapshot.CacheMisses)
	}
}

func TestCacheHitRate(t *testing.T) {
	mc := NewMetricsCollector()

	mc.RecordCacheHit("test")
	mc.RecordCacheHit("test")
	mc.RecordCacheMiss("test")
	mc.RecordCacheMiss("test")

	snapshot := mc.GetSnapshot()

	expected := 0.5
	if snapshot.CacheHitRate < expected-0.01 || snapshot.CacheHitRate > expected+0.01 {
		t.Errorf("expected hit rate %.2f, got %.2f", expected, snapshot.CacheHitRate)
	}
}

func TestAverageBuildDuration(t *testing.T) {
	mc := NewMetricsCollector()

	mc.buildDurations = []time.Duration{
		100 * time.Millisecond,
		200 * time.Millisecond,
		300 * time.Millisecond,
	}

	snapshot := mc.GetSnapshot()

	expected := 200 * time.Millisecond
	if snapshot.AvgBuildDuration != expected {
		t.Errorf("expected avg duration %v, got %v", expected, snapshot.AvgBuildDuration)
	}
}

func TestPercentileBuildDuration(t *testing.T) {
	mc := NewMetricsCollector()

	// Create 100 durations to test percentiles properly
	for i := 1; i <= 100; i++ {
		mc.buildDurations = append(mc.buildDurations, time.Duration(i)*time.Millisecond)
	}

	snapshot := mc.GetSnapshot()

	// P50 should be around 50ms
	if snapshot.P50BuildDuration < 45*time.Millisecond || snapshot.P50BuildDuration > 55*time.Millisecond {
		t.Errorf("expected P50 around 50ms, got %v", snapshot.P50BuildDuration)
	}

	// P95 should be around 95ms
	if snapshot.P95BuildDuration < 90*time.Millisecond || snapshot.P95BuildDuration > 100*time.Millisecond {
		t.Errorf("expected P95 around 95ms, got %v", snapshot.P95BuildDuration)
	}
}

func TestEmptySnapshot(t *testing.T) {
	mc := NewMetricsCollector()

	snapshot := mc.GetSnapshot()

	if snapshot.TotalBuilds != 0 {
		t.Error("expected 0 builds in empty snapshot")
	}
	if snapshot.CacheHitRate != 0 {
		t.Error("expected 0 cache hit rate when no operations")
	}
}

func TestFormatMetrics(t *testing.T) {
	mc := NewMetricsCollector()

	mc.RecordBuildStart("build-1", "tenant-1")
	mc.RecordBuildEnd(100*time.Millisecond, true, "tenant-1")
	mc.RecordCacheHit("test")
	mc.RecordCacheMiss("test")

	snapshot := mc.GetSnapshot()
	formatted := snapshot.FormatMetrics()

	if len(formatted) == 0 {
		t.Error("expected non-empty formatted metrics")
	}

	// Check for expected content
	if !contains(formatted, "DocBuilder Metrics") {
		t.Error("expected 'DocBuilder Metrics' in output")
	}
	if !contains(formatted, "Total Builds: 1") {
		t.Error("expected build count in output")
	}
	if !contains(formatted, "Cache") {
		t.Error("expected cache metrics in output")
	}
}

func TestConcurrentMetrics(t *testing.T) {
	mc := NewMetricsCollector()

	// Simulate concurrent builds
	mc.RecordBuildStart("build-1", "tenant-1")
	mc.RecordBuildStart("build-2", "tenant-1")
	mc.RecordBuildStart("build-3", "tenant-2")

	if mc.currentConcurrent != 3 {
		t.Errorf("expected 3 concurrent, got %d", mc.currentConcurrent)
	}

	// End some builds
	mc.RecordBuildEnd(100*time.Millisecond, true, "tenant-1")
	mc.RecordBuildEnd(150*time.Millisecond, true, "tenant-1")

	if mc.currentConcurrent != 1 {
		t.Errorf("expected 1 concurrent, got %d", mc.currentConcurrent)
	}
}

func TestMultipleStageMetrics(t *testing.T) {
	mc := NewMetricsCollector()

	stages := []string{"clone", "discover", "transform", "generate"}
	for _, stage := range stages {
		for i := 0; i < 3; i++ {
			mc.RecordStage(stage, time.Duration(i+1)*50*time.Millisecond, true)
		}
	}

	snapshot := mc.GetSnapshot()

	for _, stage := range stages {
		if count, ok := snapshot.StageCount[stage]; !ok || count != 3 {
			t.Errorf("expected stage %s to have count 3, got %d", stage, count)
		}
	}
}

func TestActiveTenants(t *testing.T) {
	mc := NewMetricsCollector()

	tenants := []string{"tenant-1", "tenant-2", "tenant-3"}
	for _, tenant := range tenants {
		mc.RecordBuildStart("build", tenant)
	}

	if len(mc.activeTenants) != 3 {
		t.Errorf("expected 3 active tenants, got %d", len(mc.activeTenants))
	}

	// End builds for some tenants
	mc.RecordBuildEnd(100*time.Millisecond, true, "tenant-1")
	mc.RecordBuildEnd(100*time.Millisecond, true, "tenant-2")

	if mc.activeTenants["tenant-1"] != 0 {
		t.Error("expected tenant-1 active count to be 0")
	}
	if mc.activeTenants["tenant-3"] != 1 {
		t.Error("expected tenant-3 active count to be 1")
	}
}

func TestInitGlobalMetricsCollector(t *testing.T) {
	ResetMetricsCollector()

	mc := InitMetricsCollector()
	if mc == nil {
		t.Fatal("expected MetricsCollector")
	}

	mc2 := InitMetricsCollector()
	if mc != mc2 {
		t.Error("expected same instance on second call")
	}

	ResetMetricsCollector()
}

func TestGetGlobalMetricsCollector(t *testing.T) {
	ResetMetricsCollector()

	mc := GetMetricsCollector()
	if mc == nil {
		t.Fatal("expected MetricsCollector")
	}

	mc2 := GetMetricsCollector()
	if mc != mc2 {
		t.Error("expected same instance")
	}

	ResetMetricsCollector()
}

func TestSetGlobalMetricsCollector(t *testing.T) {
	mc := NewMetricsCollector()
	SetMetricsCollector(mc)

	retrieved := GetMetricsCollector()
	if retrieved != mc {
		t.Error("expected same metrics collector")
	}

	ResetMetricsCollector()
}

func TestMetricsThreadSafety(t *testing.T) {
	mc := NewMetricsCollector()
	done := make(chan bool)

	// Concurrent writers
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 10; j++ {
				mc.RecordBuildStart("build", "tenant")
				mc.RecordCacheHit("test")
				mc.RecordStage("clone", 10*time.Millisecond, true)
				mc.RecordStorageOperation("put", 100)
			}
			done <- true
		}(i)
	}

	// Concurrent readers
	for i := 0; i < 5; i++ {
		go func() {
			for j := 0; j < 20; j++ {
				_ = mc.GetSnapshot()
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 15; i++ {
		<-done
	}

	snapshot := mc.GetSnapshot()
	if snapshot.TotalBuilds != 100 {
		t.Errorf("expected 100 builds, got %d", snapshot.TotalBuilds)
	}
	if snapshot.CacheHits != 100 {
		t.Errorf("expected 100 cache hits, got %d", snapshot.CacheHits)
	}
}

// Helper function
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
