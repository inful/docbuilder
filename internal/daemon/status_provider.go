package daemon

import (
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/forge"
)

// GetConfigFilePath returns the daemon config file path.
func (d *Daemon) GetConfigFilePath() string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.configFilePath
}

// GetLastBuildTime returns the last successful build time (if any).
func (d *Daemon) GetLastBuildTime() *time.Time {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.lastBuild
}

// GetLastDiscovery returns the last successful discovery time (if any).
func (d *Daemon) GetLastDiscovery() *time.Time {
	if d.discoveryRunner == nil {
		return nil
	}
	return d.discoveryRunner.GetLastDiscovery()
}

// GetDiscoveryResult returns the cached discovery result and error.
func (d *Daemon) GetDiscoveryResult() (*forge.DiscoveryResult, error) {
	if d.discoveryCache == nil {
		//nolint:nilnil // nil result + nil error means no discovery has run yet.
		return nil, nil
	}
	return d.discoveryCache.Get()
}
