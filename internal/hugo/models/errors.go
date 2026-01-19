package models

import "errors"

// Standard sentinels for documentation build stages.
var (
	ErrClone     = errors.New("docbuilder: clone error")     // ErrClone indicates a repository clone failure.
	ErrDiscovery = errors.New("docbuilder: discovery error") // ErrDiscovery indicates a documentation discovery failure.
	ErrHugo      = errors.New("docbuilder: hugo error")      // ErrHugo indicates a Hugo site generation failure.
)
