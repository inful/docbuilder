package daemon

import "git.home.luguber.info/inful/docbuilder/internal/forge/discoveryrunner"

// DiscoveryRunner is a type alias that preserves the daemon package API while
// the implementation lives in internal/forge/discoveryrunner.
type DiscoveryRunner = discoveryrunner.Runner

type DiscoveryRunnerConfig = discoveryrunner.Config

func NewDiscoveryRunner(cfg DiscoveryRunnerConfig) *DiscoveryRunner {
	return discoveryrunner.New(cfg)
}
