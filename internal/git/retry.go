package git

import (
	"context"
	stdErrors "errors"
	"log/slog"
	"net"
	"strings"
	"time"

	appcfg "git.home.luguber.info/inful/docbuilder/internal/config"
	derrors "git.home.luguber.info/inful/docbuilder/internal/foundation/errors"
	"git.home.luguber.info/inful/docbuilder/internal/logfields"
	"git.home.luguber.info/inful/docbuilder/internal/retry"
)

// withRetry runs fn with the client's configured retry policy, then wraps
// any per-failure error returned after retries in a classified GitError.
//
// It is generic over the success type so the string- and CloneResult-shaped
// callers share a single loop. c.inRetry is set for the duration of each
// fn call so callers like UpdateRepo/CloneRepoWithMetadata skip wrapping
// nested retry invocations.
//
// Free-function (rather than a method on *Client) because Go 1.25 does not
// allow type parameters on methods.
func withRetry[T any](ctx context.Context, c *Client, op, repoName string, fn func() (T, error)) (T, error) {
	var zero T
	if c.buildCfg == nil || c.buildCfg.MaxRetries <= 0 {
		return fn()
	}
	initial, _ := time.ParseDuration(c.buildCfg.RetryInitialDelay)
	if initial <= 0 {
		initial = 500 * time.Millisecond
	}
	maxDelay, _ := time.ParseDuration(c.buildCfg.RetryMaxDelay)
	if maxDelay <= 0 {
		maxDelay = 10 * time.Second
	}
	policy := retry.NewPolicy(appcfg.RetryBackoffMode(strings.ToLower(string(c.buildCfg.RetryBackoff))), initial, maxDelay, c.buildCfg.MaxRetries)

	const multRateLimit = 3.0
	var held T
	runErr := policy.Do(
		ctx,
		func(_ context.Context) error {
			c.inRetry = true
			defer func() { c.inRetry = false }()
			v, e := fn()
			if e == nil {
				held = v
			}
			return e
		},
		retry.RetryHooks{
			IsRetryable: func(err error) bool { return !isPermanentGitError(err) },
			AdjustDelay: func(err error, base time.Duration) time.Duration {
				ce, ok := derrors.AsClassified(err)
				if !ok {
					return base
				}
				if ce.RetryStrategy() == derrors.RetryRateLimit {
					return time.Duration(float64(base) * multRateLimit)
				}
				return base
			},
			OnRetry: func(attempt int, _ error) {
				slog.Warn("retrying git operation", slog.String("operation", op), logfields.Name(repoName), slog.Int("attempt", attempt))
			},
		},
	)
	if runErr != nil {
		// Permanent errors short-circuit through IsRetryable=false; the
		// classifier has already added context (category, retry strategy).
		// Wrap the *transient-exhaustion* case so callers see a single
		// classified error rather than the raw network/timeout cause.
		if isPermanentGitError(runErr) {
			return zero, runErr
		}
		return zero, GitError("git operation failed after retries").
			WithCause(runErr).
			WithContext("op", op).
			WithContext("repo", repoName).
			Build()
	}
	return held, nil
}

func isPermanentGitError(err error) bool {
	if err == nil {
		return false
	}
	// Prefer structured strategy if available
	if ce, ok := derrors.AsClassified(err); ok {
		return ce.RetryStrategy() == derrors.RetryNever
	}

	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "auth") || strings.Contains(msg, "permission") || strings.Contains(msg, "denied") {
		return true
	}
	if strings.Contains(msg, "not found") || strings.Contains(msg, "no such remote") || strings.Contains(msg, "invalid reference") {
		return true
	}
	if strings.Contains(msg, "unsupported protocol") {
		return true
	}
	var nerr net.Error
	if stdErrors.As(err, &nerr) {
		return !nerr.Timeout()
	}
	return false
}

// IsPermanentGitError exposes isPermanentGitError for tests within package.
var IsPermanentGitError = isPermanentGitError
