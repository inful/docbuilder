package daemon

import "git.home.luguber.info/inful/docbuilder/internal/forge/discoveryrunner"

// DiscoveryCache is a type alias that preserves the daemon package API while the
// implementation lives in internal/forge/discoveryrunner.
type DiscoveryCache = discoveryrunner.Cache

func NewDiscoveryCache() *DiscoveryCache { return discoveryrunner.NewCache() }
