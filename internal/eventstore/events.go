package eventstore

import (
	"encoding/json"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/foundation/errors"
)

// BuildStartedMeta contains typed metadata for build start events.
// This replaces the untyped map[string]interface{} for compile-time safety.
type BuildStartedMeta struct {
	Type     string `json:"type"`                // Build type (e.g., "full", "incremental")
	Priority int    `json:"priority"`            // Job priority level
	WorkerID string `json:"worker_id"`           // Worker handling this build
	TenantID string `json:"tenant_id,omitempty"` // Optional tenant identifier
}

// BuildStarted is emitted when a build begins.
type BuildStarted struct {
	BaseEvent
	TenantID string           `json:"tenant_id"`
	Config   BuildStartedMeta `json:"config"`
}

// NewBuildStarted creates a BuildStarted event with typed metadata.
func NewBuildStarted(buildID string, meta BuildStartedMeta) (*BuildStarted, error) {
	payload, err := json.Marshal(map[string]any{
		"tenant_id": meta.TenantID,
		"config":    meta,
	})
	if err != nil {
		return nil, errors.EventStoreError("failed to marshal BuildStarted payload").
			WithCause(err).
			WithContext("build_id", buildID).
			Build()
	}

	return &BuildStarted{
		BaseEvent: BaseEvent{
			EventBuildID:   buildID,
			EventType:      "BuildStarted",
			EventTimestamp: time.Now(),
			EventPayload:   payload,
		},
		TenantID: meta.TenantID,
		Config:   meta,
	}, nil
}

// RepositoryCloned is emitted when a repository is successfully cloned.
type RepositoryCloned struct {
	BaseEvent
	RepoName string        `json:"repo_name"`
	Commit   string        `json:"commit"`
	Path     string        `json:"path"`
	Duration time.Duration `json:"duration_ms"`
}

// NewRepositoryCloned creates a RepositoryCloned event.
func NewRepositoryCloned(buildID, repoName, commit, path string, duration time.Duration) (*RepositoryCloned, error) {
	payload, err := json.Marshal(map[string]any{
		"repo_name":   repoName,
		"commit":      commit,
		"path":        path,
		"duration_ms": duration.Milliseconds(),
	})
	if err != nil {
		return nil, errors.EventStoreError("failed to marshal RepositoryCloned payload").
			WithCause(err).
			WithContext("build_id", buildID).
			WithContext("repo", repoName).
			Build()
	}

	return &RepositoryCloned{
		BaseEvent: BaseEvent{
			EventBuildID:   buildID,
			EventType:      "RepositoryCloned",
			EventTimestamp: time.Now(),
			EventPayload:   payload,
		},
		RepoName: repoName,
		Commit:   commit,
		Path:     path,
		Duration: duration,
	}, nil
}

// DocumentsDiscovered is emitted when documentation files are discovered.
type DocumentsDiscovered struct {
	BaseEvent
	RepoName  string   `json:"repo_name"`
	FileCount int      `json:"file_count"`
	Files     []string `json:"files"`
}

// NewDocumentsDiscovered creates a DocumentsDiscovered event.
func NewDocumentsDiscovered(buildID, repoName string, files []string) (*DocumentsDiscovered, error) {
	payload, err := json.Marshal(map[string]any{
		"repo_name":  repoName,
		"file_count": len(files),
		"files":      files,
	})
	if err != nil {
		return nil, errors.EventStoreError("failed to marshal DocumentsDiscovered payload").
			WithCause(err).
			WithContext("build_id", buildID).
			WithContext("repo", repoName).
			Build()
	}

	return &DocumentsDiscovered{
		BaseEvent: BaseEvent{
			EventBuildID:   buildID,
			EventType:      "DocumentsDiscovered",
			EventTimestamp: time.Now(),
			EventPayload:   payload,
		},
		RepoName:  repoName,
		FileCount: len(files),
		Files:     files,
	}, nil
}

// TransformApplied is emitted when a content transform is applied.
type TransformApplied struct {
	BaseEvent
	TransformName string        `json:"transform_name"`
	FileCount     int           `json:"file_count"`
	Duration      time.Duration `json:"duration_ms"`
}

// NewTransformApplied creates a TransformApplied event.
func NewTransformApplied(buildID, transformName string, fileCount int, duration time.Duration) (*TransformApplied, error) {
	payload, err := json.Marshal(map[string]any{
		"transform_name": transformName,
		"file_count":     fileCount,
		"duration_ms":    duration.Milliseconds(),
	})
	if err != nil {
		return nil, errors.EventStoreError("failed to marshal TransformApplied payload").
			WithCause(err).
			WithContext("build_id", buildID).
			WithContext("transform", transformName).
			Build()
	}

	return &TransformApplied{
		BaseEvent: BaseEvent{
			EventBuildID:   buildID,
			EventType:      "TransformApplied",
			EventTimestamp: time.Now(),
			EventPayload:   payload,
		},
		TransformName: transformName,
		FileCount:     fileCount,
		Duration:      duration,
	}, nil
}

// HugoConfigGenerated is emitted when Hugo configuration is generated.
type HugoConfigGenerated struct {
	BaseEvent
	ConfigHash    string         `json:"config_hash"`
	ThemeFeatures map[string]any `json:"theme_features"`
}

