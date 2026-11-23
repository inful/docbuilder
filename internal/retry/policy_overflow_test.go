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

// The same overflow with a large initial delay: the delay of the retry
// was negative, and the retry loop hammered the failing service.
func TestDelayDoesNotOverflowWithALargeInitialDelay(t *testing.T) {
	policy := Policy{
		Mode:    config.RetryBackoffExponential,
		Initial: 10 * time.Second,
		Max:     time.Hour,
	}

	delay := policy.Delay(31)
	if delay < 0 {
		t.Fatalf("the delay of the retry 31 is %v", delay)
	}
	if delay > policy.Max {
		t.Fatalf("the delay of the retry 31 is %v, want at most %v", delay, policy.Max)
	}
}

// The linear backoff overflows the same way.
func TestTheLinearDelayDoesNotOverflow(t *testing.T) {
	policy := Policy{
		Mode:    config.RetryBackoffLinear,
		Initial: time.Hour,
		Max:     24 * time.Hour,
	}

	delay := policy.Delay(1 << 40)
	if delay < 0 {
		t.Fatalf("the delay of the retry %d is %v", 1<<40, delay)
	}
}
