package build

import "errors"

// Sentinel domain errors used to classify high-level pipeline failures for retry semantics.
// They should always be wrapped with contextual information at the call site.
var (
	ErrClone     = errors.New("docbuilder: clone error")
	ErrDiscovery = errors.New("docbuilder: discovery error")
	ErrHugo      = errors.New("docbuilder: hugo error")
)
