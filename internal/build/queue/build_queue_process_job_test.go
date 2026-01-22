package queue

import (
	"context"
	"errors"
	"testing"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/eventstore"
	"git.home.luguber.info/inful/docbuilder/internal/hugo/models"
)

// Mock event emitter for testing.
type mockEventEmitter struct {
	buildStartedCalls   int
	buildCompletedCalls int
	buildFailedCalls    int
	buildReportCalls    int
	emitStartedErr      error
	emitCompletedErr    error
	emitFailedErr       error
	emitReportErr       error
}

func (m *mockEventEmitter) EmitBuildStarted(ctx context.Context, buildID string, meta eventstore.BuildStartedMeta) error {
	m.buildStartedCalls++
	return m.emitStartedErr
}

func (m *mockEventEmitter) EmitBuildCompleted(ctx context.Context, buildID string, duration time.Duration, artifacts map[string]string) error {
	m.buildCompletedCalls++
	return m.emitCompletedErr
}

func (m *mockEventEmitter) EmitBuildFailed(ctx context.Context, buildID, stage, errorMsg string) error {
	m.buildFailedCalls++
	return m.emitFailedErr
}

func (m *mockEventEmitter) EmitBuildReport(ctx context.Context, buildID string, report *models.BuildReport) error {
	m.buildReportCalls++
	return m.emitReportErr
}

// Mock builder for processJob testing.
type mockProcessJobBuilder struct {
	buildErr    error
	buildReport *models.BuildReport
}

func (m *mockProcessJobBuilder) Build(ctx context.Context, job *BuildJob) (*models.BuildReport, error) {
	return m.buildReport, m.buildErr
}

func TestProcessJob_SuccessWithReport(t *testing.T) {
	emitter := &mockEventEmitter{}
	builder := &mockProcessJobBuilder{buildReport: &models.BuildReport{Files: 10, Repositories: 2}}

	bq := &BuildQueue{
		eventEmitter: emitter,
		builder:      builder,
		active:       make(map[string]*BuildJob),
		history:      make([]*BuildJob, 0),
		historySize:  10,
	}

	job := &BuildJob{ID: "test-job-1", Type: BuildTypeManual, Priority: PriorityNormal, Status: BuildStatusQueued}
	bq.processJob(t.Context(), job, "worker-1")

	if job.Status != BuildStatusCompleted {
		t.Fatalf("expected status %s, got %s", BuildStatusCompleted, job.Status)
	}

	if emitter.buildStartedCalls != 1 {
		t.Fatalf("expected 1 buildStarted call, got %d", emitter.buildStartedCalls)
	}
	if emitter.buildReportCalls != 1 {
		t.Fatalf("expected 1 buildReport call, got %d", emitter.buildReportCalls)
	}
	if emitter.buildCompletedCalls != 1 {
		t.Fatalf("expected 1 buildCompleted call, got %d", emitter.buildCompletedCalls)
	}
	if emitter.buildFailedCalls != 0 {
		t.Fatalf("expected 0 buildFailed calls, got %d", emitter.buildFailedCalls)
	}

	if job.TypedMeta == nil || job.TypedMeta.BuildReport == nil {
		t.Fatalf("expected BuildReport to be stored in TypedMeta")
	}
}

func TestProcessJob_SuccessWithoutReport(t *testing.T) {
	emitter := &mockEventEmitter{}
	builder := &mockProcessJobBuilder{buildReport: nil}

	bq := &BuildQueue{
		eventEmitter: emitter,
		builder:      builder,
		active:       make(map[string]*BuildJob),
		history:      make([]*BuildJob, 0),
		historySize:  10,
	}

	job := &BuildJob{ID: "test-job-2", Type: BuildTypeManual, Priority: PriorityNormal, Status: BuildStatusQueued}
	bq.processJob(t.Context(), job, "worker-1")

	if job.Status != BuildStatusCompleted {
		t.Fatalf("expected status %s, got %s", BuildStatusCompleted, job.Status)
	}
	if emitter.buildReportCalls != 0 {
		t.Fatalf("expected 0 buildReport calls, got %d", emitter.buildReportCalls)
	}
	if emitter.buildCompletedCalls != 1 {
		t.Fatalf("expected 1 buildCompleted call, got %d", emitter.buildCompletedCalls)
	}
}

