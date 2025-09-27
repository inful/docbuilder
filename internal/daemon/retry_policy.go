package daemon

import (
	"fmt"
	"time"
)

// RetryPolicy encapsulates retry/backoff settings for transient build stage failures.
// It is immutable after construction.
type RetryPolicy struct {
	Mode       string        // fixed|linear|exponential
	Initial    time.Duration // base delay
	Max        time.Duration // cap for growth
	MaxRetries int           // maximum retry attempts after the first failure
}

// DefaultRetryPolicy returns a sensible default policy (linear, 1s initial, 30s cap, 2 retries).
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{Mode: "linear", Initial: time.Second, Max: 30 * time.Second, MaxRetries: 2}
}

// NewRetryPolicy builds a policy from raw config fields; zero/invalid values fall back to defaults.
func NewRetryPolicy(mode string, initial, max time.Duration, maxRetries int) RetryPolicy {
	p := DefaultRetryPolicy()
	if maxRetries >= 0 {
		p.MaxRetries = maxRetries
	}
	if initial > 0 {
		p.Initial = initial
	}
	if max > 0 {
		p.Max = max
	}
	switch mode {
	case "fixed", "linear", "exponential":
		p.Mode = mode
	case "":
		// keep default
	default:
		// leave default mode, but we could log later when logger accessible.
	}
	if p.Initial > p.Max {
		p.Initial = p.Max
	}
	return p
}

// Delay returns the backoff delay for the given retry attempt number (1-based: first retry => 1).
func (p RetryPolicy) Delay(retryCount int) time.Duration {
	if retryCount <= 0 {
		return 0
	}
	switch p.Mode {
	case "fixed":
		return p.Initial
	case "exponential":
		d := p.Initial * (1 << (retryCount - 1))
		if d > p.Max {
			return p.Max
		}
		return d
	default: // linear
		d := time.Duration(retryCount) * p.Initial
		if d > p.Max {
			return p.Max
		}
		return d
	}
}

// Validate ensures invariants; returns error if policy impossible to apply.
func (p RetryPolicy) Validate() error {
	if p.Initial <= 0 {
		return fmt.Errorf("initial must be >0")
	}
	if p.Max <= 0 {
		return fmt.Errorf("max must be >0")
	}
	if p.MaxRetries < 0 {
		return fmt.Errorf("max retries cannot be negative")
	}
	return nil
}
