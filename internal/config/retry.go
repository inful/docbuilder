package config

import (
	"git.home.luguber.info/inful/docbuilder/internal/foundation/normalization"
)

// RetryBackoffMode enumerates supported backoff strategies for retries.
type RetryBackoffMode string

const (
	RetryBackoffFixed       RetryBackoffMode = "fixed"
	RetryBackoffLinear      RetryBackoffMode = "linear"
	RetryBackoffExponential RetryBackoffMode = "exponential"
)

// NormalizeRetryBackoff converts arbitrary user input (case-insensitive) into a typed mode, returning empty string for unknown.
var retryBackoffNormalizer = normalization.NewNormalizer(map[string]RetryBackoffMode{
	"fixed":       RetryBackoffFixed,
	"linear":      RetryBackoffLinear,
	"exponential": RetryBackoffExponential,
}, RetryBackoffFixed)

func NormalizeRetryBackoff(raw string) RetryBackoffMode {
	return retryBackoffNormalizer.Normalize(raw)
}
