package load

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// LoadScenario represents a load testing scenario configuration
type LoadScenario struct {
	Name               string
	ConcurrentRequests int
	TotalRequests      int
	CacheHitRate       float64 // 0.0 to 1.0
	RequestDuration    time.Duration
	RampUpDuration     time.Duration
}

// LoadResult captures metrics from a load test
type LoadResult struct {
	ScenarioName      string
	TotalRequests     int64
	SuccessfulRequest int64
	FailedRequests    int64
	TotalErrors       int
	StartTime         time.Time
	EndTime           time.Time
	Duration          time.Duration

	// Latency metrics (in milliseconds)
	Latencies  []int64
	MinLatency int64
	MaxLatency int64
	AvgLatency int64
	P50Latency int64
	P95Latency int64
	P99Latency int64

	// Throughput metrics
	RequestsPerSec float64
	CacheHits      int64
	CacheMisses    int64
	CacheHitRate   float64
}

// LoadTester manages load test execution
type LoadTester struct {
	mu      sync.Mutex
	results []*LoadResult
}

// NewLoadTester creates a new load tester
func NewLoadTester() *LoadTester {
	return &LoadTester{
		results: make([]*LoadResult, 0),
	}
}

// ExecuteScenario runs a load testing scenario
func (lt *LoadTester) ExecuteScenario(
	ctx context.Context,
	scenario LoadScenario,
	handler func(ctx context.Context) error,
) *LoadResult {
	result := &LoadResult{
		ScenarioName:  scenario.Name,
		TotalRequests: int64(scenario.TotalRequests),
		StartTime:     time.Now(),
		Latencies:     make([]int64, 0, scenario.TotalRequests),
		CacheHitRate:  scenario.CacheHitRate,
	}

	var successfulCount int64
	var failedCount int64
	var cacheHits int64
	var latenciesMu sync.Mutex

	// Calculate ramp-up increment
	rampUpIncrement := scenario.RampUpDuration / time.Duration(scenario.ConcurrentRequests)

	// Create worker pool with ramp-up
	requestsChan := make(chan int, scenario.TotalRequests)
	var wg sync.WaitGroup

	// Spawn workers gradually
	for i := 0; i < scenario.ConcurrentRequests; i++ {
		wg.Add(1)
		time.Sleep(rampUpIncrement)
		go func() {
			defer wg.Done()
			for range requestsChan {
				select {
				case <-ctx.Done():
					return
				default:
				}

				// Determine if this is a cache hit
				isCacheHit := scenario.CacheHitRate > 0 &&
					time.Now().UnixNano()%100 < int64(scenario.CacheHitRate*100)

				start := time.Now()
				err := handler(ctx)
				latency := time.Since(start).Milliseconds()

				latenciesMu.Lock()
				result.Latencies = append(result.Latencies, latency)
				latenciesMu.Unlock()

				if err == nil {
					atomic.AddInt64(&successfulCount, 1)
					if isCacheHit {
						atomic.AddInt64(&cacheHits, 1)
					}
				} else {
					atomic.AddInt64(&failedCount, 1)
					result.TotalErrors++
				}
			}
		}()
	}

	// Feed requests
	go func() {
		for i := 0; i < scenario.TotalRequests; i++ {
			select {
			case <-ctx.Done():
				close(requestsChan)
				return
			case requestsChan <- i:
			}
		}
		close(requestsChan)
	}()

	wg.Wait()

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)
	result.SuccessfulRequest = atomic.LoadInt64(&successfulCount)
	result.FailedRequests = atomic.LoadInt64(&failedCount)
	result.CacheHits = atomic.LoadInt64(&cacheHits)
	result.CacheMisses = result.SuccessfulRequest - result.CacheHits

	// Calculate latency statistics
	if len(result.Latencies) > 0 {
		sort.Slice(result.Latencies, func(i, j int) bool {
			return result.Latencies[i] < result.Latencies[j]
		})

		result.MinLatency = result.Latencies[0]
		result.MaxLatency = result.Latencies[len(result.Latencies)-1]

		// Calculate average
		var sum int64
		for _, lat := range result.Latencies {
			sum += lat
		}
		result.AvgLatency = sum / int64(len(result.Latencies))

		// Calculate percentiles
		result.P50Latency = result.Latencies[int(float64(len(result.Latencies))*0.50)]
		result.P95Latency = result.Latencies[int(float64(len(result.Latencies))*0.95)]
		result.P99Latency = result.Latencies[int(float64(len(result.Latencies))*0.99)]
	}

	// Calculate throughput
	result.RequestsPerSec = float64(result.SuccessfulRequest) / result.Duration.Seconds()

	lt.mu.Lock()
	lt.results = append(lt.results, result)
	lt.mu.Unlock()

	return result
}

