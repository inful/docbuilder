package models

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/metrics"
	"git.home.luguber.info/inful/docbuilder/internal/version"
)

// DetectHugoVersion attempts to detect the version of the hugo binary on PATH.
func DetectHugoVersion(ctx context.Context) string {
	hugoPath, err := exec.LookPath("hugo")
	if err != nil {
		return ""
	}
	// #nosec G204 - hugoPath is derived from config/discovery
	cmd := exec.CommandContext(ctx, hugoPath, "version")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return ParseHugoVersion(string(output))
}

// ParseHugoVersion extracts the semantic version from hugo version output.
func ParseHugoVersion(output string) string {
	versionRegex := regexp.MustCompile(`v?(\d+\.\d+\.\d+)`)
	matches := versionRegex.FindStringSubmatch(output)
	if len(matches) >= 2 {
		return matches[1]
	}
	simpleRegex := regexp.MustCompile(`(\d+\.\d+\.\d+)`)
	matches = simpleRegex.FindStringSubmatch(output)
	if len(matches) >= 2 {
		return matches[1]
	}
	return ""
}

// NewBuildReport constructs a new BuildReport.
func NewBuildReport(ctx context.Context, repos, files int) *BuildReport {
	return &BuildReport{
		SchemaVersion:     1,
		Repositories:      repos,
		Files:             files,
		Start:             time.Now(),
		StageDurations:    make(map[string]time.Duration),
		StageErrorKinds:   make(map[StageName]StageErrorKind),
		StageCounts:       make(map[StageName]StageCount),
		IndexTemplates:    make(map[string]IndexTemplateInfo),
		DocBuilderVersion: version.Version,
		HugoVersion:       DetectHugoVersion(ctx),
	}
}

// BuildOutcome is the typed enumeration of final build result states.
type BuildOutcome string

const (
	OutcomeSuccess  BuildOutcome = "success"
	OutcomeWarning  BuildOutcome = "warning"
	OutcomeFailed   BuildOutcome = "failed"
	OutcomeCanceled BuildOutcome = "canceled"
)

// BuildReport captures high-level metrics about a site generation run.
type BuildReport struct {
	SchemaVersion   int // schema version for external consumers
	Repositories    int
	Files           int
	Start           time.Time
	End             time.Time
	Errors          []error // fatal errors causing build abortion (at most one today)
	Warnings        []error // non-fatal issues (e.g., hugo binary missing, partial failures)
	StageDurations  map[string]time.Duration
	StageErrorKinds map[StageName]StageErrorKind // stage -> error kind (fatal|warning|canceled)
	// Enrichment fields (incremental observability additions)
	ClonedRepositories  int                      // repositories successfully cloned or validated
	FailedRepositories  int                      // repositories that failed to clone/auth
	SkippedRepositories int                      // repositories filtered out before cloning
	RenderedPages       int                      // markdown pages successfully processed & written
	StageCounts         map[StageName]StageCount // per-stage classification counts (typed keys; serialize as strings)
	StaticRendered      bool                     // true if Hugo static site render executed successfully
	Retries             int                      // total retry attempts (all stages combined)
	RetriesExhausted    bool                     // true if any stage exhausted retry budget
	Outcome             BuildOutcome             // single source of truth outcome (typed)
	// Issues captures structured machine-parsable issue taxonomy entries (warnings & errors) for future automation.
	Issues []ReportIssue // not yet populated widely; additive structure
	// SkipReason indicates why the pipeline was short-circuited (e.g. "no_changes"). Empty if full pipeline ran.
	SkipReason string
	// IndexTemplates records which source was used for each index template kind (main, repository, section)
	IndexTemplates map[string]IndexTemplateInfo
	// CloneStageSkipped is true when the pipeline did not include the clone_repos stage (direct generation path)
	// and false when the clone stage was part of the pipeline (even if it processed zero repositories).
	CloneStageSkipped bool
	// DocFilesHash is a stable hash (sha256 hex) of the discovered documentation file Hugo paths (sorted) for cache invalidation decisions.
	DocFilesHash string
	// DeltaDecision records analyzer decision: "full" | "partial" (empty if not evaluated)
	DeltaDecision string
	// DeltaChangedRepos lists repository URLs included in a partial rebuild decision.
	DeltaChangedRepos []string
	// DeltaRepoReasons maps repository URL -> reason string for inclusion/exclusion decisions (best effort; may be empty)
	DeltaRepoReasons map[string]string
	// ConfigHash stores the configuration snapshot hash used for this build (for skip/incremental decisions).
	ConfigHash string
	// PipelineVersion is an incrementing internal version for the orchestration/stage contract.
	PipelineVersion int
	// EffectiveRenderMode records the resolved render mode after considering config + legacy envs.
	EffectiveRenderMode string
	// DocBuilderVersion is the version of the docbuilder binary that created this build.
	DocBuilderVersion string
	// HugoVersion is the version of the hugo binary used to render this build (empty if not rendered).
	HugoVersion string
}

