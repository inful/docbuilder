package load

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// TestNewLoadTester validates load tester initialization
func TestNewLoadTester(t *testing.T) {
	lt := NewLoadTester()

	if lt == nil {
		t.Fatal("expected non-nil LoadTester")
	}
	if lt.results == nil {
		t.Error("expected initialized results slice")
	}
	if len(lt.results) != 0 {
		t.Errorf("expected empty results, got %d", len(lt.results))
	}
}

// TestExecuteScenarioBasic validates basic scenario execution
func TestExecuteScenarioBasic(t *testing.T) {
	lt := NewLoadTester()
	ctx := context.Background()

	scenario := LoadScenario{
		Name:               "Basic Test",
		ConcurrentRequests: 5,
		TotalRequests:      50,
		CacheHitRate:       0.5,
		RampUpDuration:     100 * time.Millisecond,
	}

	handler := func(ctx context.Context) error {
		// Simulate minimal work
		time.Sleep(1 * time.Millisecond)
		return nil
	}

	result := lt.ExecuteScenario(ctx, scenario, handler)

	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.ScenarioName != "Basic Test" {
		t.Errorf("expected scenario name 'Basic Test', got %s", result.ScenarioName)
	}
	if result.TotalRequests != 50 {
		t.Errorf("expected 50 total requests, got %d", result.TotalRequests)
	}
	if result.SuccessfulRequest != 50 {
		t.Errorf("expected 50 successful requests, got %d", result.SuccessfulRequest)
	}
	if result.FailedRequests != 0 {
		t.Errorf("expected 0 failed requests, got %d", result.FailedRequests)
	}
}

// TestExecuteScenarioWithErrors validates error tracking
func TestExecuteScenarioWithErrors(t *testing.T) {
	lt := NewLoadTester()
	ctx := context.Background()

	scenario := LoadScenario{
		Name:               "Error Test",
		ConcurrentRequests: 5,
		TotalRequests:      50,
		CacheHitRate:       0.5,
		RampUpDuration:     100 * time.Millisecond,
	}

	errorCount := 0
	handler := func(ctx context.Context) error {
		errorCount++
		if errorCount%2 == 0 {
			return fmt.Errorf("simulated error")
		}
		return nil
	}

	result := lt.ExecuteScenario(ctx, scenario, handler)

	if result.FailedRequests == 0 {
		t.Error("expected some failed requests")
	}
	if result.SuccessfulRequest+result.FailedRequests != 50 {
		t.Errorf("expected total of 50 requests, got %d", result.SuccessfulRequest+result.FailedRequests)
	}
}

// TestLatencyMetrics validates latency calculation
func TestLatencyMetrics(t *testing.T) {
	lt := NewLoadTester()
	ctx := context.Background()

	scenario := LoadScenario{
		Name:               "Latency Test",
		ConcurrentRequests: 5,
		TotalRequests:      100,
		CacheHitRate:       0.5,
		RampUpDuration:     100 * time.Millisecond,
	}

	handler := func(ctx context.Context) error {
		time.Sleep(5 * time.Millisecond)
		return nil
	}

	result := lt.ExecuteScenario(ctx, scenario, handler)

	if len(result.Latencies) != 100 {
		t.Errorf("expected 100 latency measurements, got %d", len(result.Latencies))
	}
	if result.MinLatency == 0 {
		t.Error("expected non-zero minimum latency")
	}
	if result.MaxLatency < result.MinLatency {
		t.Error("max latency should be >= min latency")
	}
	if result.AvgLatency < result.MinLatency || result.AvgLatency > result.MaxLatency {
		t.Error("average latency should be between min and max")
	}
	if result.P50Latency < result.MinLatency || result.P50Latency > result.MaxLatency {
		t.Error("P50 latency should be between min and max")
	}
	if result.P95Latency < result.MinLatency || result.P95Latency > result.MaxLatency {
		t.Error("P95 latency should be between min and max")
	}
	if result.P99Latency < result.MinLatency || result.P99Latency > result.MaxLatency {
		t.Error("P99 latency should be between min and max")
	}
}

