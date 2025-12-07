package observability

import (
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"
)

// MetricsCollector tracks application metrics.
type MetricsCollector struct {
	mu sync.RWMutex

	// Build metrics
	buildCount        int64           // Total builds started
	buildDurations    []time.Duration // Individual build durations (for percentiles)
	buildErrors       int64           // Total build failures
	buildsByStatus    map[string]int64
	currentConcurrent int64

	// Cache metrics
	cacheHits   int64
	cacheMisses int64

	// Stage metrics
	stageCount     map[string]int64
	stageDurations map[string][]time.Duration

	// Storage metrics
	storageOperations map[string]int64 // operation -> count
	storageSize       int64

	// DLQ metrics
	dlqSize int64

	// Tenant metrics
	activeTenants map[string]int64
}

// NewMetricsCollector creates a new metrics collector.
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		buildsByStatus:    make(map[string]int64),
		stageCount:        make(map[string]int64),
		stageDurations:    make(map[string][]time.Duration),
		storageOperations: make(map[string]int64),
		activeTenants:     make(map[string]int64),
	}
}

// RecordBuildStart records the start of a build.
func (mc *MetricsCollector) RecordBuildStart(buildID, tenantID string) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.buildCount++
	mc.currentConcurrent++
	mc.buildsByStatus["started"]++
	mc.activeTenants[tenantID]++

	slog.Debug("Build started", "build.count", mc.buildCount, "concurrent", mc.currentConcurrent)
}

// RecordBuildEnd records the end of a build.
func (mc *MetricsCollector) RecordBuildEnd(duration time.Duration, success bool, tenantID string) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.buildDurations = append(mc.buildDurations, duration)
	mc.currentConcurrent--

	if success {
		mc.buildsByStatus["completed"]++
		slog.Debug("Build completed", "duration_ms", duration.Milliseconds())
	} else {
		mc.buildErrors++
		mc.buildsByStatus["failed"]++
		slog.Debug("Build failed", "duration_ms", duration.Milliseconds())
	}

	if mc.activeTenants[tenantID] > 0 {
		mc.activeTenants[tenantID]--
	}
}

// RecordCacheHit records a cache hit.
func (mc *MetricsCollector) RecordCacheHit(cacheType string) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.cacheHits++
	slog.Debug("Cache hit", "type", cacheType, "total_hits", mc.cacheHits)
}

// RecordCacheMiss records a cache miss.
func (mc *MetricsCollector) RecordCacheMiss(cacheType string) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.cacheMisses++
	slog.Debug("Cache miss", "type", cacheType, "total_misses", mc.cacheMisses)
}

// RecordStage records a stage execution.
func (mc *MetricsCollector) RecordStage(stageName string, duration time.Duration, success bool) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.stageCount[stageName]++
	mc.stageDurations[stageName] = append(mc.stageDurations[stageName], duration)

	if !success {
		mc.buildErrors++
	}

	slog.Debug("Stage completed", "stage", stageName, "duration_ms", duration.Milliseconds())
}

// RecordStorageOperation records a storage operation.
func (mc *MetricsCollector) RecordStorageOperation(operation string, sizeBytes int64) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.storageOperations[operation]++
	mc.storageSize += sizeBytes

	slog.Debug("Storage operation", "operation", operation, "size_bytes", sizeBytes)
}

// RecordDLQEvent records an event added to the dead letter queue.
func (mc *MetricsCollector) RecordDLQEvent() {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.dlqSize++
	slog.Debug("Event added to DLQ", "dlq_size", mc.dlqSize)
}

// RemoveDLQEvent records removal of an event from DLQ.
func (mc *MetricsCollector) RemoveDLQEvent(count int64) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.dlqSize -= count
	if mc.dlqSize < 0 {
		mc.dlqSize = 0
	}

	slog.Debug("Events removed from DLQ", "count", count, "dlq_size", mc.dlqSize)
}

// GetSnapshot returns a snapshot of current metrics.
func (mc *MetricsCollector) GetSnapshot() MetricsSnapshot {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	snapshot := MetricsSnapshot{
		Timestamp:         time.Now(),
		TotalBuilds:       mc.buildCount,
		CurrentConcurrent: mc.currentConcurrent,
		BuildErrors:       mc.buildErrors,
		BuildsByStatus:    copyStringInt64Map(mc.buildsByStatus),
		CacheHits:         mc.cacheHits,
		CacheMisses:       mc.cacheMisses,
		CacheHitRate:      calculateHitRate(mc.cacheHits, mc.cacheMisses),
		StageCount:        copyStringInt64Map(mc.stageCount),
		StorageOperations: copyStringInt64Map(mc.storageOperations),
		StorageSizeBytes:  mc.storageSize,
		DLQSize:           mc.dlqSize,
		ActiveTenants:     len(mc.activeTenants),
	}

	// Calculate percentiles
	if len(mc.buildDurations) > 0 {
		snapshot.P50BuildDuration = calculatePercentile(mc.buildDurations, 50)
		snapshot.P95BuildDuration = calculatePercentile(mc.buildDurations, 95)
		snapshot.P99BuildDuration = calculatePercentile(mc.buildDurations, 99)
		snapshot.AvgBuildDuration = calculateAverage(mc.buildDurations)
	}

	return snapshot
}

