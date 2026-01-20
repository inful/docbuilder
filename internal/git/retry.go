package git

import (
	stdErrors "errors"
	"log/slog"
	"net"
	"strings"
	"time"

	appcfg "git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/foundation/errors"
	"git.home.luguber.info/inful/docbuilder/internal/logfields"
	"git.home.luguber.info/inful/docbuilder/internal/retry"
)

// withRetry wraps an operation with retry logic based on build configuration.
func (c *Client) withRetry(op, repoName string, fn func() (string, error)) (string, error) {
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
	pol := retry.NewPolicy(appcfg.RetryBackoffMode(strings.ToLower(string(c.buildCfg.RetryBackoff))), initial, maxDelay, c.buildCfg.MaxRetries)

	// Adaptive delay multipliers keyed by retry strategy
	const (
		multRateLimit = 3.0
		multBackoff   = 1.0
	)
	var lastErr error
	for attempt := 0; attempt <= c.buildCfg.MaxRetries; attempt++ {
		if attempt > 0 {
			slog.Warn("retrying git operation", slog.String("operation", op), logfields.Name(repoName), slog.Int("attempt", attempt))
		}
		c.inRetry = true
		path, err := fn()
		c.inRetry = false
		if err == nil {
			return path, nil
		}
		lastErr = err
		if isPermanentGitError(err) {
			slog.Error("permanent git error", slog.String("operation", op), logfields.Name(repoName), slog.String("error", err.Error()))
			return "", err
		}
		if attempt == c.buildCfg.MaxRetries {
			break
		}
		delay := pol.Delay(attempt + 1) // base delay
		// Adjust delay based on retry strategy
		if ce, ok := errors.AsClassified(err); ok {
			switch ce.RetryStrategy() {
			case errors.RetryRateLimit:
				delay = time.Duration(float64(delay) * multRateLimit)
			case errors.RetryBackoff:
				delay = time.Duration(float64(delay) * multBackoff)
			case errors.RetryNever, errors.RetryImmediate, errors.RetryUserAction:
				// Other strategies use base delay
			}
		}

		time.Sleep(delay)
	}
	return "", GitError("git operation failed after retries").
		WithCause(lastErr).
		WithContext("op", op).
		WithContext("repo", repoName).
		Build()
}

// withRetryMetadata wraps an operation returning CloneResult with retry logic.
func (c *Client) withRetryMetadata(op, repoName string, fn func() (CloneResult, error)) (CloneResult, error) {
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
	pol := retry.NewPolicy(appcfg.RetryBackoffMode(strings.ToLower(string(c.buildCfg.RetryBackoff))), initial, maxDelay, c.buildCfg.MaxRetries)

	const (
		multRateLimit = 3.0
		multBackoff   = 1.0
	)
	var lastErr error
	for attempt := 0; attempt <= c.buildCfg.MaxRetries; attempt++ {
		if attempt > 0 {
			slog.Warn("retrying git operation", slog.String("operation", op), logfields.Name(repoName), slog.Int("attempt", attempt))
		}
		c.inRetry = true
		result, err := fn()
		c.inRetry = false
		if err == nil {
			return result, nil
		}
		lastErr = err
		if isPermanentGitError(err) {
			slog.Error("permanent git error", slog.String("operation", op), logfields.Name(repoName), slog.String("error", err.Error()))
			return CloneResult{}, err
		}
		if attempt == c.buildCfg.MaxRetries {
			break
		}
		delay := pol.Delay(attempt + 1)
		// Adjust delay based on retry strategy
		if ce, ok := errors.AsClassified(err); ok {
			switch ce.RetryStrategy() {
			case errors.RetryRateLimit:
				delay = time.Duration(float64(delay) * multRateLimit)
			case errors.RetryBackoff:
				delay = time.Duration(float64(delay) * multBackoff)
			case errors.RetryNever, errors.RetryImmediate, errors.RetryUserAction:
				// Other strategies use base delay
			}
		}
		time.Sleep(delay)
	}
	return CloneResult{}, GitError("git operation failed after retries").
		WithCause(lastErr).
		WithContext("op", op).
		WithContext("repo", repoName).
		Build()
}

func isPermanentGitError(err error) bool {
	if err == nil {
		return false
	}
	// Prefer structured strategy if available
	if ce, ok := errors.AsClassified(err); ok {
		return ce.RetryStrategy() == errors.RetryNever
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

// IsPermanentGitError is exposed for tests within package.
var IsPermanentGitError = isPermanentGitError