// AddIssue appends a structured issue and mirrors severity into Errors/Warnings slices.
func (r *BuildReport) AddIssue(code ReportIssueCode, stage StageName, severity IssueSeverity, msg string, transient bool, err error) {
	issue := ReportIssue{Code: code, Stage: stage, Severity: severity, Message: msg, Transient: transient}
	r.Issues = append(r.Issues, issue)
	if err != nil {
		switch severity {
		case SeverityError:
			r.Errors = append(r.Errors, err)
		case SeverityWarning:
			r.Warnings = append(r.Warnings, err)
		}
	}
}

// ReportIssueCode enumerates machine-parseable issue identifiers.
// These codes are stable contract and should only be appended (no reuse on removal).
type ReportIssueCode string

const (
	IssueCloneFailure      ReportIssueCode = "CLONE_FAILURE"
	IssuePartialClone      ReportIssueCode = "PARTIAL_CLONE"
	IssueDiscoveryFailure  ReportIssueCode = "DISCOVERY_FAILURE"
	IssueNoRepositories    ReportIssueCode = "NO_REPOSITORIES"
	IssueHugoExecution     ReportIssueCode = "HUGO_EXECUTION"
	IssueCanceled          ReportIssueCode = "BUILD_CANCELED"
	IssueAllClonesFailed   ReportIssueCode = "ALL_CLONES_FAILED"
	IssueGenericStageError ReportIssueCode = "GENERIC_STAGE_ERROR"
	IssueAuthFailure       ReportIssueCode = "AUTH_FAILURE"
	IssueRepoNotFound      ReportIssueCode = "REPO_NOT_FOUND"
	IssueUnsupportedProto  ReportIssueCode = "UNSUPPORTED_PROTOCOL"
	IssueRemoteDiverged    ReportIssueCode = "REMOTE_DIVERGED"
	IssueRateLimit         ReportIssueCode = "RATE_LIMIT"
	IssueNetworkTimeout    ReportIssueCode = "NETWORK_TIMEOUT"
)

// IssueSeverity represents normalized severity levels.
type IssueSeverity string

const (
	SeverityError   IssueSeverity = "error"
	SeverityWarning IssueSeverity = "warning"
)

// ReportIssue is a structured taxonomy entry describing a discrete problem encountered.
type ReportIssue struct {
	Code      ReportIssueCode `json:"code"`
	Stage     StageName       `json:"stage"`
	Severity  IssueSeverity   `json:"severity"`
	Message   string          `json:"message"`
	Transient bool            `json:"transient"`
}

// IndexTemplateInfo captures the resolution details for an index template kind.
type IndexTemplateInfo struct {
	Source string `json:"source"` // embedded | file
	Path   string `json:"path,omitempty"`
}

// StageCount aggregates counts of outcomes for a stage.
type StageCount struct {
	Success  int
	Warning  int
	Fatal    int
	Canceled int
}

// Finish sets the end time of the report.
func (r *BuildReport) Finish() { r.End = time.Now() }

