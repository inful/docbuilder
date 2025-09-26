package hugo

import (
	"fmt"
	"time"
)

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

// Summary returns a human-readable single-line summary.
func (r *BuildReport) Summary() string {
	dur := r.End.Sub(r.Start)
	return fmt.Sprintf("repos=%d files=%d duration=%s errors=%d", r.Repositories, r.Files, dur.Truncate(time.Millisecond), len(r.Errors))
}

