package daemon

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/eventstore"
	"git.home.luguber.info/inful/docbuilder/internal/hugo/models"
)

type noopBuilder struct{}

func (noopBuilder) Build(context.Context, *BuildJob) (*models.BuildReport, error) {
	return &models.BuildReport{}, nil
}

// Helper function to create a minimal daemon for status testing.
func newTestDaemon() *Daemon {
	d := &Daemon{
		config:          &config.Config{},
		startTime:       time.Now(),
		discoveryCache:  NewDiscoveryCache(),
		discoveryRunner: &DiscoveryRunner{},
	}
	d.status.Store(StatusRunning)
	return d
}

// TestGenerateStatusData_BasicInfo tests basic daemon info generation.
func TestGenerateStatusData_BasicInfo(t *testing.T) {
	d := newTestDaemon()
	d.startTime = time.Now().Add(-1 * time.Hour)
	d.configFilePath = "/path/to/config.yaml"

	status, err := d.GenerateStatusData()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if status.DaemonInfo.Status != StatusRunning {
		t.Errorf("expected status %s, got %s", StatusRunning, status.DaemonInfo.Status)
	}
	if status.DaemonInfo.ConfigFile != "/path/to/config.yaml" {
		t.Errorf("expected config file %s, got %s", "/path/to/config.yaml", status.DaemonInfo.ConfigFile)
	}
	if status.DaemonInfo.Uptime == "" {
		t.Error("expected uptime to be set")
	}
}

// TestGenerateStatusData_NoStatusLoaded tests fallback when status not initialized.
func TestGenerateStatusData_NoStatusLoaded(t *testing.T) {
	d := newTestDaemon()
	d.status = atomic.Value{} // Not initialized

	status, err := d.GenerateStatusData()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if status.DaemonInfo.Status != StatusStopped {
		t.Errorf("expected fallback status %s, got %s", StatusStopped, status.DaemonInfo.Status)
	}
}

// TestGenerateStatusData_NoConfigFile tests fallback for missing config file.
func TestGenerateStatusData_NoConfigFile(t *testing.T) {
	d := newTestDaemon()
	d.configFilePath = "" // Empty

	status, err := d.GenerateStatusData()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if status.DaemonInfo.ConfigFile != "config.yaml" {
		t.Errorf("expected fallback config file 'config.yaml', got %s", status.DaemonInfo.ConfigFile)
	}
}

// TestGenerateStatusData_WithBuildQueue tests with build queue present.
func TestGenerateStatusData_WithBuildQueue(t *testing.T) {
	bq := NewBuildQueue(10, 1, noopBuilder{})
	// Add some jobs to queue (do not start workers; keep queued)
	if err := bq.Enqueue(&BuildJob{ID: "job1"}); err != nil {
		t.Fatalf("enqueue job1: %v", err)
	}
	if err := bq.Enqueue(&BuildJob{ID: "job2"}); err != nil {
		t.Fatalf("enqueue job2: %v", err)
	}

	d := newTestDaemon()
	d.buildQueue = bq

	status, err := d.GenerateStatusData()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if status.BuildStatus.QueueLength != 2 {
		t.Errorf("expected queue length 2, got %d", status.BuildStatus.QueueLength)
	}
}

// TestGenerateStatusData_NoBuildQueue tests without build queue.
func TestGenerateStatusData_NoBuildQueue(t *testing.T) {
	d := newTestDaemon()
	d.buildQueue = nil
	d.queueLength = 5 // Fallback value

	status, err := d.GenerateStatusData()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if status.BuildStatus.QueueLength != 5 {
		t.Errorf("expected queue length 5, got %d", status.BuildStatus.QueueLength)
	}
}