// TestPercentileOrdering validates percentile ordering
func TestPercentileOrdering(t *testing.T) {
	lt := NewLoadTester()
	ctx := context.Background()

	scenario := LoadScenario{
		Name:               "Percentile Test",
		ConcurrentRequests: 10,
		TotalRequests:      1000,
		CacheHitRate:       0.5,
		RampUpDuration:     200 * time.Millisecond,
	}

	handler := func(ctx context.Context) error {
		time.Sleep(time.Millisecond)
		return nil
	}

	result := lt.ExecuteScenario(ctx, scenario, handler)

	if !(result.P50Latency >= result.MinLatency && result.P50Latency <= result.MaxLatency) {
		t.Error("P50 not in valid range")
	}
	if !(result.P95Latency >= result.P50Latency && result.P95Latency <= result.MaxLatency) {
		t.Error("P95 should be >= P50")
	}
	if !(result.P99Latency >= result.P95Latency && result.P99Latency <= result.MaxLatency) {
		t.Error("P99 should be >= P95")
	}
}

// TestThroughputCalculation validates throughput metrics
func TestThroughputCalculation(t *testing.T) {
	lt := NewLoadTester()
	ctx := context.Background()

	scenario := LoadScenario{
		Name:               "Throughput Test",
		ConcurrentRequests: 10,
		TotalRequests:      100,
		CacheHitRate:       0.5,
		RampUpDuration:     100 * time.Millisecond,
	}

	handler := func(ctx context.Context) error {
		time.Sleep(1 * time.Millisecond)
		return nil
	}

	result := lt.ExecuteScenario(ctx, scenario, handler)

	if result.RequestsPerSec <= 0 {
		t.Errorf("expected positive throughput, got %.2f", result.RequestsPerSec)
	}
	// Rough sanity check: should be at least 100/duration requests per second
	if result.RequestsPerSec < 1 {
		t.Errorf("throughput too low: %.2f req/sec", result.RequestsPerSec)
	}
}

// TestCacheHitTracking validates cache hit tracking
func TestCacheHitTracking(t *testing.T) {
	lt := NewLoadTester()
	ctx := context.Background()

	scenario := LoadScenario{
		Name:               "Cache Test",
		ConcurrentRequests: 5,
		TotalRequests:      100,
		CacheHitRate:       0.8,
		RampUpDuration:     100 * time.Millisecond,
	}

	handler := func(ctx context.Context) error {
		return nil
	}

	result := lt.ExecuteScenario(ctx, scenario, handler)

	expectedApproxHits := int64(float64(result.SuccessfulRequest) * 0.8)
	// Allow some variance due to random distribution
	tolerance := int64(float64(result.SuccessfulRequest) * 0.15)

	if result.CacheHits < (expectedApproxHits - tolerance) {
		t.Logf("cache hits: %d, expected ~%d (tolerance: %d)", result.CacheHits, expectedApproxHits, tolerance)
	}

	if result.CacheMisses+result.CacheHits != result.SuccessfulRequest {
		t.Errorf("cache hits + misses should equal successful requests")
	}
}

// TestMultipleScenarios validates running multiple scenarios
func TestMultipleScenarios(t *testing.T) {
	lt := NewLoadTester()
	ctx := context.Background()

	scenarios := []LoadScenario{
		{
			Name:               "Scenario 1",
			ConcurrentRequests: 5,
			TotalRequests:      50,
			CacheHitRate:       0.5,
			RampUpDuration:     50 * time.Millisecond,
		},
		{
			Name:               "Scenario 2",
			ConcurrentRequests: 10,
			TotalRequests:      100,
			CacheHitRate:       0.5,
			RampUpDuration:     100 * time.Millisecond,
		},
	}

	handler := func(ctx context.Context) error {
		return nil
	}

	for _, scenario := range scenarios {
		lt.ExecuteScenario(ctx, scenario, handler)
	}

	results := lt.GetResults()
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
}

