package daemon

import "testing"

// TestHistogramKeepsLastValues proves the histogram keeps the last values
// only, and that the summary still describes them. The buffer used to be
// shifted on every record once it was full.
func TestHistogramKeepsLastValues(t *testing.T) {
	h := &Histogram{}
	for i := range maxHistogramValues + 50 {
		h.Record(float64(i))
	}

	stats := h.GetStats()
	if stats.Count != int64(maxHistogramValues+50) {
		t.Fatalf("count = %d, want %d", stats.Count, maxHistogramValues+50)
	}
	if stats.Min != 50 {
		t.Fatalf("min = %v, want 50 (the oldest values must be dropped)", stats.Min)
	}
	if stats.Max != float64(maxHistogramValues+49) {
		t.Fatalf("max = %v, want %v", stats.Max, maxHistogramValues+49)
	}
	if stats.Mean != stats.Sum/float64(stats.Count) {
		t.Fatalf("mean = %v, want %v", stats.Mean, stats.Sum/float64(stats.Count))
	}
}