// MetricsSnapshot represents a point-in-time snapshot of metrics.
type MetricsSnapshot struct {
	Timestamp         time.Time
	TotalBuilds       int64
	CurrentConcurrent int64
	BuildErrors       int64
	BuildsByStatus    map[string]int64
	CacheHits         int64
	CacheMisses       int64
	CacheHitRate      float64
	P50BuildDuration  time.Duration
	P95BuildDuration  time.Duration
	P99BuildDuration  time.Duration
	AvgBuildDuration  time.Duration
	StageCount        map[string]int64
	StorageOperations map[string]int64
	StorageSizeBytes  int64
	DLQSize           int64
	ActiveTenants     int
}

// FormatMetrics returns a human-readable string of metrics.
func (s MetricsSnapshot) FormatMetrics() string {
	cacheTotal := s.CacheHits + s.CacheMisses
	successRate := 0.0
	if s.TotalBuilds > 0 {
		successRate = float64(s.TotalBuilds-s.BuildErrors) / float64(s.TotalBuilds) * 100
	}

	output := fmt.Sprintf(`
=== DocBuilder Metrics ===
Timestamp: %s

Build Metrics:
  Total Builds: %d
  Current Concurrent: %d
  Build Errors: %d (%.2f%% error rate)
  Success Rate: %.2f%%

Build Durations:
  Average: %v
  P50: %v
  P95: %v
  P99: %v

Cache Metrics:
  Cache Hits: %d
  Cache Misses: %d
  Total Cache Ops: %d
  Hit Rate: %.2f%%

Stage Metrics: %d stages tracked
  Details: %v

Storage Metrics:
  Total Operations: %d
  Total Size: %d bytes (%.2f MB)
  Operations by Type: %v

Queue Metrics:
  DLQ Size: %d

Tenant Metrics:
  Active Tenants: %d

Status Breakdown: %v
======================
`,
		s.Timestamp.Format(time.RFC3339),
		s.TotalBuilds,
		s.CurrentConcurrent,
		s.BuildErrors,
		float64(s.BuildErrors)/float64(s.TotalBuilds+1)*100,
		successRate,
		s.AvgBuildDuration,
		s.P50BuildDuration,
		s.P95BuildDuration,
		s.P99BuildDuration,
		s.CacheHits,
		s.CacheMisses,
		cacheTotal,
		s.CacheHitRate*100,
		len(s.StageCount),
		s.StageCount,
		sumInt64Values(s.StorageOperations),
		s.StorageSizeBytes,
		float64(s.StorageSizeBytes)/(1024*1024),
		s.StorageOperations,
		s.DLQSize,
		s.ActiveTenants,
		s.BuildsByStatus,
	)

	return output
}

// Helper functions

func copyStringInt64Map(m map[string]int64) map[string]int64 {
	result := make(map[string]int64)
	for k, v := range m {
		result[k] = v
	}
	return result
}

func calculateHitRate(hits, misses int64) float64 {
	total := hits + misses
	if total == 0 {
		return 0
	}
	return float64(hits) / float64(total)
}

func calculateAverage(durations []time.Duration) time.Duration {
	if len(durations) == 0 {
		return 0
	}

	var total time.Duration
	for _, d := range durations {
		total += d
	}
	return total / time.Duration(len(durations))
}

func calculatePercentile(durations []time.Duration, percentile int) time.Duration {
	if len(durations) == 0 {
		return 0
	}

	// Sort durations for accurate percentile calculation
	sorted := make([]time.Duration, len(durations))
	copy(sorted, durations)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

	// Calculate index
	index := (len(sorted) * percentile) / 100
	if index >= len(sorted) {
		index = len(sorted) - 1
	}

	return sorted[index]
}

func sumInt64Values(m map[string]int64) int64 {
	var sum int64
	for _, v := range m {
		sum += v
	}
	return sum
}

// GlobalMetricsCollector holds the singleton metrics collector.
var globalMetricsCollector *MetricsCollector

// InitMetricsCollector initializes the global metrics collector.
func InitMetricsCollector() *MetricsCollector {
	if globalMetricsCollector == nil {
		globalMetricsCollector = NewMetricsCollector()
	}
	return globalMetricsCollector
}

// GetMetricsCollector returns the global metrics collector.
func GetMetricsCollector() *MetricsCollector {
	if globalMetricsCollector == nil {
		return InitMetricsCollector()
	}
	return globalMetricsCollector
}

// SetMetricsCollector sets the global metrics collector (for testing).
func SetMetricsCollector(mc *MetricsCollector) {
	globalMetricsCollector = mc
}

// ResetMetricsCollector resets the global metrics collector (for testing).
func ResetMetricsCollector() {
	globalMetricsCollector = nil
}
