package daemon

import (
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/forge"
)

// GetConfigFilePath returns the daemon config file path.
func (d *Daemon) GetConfigFilePath() string {
	d.lifecycle.mu.RLock()
	defer d.lifecycle.mu.RUnlock()
	return d.configFilePath
}

// GetLastBuildTime returns the last successful build time (if any).
func (d *Daemon) GetLastBuildTime() *time.Time {
	d.lifecycle.mu.RLock()
	defer d.lifecycle.mu.RUnlock()
	return d.lifecycle.lastBuild
}

// GetLastDiscovery returns the last successful discovery time (if any).
func (d *Daemon) GetLastDiscovery() *time.Time {
	if d.discover.discoveryRunner == nil {
		return nil
	}
	return d.discover.discoveryRunner.GetLastDiscovery()
}

// GetDiscoveryResult returns the cached discovery result and error.
func (d *Daemon) GetDiscoveryResult() (*forge.DiscoveryResult, error) {
	if d.discover.discoveryCache == nil {
		//nolint:nilnil // nil result + nil error means no discovery has run yet.
		return nil, nil
	}
	return d.discover.discoveryCache.Get()
}
