package hugo

import "time"

// BuildReport captures high-level metrics about a site generation run.
type BuildReport struct {
    Repositories int
    Files        int
    Start        time.Time
    End          time.Time
    Errors       []error
}

func newBuildReport(repos, files int) *BuildReport {
    return &BuildReport{Repositories: repos, Files: files, Start: time.Now()}
}

func (r *BuildReport) finish() { r.End = time.Now() }