// NewHugoConfigGenerated creates a HugoConfigGenerated event.
func NewHugoConfigGenerated(buildID, configHash string, themeFeatures map[string]any) (*HugoConfigGenerated, error) {
	payload, err := json.Marshal(map[string]any{
		"config_hash":    configHash,
		"theme_features": themeFeatures,
	})
	if err != nil {
		return nil, errors.EventStoreError("failed to marshal HugoConfigGenerated payload").
			WithCause(err).
			WithContext("build_id", buildID).
			Build()
	}

	return &HugoConfigGenerated{
		BaseEvent: BaseEvent{
			EventBuildID:   buildID,
			EventType:      "HugoConfigGenerated",
			EventTimestamp: time.Now(),
			EventPayload:   payload,
		},
		ConfigHash:    configHash,
		ThemeFeatures: themeFeatures,
	}, nil
}

// SiteGenerated is emitted when the Hugo site is generated.
type SiteGenerated struct {
	BaseEvent
	OutputPath string        `json:"output_path"`
	FileCount  int           `json:"file_count"`
	Duration   time.Duration `json:"duration_ms"`
}

// NewSiteGenerated creates a SiteGenerated event.
func NewSiteGenerated(buildID, outputPath string, fileCount int, duration time.Duration) (*SiteGenerated, error) {
	payload, err := json.Marshal(map[string]any{
		"output_path": outputPath,
		"file_count":  fileCount,
		"duration_ms": duration.Milliseconds(),
	})
	if err != nil {
		return nil, errors.EventStoreError("failed to marshal SiteGenerated payload").
			WithCause(err).
			WithContext("build_id", buildID).
			Build()
	}

	return &SiteGenerated{
		BaseEvent: BaseEvent{
			EventBuildID:   buildID,
			EventType:      "SiteGenerated",
			EventTimestamp: time.Now(),
			EventPayload:   payload,
		},
		OutputPath: outputPath,
		FileCount:  fileCount,
		Duration:   duration,
	}, nil
}

// BuildCompleted is emitted when a build completes successfully.
type BuildCompleted struct {
	BaseEvent
	Status    string            `json:"status"`
	Duration  time.Duration     `json:"duration_ms"`
	Artifacts map[string]string `json:"artifacts"`
}

// NewBuildCompleted creates a BuildCompleted event.
func NewBuildCompleted(buildID, status string, duration time.Duration, artifacts map[string]string) (*BuildCompleted, error) {
	payload, err := json.Marshal(map[string]any{
		"status":      status,
		"duration_ms": duration.Milliseconds(),
		"artifacts":   artifacts,
	})
	if err != nil {
		return nil, errors.EventStoreError("failed to marshal BuildCompleted payload").
			WithCause(err).
			WithContext("build_id", buildID).
			Build()
	}

	return &BuildCompleted{
		BaseEvent: BaseEvent{
			EventBuildID:   buildID,
			EventType:      "BuildCompleted",
			EventTimestamp: time.Now(),
			EventPayload:   payload,
		},
		Status:    status,
		Duration:  duration,
		Artifacts: artifacts,
	}, nil
}

// BuildFailed is emitted when a build fails.
type BuildFailed struct {
	BaseEvent
	Stage string `json:"stage"`
	Error string `json:"error"`
}

// NewBuildFailed creates a BuildFailed event.
func NewBuildFailed(buildID, stage, errorMsg string) (*BuildFailed, error) {
	payload, err := json.Marshal(map[string]any{
		"stage": stage,
		"error": errorMsg,
	})
	if err != nil {
		return nil, errors.EventStoreError("failed to marshal BuildFailed payload").
			WithCause(err).
			WithContext("build_id", buildID).
			WithContext("stage", stage).
			Build()
	}

	return &BuildFailed{
		BaseEvent: BaseEvent{
			EventBuildID:   buildID,
			EventType:      "BuildFailed",
			EventTimestamp: time.Now(),
			EventPayload:   payload,
		},
		Stage: stage,
		Error: errorMsg,
	}, nil
}

// BuildReportData contains the key metrics from a build report.
// This is a subset of hugo.BuildReport optimized for event storage.
type BuildReportData struct {
	Outcome             string           `json:"outcome"`
	Summary             string           `json:"summary"`
	RenderedPages       int              `json:"rendered_pages"`
	ClonedRepositories  int              `json:"cloned_repositories"`
	FailedRepositories  int              `json:"failed_repositories"`
	SkippedRepositories int              `json:"skipped_repositories"`
	StaticRendered      bool             `json:"static_rendered"`
	StageDurations      map[string]int64 `json:"stage_durations_ms"` // stage -> milliseconds
	Errors              []string         `json:"errors,omitempty"`
	Warnings            []string         `json:"warnings,omitempty"`
}

// BuildReportGenerated is emitted when a build report is finalized.
type BuildReportGenerated struct {
	BaseEvent
	Report BuildReportData `json:"report"`
}

// NewBuildReportGenerated creates a BuildReportGenerated event.
func NewBuildReportGenerated(buildID string, report BuildReportData) (*BuildReportGenerated, error) {
	payload, err := json.Marshal(report)
	if err != nil {
		return nil, errors.EventStoreError("failed to marshal BuildReportGenerated payload").
			WithCause(err).
			WithContext("build_id", buildID).
			Build()
	}

	return &BuildReportGenerated{
		BaseEvent: BaseEvent{
			EventBuildID:   buildID,
			EventType:      "BuildReportGenerated",
			EventTimestamp: time.Now(),
			EventPayload:   payload,
		},
		Report: report,
	}, nil
}
