package config

import "strings"

// RetryBackoffMode enumerates supported backoff strategies for retries.
type RetryBackoffMode string

const (
    RetryBackoffFixed       RetryBackoffMode = "fixed"
    RetryBackoffLinear      RetryBackoffMode = "linear"
    RetryBackoffExponential RetryBackoffMode = "exponential"
)

// NormalizeRetryBackoff converts arbitrary user input (case-insensitive) into a typed mode, returning empty string for unknown.
func NormalizeRetryBackoff(raw string) RetryBackoffMode {
    switch strings.ToLower(strings.TrimSpace(raw)) {
    case string(RetryBackoffFixed):
        return RetryBackoffFixed
    case string(RetryBackoffLinear):
        return RetryBackoffLinear
    case string(RetryBackoffExponential):
        return RetryBackoffExponential
    default:
        return ""
    }
}
