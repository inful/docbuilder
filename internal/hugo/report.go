package hugo

import (
	"fmt"
	"time"
)

// BuildReport captures high-level metrics about a site generation run.
type BuildReport struct {
	Repositories    int
	Files           int
	Start           time.Time
	End             time.Time
	Errors          []error // fatal errors causing build abortion (at most one today)
	Warnings        []error // non-fatal issues (e.g., hugo binary missing, partial failures)
	StageDurations  map[string]time.Duration
	StageErrorKinds map[string]string // stage -> error kind (fatal|warning|canceled)
	// Enrichment fields (incremental observability additions)
	ClonedRepositories int               // repositories successfully cloned or validated
	FailedRepositories int               // repositories that failed to clone/auth
	RenderedPages      int               // markdown pages successfully processed & written
	StageCounts        map[string]StageCount // per-stage classification counts
	Outcome            string            // derived overall outcome: success|warning|failed|canceled
	StaticRendered     bool              // true if Hugo static site render executed successfully
}

// StageCount aggregates counts of outcomes for a stage (future proofing if we repeat stages or add sub-steps)
type StageCount struct {
	Success int
	Warning int
	Fatal   int
	Canceled int
}

func newBuildReport(repos, files int) *BuildReport {
	return &BuildReport{
		Repositories:    repos,
		Files:           files,
		Start:           time.Now(),
		StageDurations:  make(map[string]time.Duration),
		StageErrorKinds: make(map[string]string),
		StageCounts:     make(map[string]StageCount),
		// ClonedRepositories defaults to repos (best-effort until clone metrics added)
		ClonedRepositories: repos,
	}
}

func (r *BuildReport) finish() { r.End = time.Now() }

// Summary returns a human-readable single-line summary.
func (r *BuildReport) Summary() string {
	dur := r.End.Sub(r.Start)
	return fmt.Sprintf("repos=%d files=%d duration=%s errors=%d warnings=%d stages=%d rendered=%d outcome=%s", r.Repositories, r.Files, dur.Truncate(time.Millisecond), len(r.Errors), len(r.Warnings), len(r.StageDurations), r.RenderedPages, r.Outcome)
}

// deriveOutcome sets the Outcome field based on recorded errors/warnings
func (r *BuildReport) deriveOutcome() {
	if len(r.Errors) > 0 {
		// Distinguish canceled vs failed
		for _, e := range r.Errors {
			if se, ok := e.(*StageError); ok && se.Kind == StageErrorCanceled {
				r.Outcome = "canceled"
				return
			}
		}
		r.Outcome = "failed"
		return
	}
	if len(r.Warnings) > 0 {
		r.Outcome = "warning"
		return
	}
	r.Outcome = "success"
}
