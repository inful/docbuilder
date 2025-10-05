// Package build provides sentinel errors for classifying high-level pipeline failures in DocBuilder.
// These errors are used for retry semantics and should be wrapped with context at the call site.
package build

import "errors"

// ErrClone is returned when a repository clone operation fails.
//
// Always wrap this error with contextual information at the call site.
var (
	ErrClone     = errors.New("docbuilder: clone error")     // ErrClone indicates a repository clone failure.
	ErrDiscovery = errors.New("docbuilder: discovery error") // ErrDiscovery indicates a documentation discovery failure.
	ErrHugo      = errors.New("docbuilder: hugo error")      // ErrHugo indicates a Hugo site generation failure.
)
