package linkverify

import "testing"

// A configuration that asks for a negative number of workers would stop
// the daemon: a channel of a negative size is a panic.
func TestMaxConcurrentNeverGoesBelowOne(t *testing.T) {
	for _, configured := range []int{-5, 0, 1, 10} {
		got := maxConcurrent(configured)
		if got < 1 {
			t.Fatalf("maxConcurrent(%d) = %d", configured, got)
		}
		if configured >= 1 && got != configured {
			t.Fatalf("maxConcurrent(%d) = %d", configured, got)
		}
	}
}