// RecordStageResult updates BuildReport counters and emits metrics (if recorder non-nil).
func (r *BuildReport) RecordStageResult(stage StageName, res StageResult, recorder metrics.Recorder) {
	if r.StageCounts == nil {
		r.StageCounts = make(map[StageName]StageCount)
	}
	sc := r.StageCounts[stage]
	switch res {
	case StageResultSuccess:
		sc.Success++
		if recorder != nil {
			recorder.IncStageResult(string(stage), metrics.ResultSuccess)
		}
	case StageResultWarning:
		sc.Warning++
		if recorder != nil {
			recorder.IncStageResult(string(stage), metrics.ResultWarning)
		}
	case StageResultFatal:
		sc.Fatal++
		if recorder != nil {
			recorder.IncStageResult(string(stage), metrics.ResultFatal)
		}
	case StageResultCanceled:
		sc.Canceled++
		if recorder != nil {
			recorder.IncStageResult(string(stage), metrics.ResultCanceled)
		}
	case StageResultSkipped:
		// No counters for skipped yet
	}
	r.StageCounts[stage] = sc
}

// Summary returns a human-readable single-line summary.
func (r *BuildReport) Summary() string {
	dur := r.End.Sub(r.Start)
	return fmt.Sprintf("repos=%d files=%d duration=%s errors=%d warnings=%d stages=%d rendered=%d outcome=%s", r.Repositories, r.Files, dur.Truncate(time.Millisecond), len(r.Errors), len(r.Warnings), len(r.StageDurations), r.RenderedPages, string(r.Outcome))
}

// DeriveOutcome sets the Outcome field based on recorded errors/warnings.
func (r *BuildReport) DeriveOutcome() {
	if len(r.Errors) > 0 {
		for _, e := range r.Errors {
			var se *StageError
			if errors.As(e, &se) && se.Kind == StageErrorCanceled {
				r.Outcome = OutcomeCanceled
				return
			}
		}
		r.Outcome = OutcomeFailed
		return
	}
	if len(r.Warnings) > 0 {
		r.Outcome = OutcomeWarning
		return
	}
	r.Outcome = OutcomeSuccess
}

// Persist writes the report atomically into the provided root directory.
func (r *BuildReport) Persist(root string) error {
	if r.End.IsZero() {
		r.Finish()
		r.DeriveOutcome()
	}
	if err := os.MkdirAll(root, 0o750); err != nil {
		return fmt.Errorf("ensure root for report: %w", err)
	}
	// JSON
	jb, err := json.MarshalIndent(r.SanitizedCopy(), "", "  ")
	if err != nil {
		return fmt.Errorf("marshal report json: %w", err)
	}
	jsonPath := filepath.Join(root, "build-report.json")
	tmpJSON := jsonPath + ".tmp"
	if err := os.WriteFile(tmpJSON, jb, 0o600); err != nil {
		return fmt.Errorf("write temp report json: %w", err)
	}
	if err := os.Rename(tmpJSON, jsonPath); err != nil {
		return fmt.Errorf("atomic rename json: %w", err)
	}
	// Text summary
	summaryPath := filepath.Join(root, "build-report.txt")
	tmpTxt := summaryPath + ".tmp"
	if err := os.WriteFile(tmpTxt, []byte(r.Summary()+"\n"), 0o600); err != nil {
		return fmt.Errorf("write temp report summary: %w", err)
	}
	if err := os.Rename(tmpTxt, summaryPath); err != nil {
		return fmt.Errorf("atomic rename summary: %w", err)
	}
	return nil
}

