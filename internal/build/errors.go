package build

import (
	"git.home.luguber.info/inful/docbuilder/internal/hugo/models"
)

// ErrClone is returned when a repository clone operation fails.
//
// Always wrap this error with contextual information at the call site.
var (
	ErrClone     = models.ErrClone     // ErrClone indicates a repository clone failure.
	ErrDiscovery = models.ErrDiscovery // ErrDiscovery indicates a documentation discovery failure.
	ErrHugo      = models.ErrHugo      // ErrHugo indicates a Hugo site generation failure.
)
