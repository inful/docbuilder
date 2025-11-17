package retry

import (
	"fmt"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// Policy encapsulates retry/backoff settings for transient failures.
// It is immutable after construction.
type Policy struct {
	Mode       config.RetryBackoffMode // fixed|linear|exponential
	Initial    time.Duration           // base delay
	Max        time.Duration           // cap for growth
	MaxRetries int                     // maximum retry attempts after the first failure
}

// DefaultPolicy returns a sensible default policy (linear, 1s initial, 30s cap, 2 retries).
func DefaultPolicy() Policy {
	return Policy{Mode: config.RetryBackoffLinear, Initial: time.Second, Max: 30 * time.Second, MaxRetries: 2}
}

// NewPolicy builds a policy from raw config fields; zero/invalid values fall back to defaults.
func NewPolicy(mode config.RetryBackoffMode, initial, maxDuration time.Duration, maxRetries int) Policy {
	p := DefaultPolicy()
	if maxRetries >= 0 {
		p.MaxRetries = maxRetries
	}
	if initial > 0 {
		p.Initial = initial
	}
	if maxDuration > 0 {
		p.Max = maxDuration
	}
	if mode != "" {
		switch mode {
		case config.RetryBackoffFixed, config.RetryBackoffLinear, config.RetryBackoffExponential:
			p.Mode = mode
		default:
			// unknown -> keep default
		}
	}
	if p.Initial > p.Max {
		p.Initial = p.Max
	}
	return p
}

// Delay returns the backoff delay for the given retry attempt number (1-based: first retry => 1).
func (p Policy) Delay(retryCount int) time.Duration {
	if retryCount <= 0 {
		return 0
	}
	switch p.Mode {
	case config.RetryBackoffFixed:
		return p.Initial
	case config.RetryBackoffExponential:
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
func (p Policy) Validate() error {
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