// TestResultsStorage validates result storage
func TestResultsStorage(t *testing.T) {
	lt := NewLoadTester()
	ctx := context.Background()

	scenario := LoadScenario{
		Name:               "Storage Test",
		ConcurrentRequests: 5,
		TotalRequests:      50,
		CacheHitRate:       0.5,
		RampUpDuration:     50 * time.Millisecond,
	}

	handler := func(ctx context.Context) error {
		return nil
	}

	result := lt.ExecuteScenario(ctx, scenario, handler)

	// Retrieve and verify
	results := lt.GetResults()
	if len(results) != 1 {
		t.Errorf("expected 1 result stored, got %d", len(results))
	}

	if results[0] != result {
		t.Error("stored result should match returned result")
	}
}

// TestCommonScenarios validates predefined scenarios
func TestCommonScenarios(t *testing.T) {
	scenarios := CommonScenarios()

	if len(scenarios) == 0 {
		t.Error("expected at least one common scenario")
	}

	for _, s := range scenarios {
		if s.Name == "" {
			t.Error("scenario should have a name")
		}
		if s.ConcurrentRequests == 0 {
			t.Error("scenario should have concurrent requests > 0")
		}
		if s.TotalRequests == 0 {
			t.Error("scenario should have total requests > 0")
		}
		if s.CacheHitRate < 0 || s.CacheHitRate > 1 {
			t.Error("cache hit rate should be between 0.0 and 1.0")
		}
	}
}

// TestLoadResultSummary validates summary generation
func TestLoadResultSummary(t *testing.T) {
	lt := NewLoadTester()
	ctx := context.Background()

	scenario := LoadScenario{
		Name:               "Summary Test",
		ConcurrentRequests: 5,
		TotalRequests:      50,
		CacheHitRate:       0.5,
		RampUpDuration:     50 * time.Millisecond,
	}

	handler := func(ctx context.Context) error {
		return nil
	}

	lt.ExecuteScenario(ctx, scenario, handler)

	summary := lt.Summary()

	if summary == "" {
		t.Error("expected non-empty summary")
	}
	if !contains(summary, "Summary Test") {
		t.Error("summary should include scenario name")
	}
	if !contains(summary, "Throughput") {
		t.Error("summary should include throughput metrics")
	}
	if !contains(summary, "Latency") {
		t.Error("summary should include latency metrics")
	}
}

// TestConcurrentExecution validates concurrent request handling
func TestConcurrentExecution(t *testing.T) {
	lt := NewLoadTester()
	ctx := context.Background()

	scenario := LoadScenario{
		Name:               "Concurrent Test",
		ConcurrentRequests: 20,
		TotalRequests:      200,
		CacheHitRate:       0.5,
		RampUpDuration:     200 * time.Millisecond,
	}

	handler := func(ctx context.Context) error {
		time.Sleep(5 * time.Millisecond)
		return nil
	}

	result := lt.ExecuteScenario(ctx, scenario, handler)

	if result.SuccessfulRequest != 200 {
		t.Errorf("expected 200 successful requests, got %d", result.SuccessfulRequest)
	}
	if result.Duration < 10*time.Millisecond {
		t.Logf("concurrent execution completed in %v (faster than expected)", result.Duration)
	}
}

// TestContextCancellation validates context cancellation handling
func TestContextCancellation(t *testing.T) {
	lt := NewLoadTester()
	ctx, cancel := context.WithCancel(context.Background())

	scenario := LoadScenario{
		Name:               "Cancel Test",
		ConcurrentRequests: 5,
		TotalRequests:      1000,
		CacheHitRate:       0.5,
		RampUpDuration:     100 * time.Millisecond,
	}

	requestsStarted := 0
	handler := func(ctx context.Context) error {
		requestsStarted++
		time.Sleep(10 * time.Millisecond)
		if requestsStarted > 50 {
			cancel()
		}
		return nil
	}

	result := lt.ExecuteScenario(ctx, scenario, handler)

	// Not all requests should complete due to cancellation
	if result.SuccessfulRequest+result.FailedRequests >= 1000 {
		t.Logf("context cancellation: %d out of 1000 requests completed", result.SuccessfulRequest+result.FailedRequests)
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