// TestGenerateStatusData_WithBuildProjection tests with build projection data.
func TestGenerateStatusData_WithBuildProjection(t *testing.T) {
	// Create event store and projection
	store, err := eventstore.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer func() { _ = store.Close() }()

	projection := eventstore.NewBuildHistoryProjection(store, 100)

	// Create and apply build events
	buildID := "test-build-1"

	// Start event
	startEvent, _ := eventstore.NewBuildStarted(buildID, eventstore.BuildStartedMeta{
		Type:     "manual",
		Priority: 1,
		WorkerID: "worker-1",
	})
	projection.Apply(startEvent)

	// Report event with data
	reportData := eventstore.BuildReportData{
		StageDurations: map[string]int64{
			"clone":    1000,
			"discover": 500,
			"hugo":     2000,
		},
		Outcome:             "success",
		Summary:             "Build completed successfully",
		RenderedPages:       42,
		ClonedRepositories:  3,
		FailedRepositories:  1,
		SkippedRepositories: 2,
		StaticRendered:      true,
		Errors:              []string{"error1"},
		Warnings:            []string{"warning1", "warning2"},
	}
	reportEvent, _ := eventstore.NewBuildReportGenerated(buildID, reportData)
	projection.Apply(reportEvent)

	// Completed event
	completedEvent, _ := eventstore.NewBuildCompleted(buildID, "completed", 5*time.Second, map[string]string{})
	projection.Apply(completedEvent)

	d := newTestDaemon()
	d.buildProjection = projection

	status, err := d.GenerateStatusData()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify stage durations were converted
	if len(status.BuildStatus.LastBuildStages) != 3 {
		t.Errorf("expected 3 stages, got %d", len(status.BuildStatus.LastBuildStages))
	}
	if status.BuildStatus.LastBuildStages["clone"] != "1s" {
		t.Errorf("expected '1s', got %s", status.BuildStatus.LastBuildStages["clone"])
	}

	// Verify outcome and summary
	if status.BuildStatus.LastBuildOutcome != "success" {
		t.Errorf("expected outcome 'success', got %s", status.BuildStatus.LastBuildOutcome)
	}
	if status.BuildStatus.LastBuildSummary != "Build completed successfully" {
		t.Errorf("expected summary, got %s", status.BuildStatus.LastBuildSummary)
	}

	// Verify pointers are set correctly
	if status.BuildStatus.RenderedPages == nil || *status.BuildStatus.RenderedPages != 42 {
		t.Error("expected RenderedPages to be 42")
	}
	if status.BuildStatus.ClonedRepositories == nil || *status.BuildStatus.ClonedRepositories != 3 {
		t.Error("expected ClonedRepositories to be 3")
	}
	if status.BuildStatus.FailedRepositories == nil || *status.BuildStatus.FailedRepositories != 1 {
		t.Error("expected FailedRepositories to be 1")
	}
	if status.BuildStatus.SkippedRepositories == nil || *status.BuildStatus.SkippedRepositories != 2 {
		t.Error("expected SkippedRepositories to be 2")
	}
	if status.BuildStatus.StaticRendered == nil || !*status.BuildStatus.StaticRendered {
		t.Error("expected StaticRendered to be true")
	}

	// Verify errors and warnings
	if len(status.BuildStatus.LastBuildErrors) != 1 {
		t.Errorf("expected 1 error, got %d", len(status.BuildStatus.LastBuildErrors))
	}
	if len(status.BuildStatus.LastBuildWarnings) != 2 {
		t.Errorf("expected 2 warnings, got %d", len(status.BuildStatus.LastBuildWarnings))
	}
}

// TestGenerateStatusData_NoBuildProjection tests without build projection.
func TestGenerateStatusData_NoBuildProjection(t *testing.T) {
	d := newTestDaemon()
	d.buildProjection = nil

	status, err := d.GenerateStatusData()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should not panic and fields should be empty
	if len(status.BuildStatus.LastBuildStages) != 0 {
		t.Errorf("expected no stages, got %d", len(status.BuildStatus.LastBuildStages))
	}
}

// TestGenerateStatusData_EmptyReportData tests with projection but no report data.
func TestGenerateStatusData_EmptyReportData(t *testing.T) {
	// Create projection with a build but no report data
	store, err := eventstore.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer func() { _ = store.Close() }()

	projection := eventstore.NewBuildHistoryProjection(store, 100)

	buildID := "test-build-2"
	startEvent, _ := eventstore.NewBuildStarted(buildID, eventstore.BuildStartedMeta{})
	projection.Apply(startEvent)

	// Complete without report
	completedEvent, _ := eventstore.NewBuildCompleted(buildID, "completed", 1*time.Second, map[string]string{})
	projection.Apply(completedEvent)

	d := newTestDaemon()
	d.buildProjection = projection

	status, err := d.GenerateStatusData()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should handle nil report data gracefully
	if len(status.BuildStatus.LastBuildStages) != 0 {
		t.Errorf("expected no stages, got %d", len(status.BuildStatus.LastBuildStages))
	}
}
