package git

import (
    "fmt"
    "strings"
)

// Base typed git errors enabling structured classification without string parsing upstream.
type AuthError struct { Op, URL string; Err error }
func (e *AuthError) Error() string { return fmt.Sprintf("%s auth error for %s: %v", e.Op, e.URL, e.Err) }
func (e *AuthError) Unwrap() error { return e.Err }

type NotFoundError struct { Op, URL string; Err error }
func (e *NotFoundError) Error() string { return fmt.Sprintf("%s not found %s: %v", e.Op, e.URL, e.Err) }
func (e *NotFoundError) Unwrap() error { return e.Err }

type UnsupportedProtocolError struct { Op, URL string; Err error }
func (e *UnsupportedProtocolError) Error() string { return fmt.Sprintf("%s unsupported protocol %s: %v", e.Op, e.URL, e.Err) }
func (e *UnsupportedProtocolError) Unwrap() error { return e.Err }

type RemoteDivergedError struct { Op, URL, Branch string; Err error }
func (e *RemoteDivergedError) Error() string { return fmt.Sprintf("%s remote diverged %s@%s: %v", e.Op, e.URL, e.Branch, e.Err) }
func (e *RemoteDivergedError) Unwrap() error { return e.Err }

// Transient typed errors
type RateLimitError struct { Op, URL string; Err error }
func (e *RateLimitError) Error() string { return fmt.Sprintf("%s rate limit %s: %v", e.Op, e.URL, e.Err) }
func (e *RateLimitError) Unwrap() error { return e.Err }

type NetworkTimeoutError struct { Op, URL string; Err error }
func (e *NetworkTimeoutError) Error() string { return fmt.Sprintf("%s network timeout %s: %v", e.Op, e.URL, e.Err) }
func (e *NetworkTimeoutError) Unwrap() error { return e.Err }

// classifyFetchError wraps fetch-origin failures into typed variants when possible.
func classifyFetchError(url string, err error) error {
    if err == nil { return nil }
    l := strings.ToLower(err.Error())
    switch {
    case strings.Contains(l, "auth"):
        return &AuthError{Op: "fetch", URL: url, Err: err}
    case strings.Contains(l, "not found") || strings.Contains(l, "repository does not exist"):
        return &NotFoundError{Op: "fetch", URL: url, Err: err}
    case strings.Contains(l, "unsupported protocol"):
        return &UnsupportedProtocolError{Op: "fetch", URL: url, Err: err}
    case strings.Contains(l, "rate limit") || strings.Contains(l, "too many requests"):
        return &RateLimitError{Op: "fetch", URL: url, Err: err}
    case strings.Contains(l, "timeout") || strings.Contains(l, "i/o timeout"):
        return &NetworkTimeoutError{Op: "fetch", URL: url, Err: err}
    default:
        return err
    }
}