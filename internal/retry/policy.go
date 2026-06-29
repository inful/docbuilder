package retry

import (
	"context"
	"errors"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	derrors "git.home.luguber.info/inful/docbuilder/internal/foundation/errors"
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
	case config.RetryBackoffLinear:
		d := time.Duration(retryCount) * p.Initial
		if d > p.Max {
			return p.Max
		}
		return d
	default:
		// Unknown mode - fallback to linear
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
		return errors.New("initial must be >0")
	}
	if p.Max <= 0 {
		return errors.New("max must be >0")
	}
	if p.MaxRetries < 0 {
		return errors.New("max retries cannot be negative")
	}
	return nil
}

// RetryHooks customizes per-error behavior of Policy.Do.
//
// All fields are optional. Nil callbacks fall back to conservative defaults:
//   - IsRetryable: retry unless the error classifies as RetryNever.
//   - AdjustDelay: use the policy's base backoff as-is.
//   - OnRetry: no observability.
//
// OnRetry fires *before* the per-retry delay (so the recorded attempt number
// is the attempt that just failed; the next attempt will be attempt+1).
// The attempt value is 1-based: it counts how many calls of fn have been made.
type RetryHooks struct {
	IsRetryable func(err error) bool
	AdjustDelay func(err error, base time.Duration) time.Duration
	OnRetry     func(attempt int, err error)
}

// Do executes fn with retry+backoff as configured by p.
//
// fn is invoked up to MaxRetries+1 times. The first failure short-circuits
// when IsRetryable (or the default classifier fallback) reports the error
// as non-retryable. Returns:
//
//   - nil if fn succeeds at any attempt;
//   - the last fn error if it is non-retryable or MaxRetries is exhausted;
//   - ctx.Err() if ctx is canceled mid-backoff.
//
// Do honors ctx during the backoff sleep; cancellation returned to the caller.
// The MaxRetries<=0 short-circuit skips the loop entirely (single fn call).
func (p Policy) Do(ctx context.Context, fn func(context.Context) error, hooks RetryHooks) error {
	if p.MaxRetries <= 0 {
		return fn(ctx)
	}
	var lastErr error
	for attempt := 0; attempt <= p.MaxRetries; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		lastErr = fn(ctx)
		if lastErr == nil {
			return nil
		}
		if !shouldRetry(lastErr, hooks) {
			return lastErr
		}
		if attempt == p.MaxRetries {
			break
		}
		delay := p.Delay(attempt + 1)
		if hooks.AdjustDelay != nil {
			delay = hooks.AdjustDelay(lastErr, delay)
		}
		if hooks.OnRetry != nil {
			hooks.OnRetry(attempt+1, lastErr)
		}
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return lastErr
}

// shouldRetry centralizes the default vs hook override for error retryability.
func shouldRetry(err error, hooks RetryHooks) bool {
	if hooks.IsRetryable != nil {
		return hooks.IsRetryable(err)
	}
	if ce, ok := derrors.AsClassified(err); ok {
		return ce.RetryStrategy() != derrors.RetryNever
	}
	// Unclassified errors are retried; matches the prior
	// git/build_queue behavior of "retry unless proven permanent".
	return true
}
