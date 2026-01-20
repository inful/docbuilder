package git

import (
	"testing"
	"time"

	appcfg "git.home.luguber.info/inful/docbuilder/internal/config"
)

// fakeOp simulates an operation returning provided errors sequentially.
func TestAdaptiveRetryRateLimit(t *testing.T) {
	c := &Client{workspaceDir: t.TempDir(), buildCfg: &appcfg.BuildConfig{MaxRetries: 2, RetryBackoff: appcfg.RetryBackoffFixed, RetryInitialDelay: "10ms", RetryMaxDelay: "50ms"}}
	calls := 0
	start := time.Now()
	_, err := c.withRetry("clone", "repo", func() (string, error) {
		calls++
		if calls < 3 { // fail first two attempts
			return "", GitError("rate limit exceeded").RateLimit().Build()
		}
		return "path", nil
	})
	if err != nil {
		t.Fatalf("expected success after retries: %v", err)
	}
	if calls != 3 {
		t.Fatalf("expected 3 attempts, got %d", calls)
	}
	dur := time.Since(start)
	if dur < 20*time.Millisecond { // two waits scaled by multiplier (>= base * 2)
		t.Fatalf("expected cumulative delay >=20ms, got %s", dur)
	}
}
