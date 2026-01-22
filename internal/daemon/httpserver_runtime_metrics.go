package daemon

import "time"

// HTTPRequestsTotal is a metrics bridge for monitoring endpoints.
//
// The daemon currently doesn't track per-request totals in a dedicated counter.
// Return 0 as an explicit "unavailable" value.
func (d *Daemon) HTTPRequestsTotal() int {
	return 0
}

// RepositoriesTotal returns the number of discovered repositories from the last cached discovery result.
func (d *Daemon) RepositoriesTotal() int {
	if d == nil || d.discoveryCache == nil {
		return 0
	}
	result, err := d.discoveryCache.Get()
	if err != nil || result == nil {
		return 0
	}
	return len(result.Repositories)
}

// LastDiscoveryDurationSec returns the duration (seconds) of the last discovery run.
func (d *Daemon) LastDiscoveryDurationSec() int {
	if d == nil || d.discoveryCache == nil {
		return 0
	}
	result, err := d.discoveryCache.Get()
	if err != nil || result == nil {
		return 0
	}
	return int(result.Duration.Seconds())
}

// LastBuildDurationSec returns the duration (seconds) of the last completed build.
//
// This is computed by summing stage duration samples when available.
func (d *Daemon) LastBuildDurationSec() int {
	if d == nil || d.buildProjection == nil {
		return 0
	}
	last := d.buildProjection.GetLastCompletedBuild()
	if last == nil || last.ReportData == nil {
		return 0
	}

	var total time.Duration
	for _, ms := range last.ReportData.StageDurations {
		total += time.Duration(ms) * time.Millisecond
	}
	return int(total.Seconds())
}
