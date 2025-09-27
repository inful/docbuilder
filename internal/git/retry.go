package git

import (
    "errors"
    "fmt"
    "log/slog"
    "net"
    "strings"
    "time"

    "git.home.luguber.info/inful/docbuilder/internal/logfields"
)

// withRetry wraps an operation with retry logic based on build configuration.
func (c *Client) withRetry(op, repoName string, fn func() (string, error)) (string, error) {
    if c.buildCfg == nil || c.buildCfg.MaxRetries <= 0 { return fn() }
    initial, _ := time.ParseDuration(c.buildCfg.RetryInitialDelay)
    if initial <= 0 { initial = 500 * time.Millisecond }
    maxDelay, _ := time.ParseDuration(c.buildCfg.RetryMaxDelay)
    if maxDelay <= 0 { maxDelay = 10 * time.Second }
    var lastErr error
    for attempt := 0; attempt <= c.buildCfg.MaxRetries; attempt++ {
        if attempt > 0 {
            slog.Warn("retrying git operation", slog.String("operation", op), logfields.Name(repoName), slog.Int("attempt", attempt))
        }
        c.inRetry = true
        path, err := fn()
        c.inRetry = false
        if err == nil { return path, nil }
        lastErr = err
        if isPermanentGitError(err) {
            slog.Error("permanent git error", slog.String("operation", op), logfields.Name(repoName), slog.String("error", err.Error()))
            return "", err
        }
        if attempt == c.buildCfg.MaxRetries { break }
        delay := computeBackoffDelay(string(c.buildCfg.RetryBackoff), attempt, initial, maxDelay)
        time.Sleep(delay)
    }
    return "", fmt.Errorf("git %s failed after retries: %w", op, lastErr)
}

func computeBackoffDelay(strategy string, attempt int, initial, max time.Duration) time.Duration {
    if attempt <= 0 { return initial }
    switch strings.ToLower(strategy) {
    case "linear":
        d := time.Duration(attempt+1) * initial
        if d > max { return max }
        return d
    case "exponential":
        d := initial * (1 << attempt)
        if d > max { return max }
        return d
    case "fixed", "":
        fallthrough
    default:
        if initial > max { return max }
        return initial
    }
}

func isPermanentGitError(err error) bool {
    if err == nil { return false }
    msg := strings.ToLower(err.Error())
    if strings.Contains(msg, "auth") || strings.Contains(msg, "permission") || strings.Contains(msg, "denied") { return true }
    if strings.Contains(msg, "not found") || strings.Contains(msg, "no such remote") || strings.Contains(msg, "invalid reference") { return true }
    if strings.Contains(msg, "unsupported protocol") { return true }
    var nerr net.Error
    if errors.As(err, &nerr) { return !nerr.Timeout() }
    return false
}

// expose for tests within package (tests already import these names)
var (
    ComputeBackoffDelay = computeBackoffDelay
    IsPermanentGitError = isPermanentGitError
)