// GetResults returns all collected results
func (lt *LoadTester) GetResults() []*LoadResult {
	lt.mu.Lock()
	defer lt.mu.Unlock()
	return lt.results
}

// Summary returns a formatted summary of all results
func (lt *LoadTester) Summary() string {
	lt.mu.Lock()
	results := lt.results
	lt.mu.Unlock()

	summary := "Load Test Results Summary\n"
	summary += "=" + fmt.Sprintf("%*s", 79, "") + "\n\n"

	for _, r := range results {
		summary += fmt.Sprintf("Scenario: %s\n", r.ScenarioName)
		summary += fmt.Sprintf("  Duration:       %v\n", r.Duration)
		summary += fmt.Sprintf("  Total:          %d\n", r.TotalRequests)
		summary += fmt.Sprintf("  Successful:     %d (%.2f%%)\n",
			r.SuccessfulRequest,
			float64(r.SuccessfulRequest)/float64(r.TotalRequests)*100)
		summary += fmt.Sprintf("  Failed:         %d (%.2f%%)\n",
			r.FailedRequests,
			float64(r.FailedRequests)/float64(r.TotalRequests)*100)
		summary += fmt.Sprintf("  Throughput:     %.2f req/sec\n", r.RequestsPerSec)
		summary += fmt.Sprintf("  Cache Hits:     %d (%.2f%%)\n",
			r.CacheHits,
			float64(r.CacheHits)/float64(r.SuccessfulRequest)*100)
		summary += "  Latency (ms):\n"
		summary += fmt.Sprintf("    Min:          %d\n", r.MinLatency)
		summary += fmt.Sprintf("    Avg:          %d\n", r.AvgLatency)
		summary += fmt.Sprintf("    P50:          %d\n", r.P50Latency)
		summary += fmt.Sprintf("    P95:          %d\n", r.P95Latency)
		summary += fmt.Sprintf("    P99:          %d\n", r.P99Latency)
		summary += fmt.Sprintf("    Max:          %d\n", r.MaxLatency)
		summary += "\n"
	}

	return summary
}

// CommonScenarios returns predefined load testing scenarios
func CommonScenarios() []LoadScenario {
	return []LoadScenario{
		{
			Name:               "Light Load",
			ConcurrentRequests: 10,
			TotalRequests:      100,
			CacheHitRate:       0.5,
			RampUpDuration:     100 * time.Millisecond,
		},
		{
			Name:               "Medium Load",
			ConcurrentRequests: 50,
			TotalRequests:      500,
			CacheHitRate:       0.5,
			RampUpDuration:     500 * time.Millisecond,
		},
		{
			Name:               "Heavy Load",
			ConcurrentRequests: 100,
			TotalRequests:      1000,
			CacheHitRate:       0.5,
			RampUpDuration:     1000 * time.Millisecond,
		},
		{
			Name:               "High Cache Hit",
			ConcurrentRequests: 50,
			TotalRequests:      500,
			CacheHitRate:       0.8,
			RampUpDuration:     500 * time.Millisecond,
		},
		{
			Name:               "No Cache",
			ConcurrentRequests: 25,
			TotalRequests:      250,
			CacheHitRate:       0.0,
			RampUpDuration:     250 * time.Millisecond,
		},
	}
}