func TestProcessJob_Failure(t *testing.T) {
	emitter := &mockEventEmitter{}
	buildErr := errors.New("build failed")
	builder := &mockProcessJobBuilder{buildErr: buildErr}

	bq := &BuildQueue{
		eventEmitter: emitter,
		builder:      builder,
		active:       make(map[string]*BuildJob),
		history:      make([]*BuildJob, 0),
		historySize:  10,
	}

	job := &BuildJob{ID: "test-job-3", Type: BuildTypeManual, Priority: PriorityNormal, Status: BuildStatusQueued}
	bq.processJob(t.Context(), job, "worker-1")

	if job.Status != BuildStatusFailed {
		t.Fatalf("expected status %s, got %s", BuildStatusFailed, job.Status)
	}
	if job.Error != buildErr.Error() {
		t.Fatalf("expected error %q, got %q", buildErr.Error(), job.Error)
	}
	if emitter.buildFailedCalls != 1 {
		t.Fatalf("expected 1 buildFailed call, got %d", emitter.buildFailedCalls)
	}
	if emitter.buildCompletedCalls != 0 {
		t.Fatalf("expected 0 buildCompleted calls, got %d", emitter.buildCompletedCalls)
	}
}

func TestProcessJob_FailureWithReport(t *testing.T) {
	emitter := &mockEventEmitter{}
	buildErr := errors.New("partial build failure")
	builder := &mockProcessJobBuilder{buildErr: buildErr, buildReport: &models.BuildReport{Files: 5, Repositories: 1}}

	bq := &BuildQueue{
		eventEmitter: emitter,
		builder:      builder,
		active:       make(map[string]*BuildJob),
		history:      make([]*BuildJob, 0),
		historySize:  10,
	}

	job := &BuildJob{ID: "test-job-4", Type: BuildTypeManual, Priority: PriorityNormal, Status: BuildStatusQueued}
	bq.processJob(t.Context(), job, "worker-1")

	if job.Status != BuildStatusFailed {
		t.Fatalf("expected status %s, got %s", BuildStatusFailed, job.Status)
	}
	if emitter.buildReportCalls != 1 {
		t.Fatalf("expected 1 buildReport call, got %d", emitter.buildReportCalls)
	}
	if emitter.buildFailedCalls != 1 {
		t.Fatalf("expected 1 buildFailed call, got %d", emitter.buildFailedCalls)
	}
}

func TestProcessJob_NoEventEmitter(t *testing.T) {
	builder := &mockProcessJobBuilder{buildReport: &models.BuildReport{Files: 10}}
	bq := &BuildQueue{eventEmitter: nil, builder: builder, active: make(map[string]*BuildJob), history: make([]*BuildJob, 0), historySize: 10}

	job := &BuildJob{ID: "test-job-5", Type: BuildTypeManual, Priority: PriorityNormal, Status: BuildStatusQueued}
	bq.processJob(t.Context(), job, "worker-1")

	if job.Status != BuildStatusCompleted {
		t.Fatalf("expected status %s, got %s", BuildStatusCompleted, job.Status)
	}
}

func TestProcessJob_EventEmitterErrors(t *testing.T) {
	emitter := &mockEventEmitter{
		emitStartedErr:   errors.New("started emit error"),
		emitReportErr:    errors.New("report emit error"),
		emitCompletedErr: errors.New("completed emit error"),
	}
	builder := &mockProcessJobBuilder{buildReport: &models.BuildReport{Files: 10}}

	bq := &BuildQueue{
		eventEmitter: emitter,
		builder:      builder,
		active:       make(map[string]*BuildJob),
		history:      make([]*BuildJob, 0),
		historySize:  10,
	}

	job := &BuildJob{ID: "test-job-6", Type: BuildTypeManual, Priority: PriorityNormal, Status: BuildStatusQueued}
	bq.processJob(t.Context(), job, "worker-1")

	if job.Status != BuildStatusCompleted {
		t.Fatalf("expected status %s, got %s", BuildStatusCompleted, job.Status)
	}
	if emitter.buildStartedCalls != 1 {
		t.Fatalf("expected 1 buildStarted call, got %d", emitter.buildStartedCalls)
	}
	if emitter.buildReportCalls != 1 {
		t.Fatalf("expected 1 buildReport call, got %d", emitter.buildReportCalls)
	}
	if emitter.buildCompletedCalls != 1 {
		t.Fatalf("expected 1 buildCompleted call, got %d", emitter.buildCompletedCalls)
	}
}
