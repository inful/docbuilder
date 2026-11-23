package retry

import (
	"testing"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// The backoff of a large number of retries must not overflow: the delay
// became negative and the retry loop spun without a backoff.
func TestDelayDoesNotOverflow(t *testing.T) {
	policy := Policy{
		Mode:    config.RetryBackoffExponential,
		Initial: time.Nanosecond,
		Max:     time.Hour,
	}

	delay := policy.Delay(64)
	if delay < 0 {
		t.Fatalf("the delay of the retry 64 is %v", delay)
	}
	if delay > policy.Max {
		t.Fatalf("the delay of the retry 64 is %v, want at most %v", delay, policy.Max)
	}
}