// SanitizedCopy returns a shallow copy with error fields converted to strings for JSON friendliness.
func (r *BuildReport) SanitizedCopy() *BuildReportSerializable {
	stageCounts := make(map[string]StageCount, len(r.StageCounts))
	for k, v := range r.StageCounts {
		stageCounts[string(k)] = v
	}
	sek := make(map[string]string, len(r.StageErrorKinds))
	for k, v := range r.StageErrorKinds {
		sek[string(k)] = string(v)
	}

	if r.StageDurations == nil {
		r.StageDurations = map[string]time.Duration{}
	}
	if r.IndexTemplates == nil {
		r.IndexTemplates = map[string]IndexTemplateInfo{}
	}
	if r.Issues == nil {
		r.Issues = []ReportIssue{}
	}

	s := &BuildReportSerializable{
		SchemaVersion:       r.SchemaVersion,
		Repositories:        r.Repositories,
		Files:               r.Files,
		Start:               r.Start,
		End:                 r.End,
		Errors:              make([]string, len(r.Errors)),
		Warnings:            make([]string, len(r.Warnings)),
		StageDurations:      r.StageDurations,
		StageErrorKinds:     sek,
		ClonedRepositories:  r.ClonedRepositories,
		FailedRepositories:  r.FailedRepositories,
		SkippedRepositories: r.SkippedRepositories,
		RenderedPages:       r.RenderedPages,
		StageCounts:         stageCounts,
		Outcome:             string(r.Outcome),
		StaticRendered:      r.StaticRendered,
		Retries:             r.Retries,
		RetriesExhausted:    r.RetriesExhausted,
		Issues:              r.Issues,
		SkipReason:          r.SkipReason,
		IndexTemplates:      r.IndexTemplates,
		CloneStageSkipped:   r.CloneStageSkipped,
		DocFilesHash:        r.DocFilesHash,
		DeltaDecision:       r.DeltaDecision,
		DeltaChangedRepos:   r.DeltaChangedRepos,
		DeltaRepoReasons:    r.DeltaRepoReasons,
		ConfigHash:          r.ConfigHash,
		PipelineVersion:     r.PipelineVersion,
		EffectiveRenderMode: r.EffectiveRenderMode,
		DocBuilderVersion:   r.DocBuilderVersion,
		HugoVersion:         r.HugoVersion,
	}
	for i, e := range r.Errors {
		s.Errors[i] = e.Error()
	}
	for i, w := range r.Warnings {
		s.Warnings[i] = w.Error()
	}
	return s
}

// BuildReportSerializable mirrors BuildReport but with string errors for JSON output.
type BuildReportSerializable struct {
	SchemaVersion       int                          `json:"schema_version"`
	Repositories        int                          `json:"repositories"`
	Files               int                          `json:"files"`
	Start               time.Time                    `json:"start"`
	End                 time.Time                    `json:"end"`
	Errors              []string                     `json:"errors"`
	Warnings            []string                     `json:"warnings"`
	StageDurations      map[string]time.Duration     `json:"stage_durations"`
	StageErrorKinds     map[string]string            `json:"stage_error_kinds"`
	ClonedRepositories  int                          `json:"cloned_repositories"`
	FailedRepositories  int                          `json:"failed_repositories"`
	SkippedRepositories int                          `json:"skipped_repositories"`
	RenderedPages       int                          `json:"rendered_pages"`
	StageCounts         map[string]StageCount        `json:"stage_counts"`
	Outcome             string                       `json:"outcome"`
	StaticRendered      bool                         `json:"static_rendered"`
	Retries             int                          `json:"retries"`
	RetriesExhausted    bool                         `json:"retries_exhausted"`
	Issues              []ReportIssue                `json:"issues"`
	SkipReason          string                       `json:"skip_reason,omitempty"`
	IndexTemplates      map[string]IndexTemplateInfo `json:"index_templates,omitempty"`
	CloneStageSkipped   bool                         `json:"clone_stage_skipped,omitempty"`
	DocFilesHash        string                       `json:"doc_files_hash,omitempty"`
	DeltaDecision       string                       `json:"delta_decision,omitempty"`
	DeltaChangedRepos   []string                     `json:"delta_changed_repos,omitempty"`
	DeltaRepoReasons    map[string]string            `json:"delta_repo_reasons,omitempty"`
	ConfigHash          string                       `json:"config_hash,omitempty"`
	PipelineVersion     int                          `json:"pipeline_version,omitempty"`
	EffectiveRenderMode string                       `json:"effective_render_mode,omitempty"`
	DocBuilderVersion   string                       `json:"docbuilder_version,omitempty"`
	HugoVersion         string                       `json:"hugo_version,omitempty"`
}

func GetDocBuilderVersion() string {
	return version.Version
}
