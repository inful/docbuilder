package daemon

import (
	"strconv"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/server/responses"
	"git.home.luguber.info/inful/docbuilder/internal/state"
)

// RepositoryStatuses returns the repositories the daemon knows about and
// their last known state.
//
// The list comes from the configured repositories plus the ones the last
// discovery run found; the state comes from the state store, so a
// repository that was never built shows up with an empty status instead
// of disappearing.
func (d *Daemon) RepositoryStatuses() []responses.RepositoryInfo {
	if d == nil {
		return nil
	}

	urls := make([]string, 0, len(d.config.Repositories))
	seen := make(map[string]struct{}, len(d.config.Repositories))
	add := func(url string) {
		if url == "" {
			return
		}
		if _, ok := seen[url]; ok {
			return
		}
		seen[url] = struct{}{}
		urls = append(urls, url)
	}

	for i := range d.config.Repositories {
		add(d.config.Repositories[i].URL)
	}
	if d.discover.discoveryCache != nil {
		if result, err := d.discover.discoveryCache.Get(); err == nil && result != nil {
			for _, repo := range result.Repositories {
				if repo == nil {
					continue
				}
				add(repo.CloneURL)
			}
		}
	}

	infos := make([]responses.RepositoryInfo, 0, len(urls))
	for _, url := range urls {
		info := responses.RepositoryInfo{URL: url}
		// The preview has no state: there is nothing to read.
		var repoState *state.RepositoryState
		if d.state.stateManager != nil {
			repoState = d.state.stateManager.GetRepository(url)
		}
		if repoState != nil {
			info.Name = repoState.Name
			info.Branch = repoState.Branch
			info.LastSync = repoState.LastBuild
			info.LastCommit = repoState.LastCommit
			info.DocCount = repoState.DocumentCount
			info.Status = repositoryStatus(repoState.BuildCount, repoState.ErrorCount, repoState.LastError)
		} else {
			info.Status = "unknown"
		}
		infos = append(infos, info)
	}
	return infos
}

// repositoryStatus turns the build counters of a repository into a short
// status word.
func repositoryStatus(buildCount, errorCount int64, lastError string) string {
	switch {
	case lastError != "":
		return "error"
	case errorCount > 0 && buildCount == errorCount:
		return "failed"
	case buildCount > 0:
		return "ok"
	default:
		return "pending"
	}
}

// BuildStats returns the build statistics of the daemon and the last
// build it ran.
func (d *Daemon) BuildStats() (responses.BuildStatistics, *responses.BuildInfo) {
	if d == nil || d.state.buildProjection == nil {
		return responses.BuildStatistics{}, nil
	}

	history := d.state.buildProjection.GetHistory()
	stats := responses.BuildStatistics{}
	for _, build := range history {
		stats.TotalBuilds++
		switch build.Status {
		case "completed":
			stats.SuccessfulBuilds++
		case "failed":
			stats.FailedBuilds++
		}
	}

	last := d.state.buildProjection.GetLastCompletedBuild()
	if last == nil {
		return stats, nil
	}

	info := &responses.BuildInfo{
		ID:        last.BuildID,
		Status:    last.Status,
		StartTime: &last.StartedAt,
	}
	if last.CompletedAt != nil {
		end := *last.CompletedAt
		info.EndTime = &end
	}
	if last.Duration > 0 {
		duration := last.Duration.String()
		info.Duration = &duration
	}
	if last.ReportData != nil {
		info.Stages = make(map[string]string, len(last.ReportData.StageDurations))
		for stage, ms := range last.ReportData.StageDurations {
			info.Stages[stage] = strconv.FormatInt(ms, 10) + "ms"
		}
	}

	return stats, info
}

// LastBuildInfo returns the last build the daemon ran, if any.
func (d *Daemon) LastBuildInfo() *responses.BuildInfo {
	_, info := d.BuildStats()
	return info
}

// LastBuildDuration returns how long the last completed build took.
func (d *Daemon) LastBuildDuration() time.Duration {
	if d == nil || d.state.buildProjection == nil {
		return 0
	}
	last := d.state.buildProjection.GetLastCompletedBuild()
	if last == nil {
		return 0
	}
	return last.Duration
}
