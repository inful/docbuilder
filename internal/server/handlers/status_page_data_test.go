package handlers

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/eventstore"
	"git.home.luguber.info/inful/docbuilder/internal/forge"
)

type fakeStatusProvider struct {
	status         string
	startTime      time.Time
	activeJobs     int
	queueLength    int
	cfg            *config.Config
	configFilePath string
	lastBuildTime  *time.Time
	buildProj      *eventstore.BuildHistoryProjection
	lastDiscovery  *time.Time
	discoveryRes   *forge.DiscoveryResult
	discoveryErr   error
}

func (f fakeStatusProvider) GetStatus() string { return f.status }
func (f fakeStatusProvider) GetStartTime() time.Time {
	return f.startTime
}
func (f fakeStatusProvider) GetActiveJobs() int           { return f.activeJobs }
func (f fakeStatusProvider) GetQueueLength() int          { return f.queueLength }
func (f fakeStatusProvider) GetConfig() *config.Config    { return f.cfg }
func (f fakeStatusProvider) GetConfigFilePath() string    { return f.configFilePath }
func (f fakeStatusProvider) GetLastBuildTime() *time.Time { return f.lastBuildTime }
func (f fakeStatusProvider) GetBuildProjection() *eventstore.BuildHistoryProjection {
	return f.buildProj
}
func (f fakeStatusProvider) GetLastDiscovery() *time.Time { return f.lastDiscovery }
func (f fakeStatusProvider) GetDiscoveryResult() (*forge.DiscoveryResult, error) {
	return f.discoveryRes, f.discoveryErr
}

func TestGenerateStatusData_BasicInfo(t *testing.T) {
	p := fakeStatusProvider{
		status:         "running",
		startTime:      time.Now().Add(-1 * time.Hour),
		cfg:            &config.Config{Version: "2.0"},
		configFilePath: "/path/to/config.yaml",
	}

	data, err := GenerateStatusData(context.Background(), p)
	require.NoError(t, err)
	require.Equal(t, "running", data.DaemonInfo.Status)
	require.Equal(t, "/path/to/config.yaml", data.DaemonInfo.ConfigFile)
	require.NotEmpty(t, data.DaemonInfo.Uptime)
}

func TestGenerateStatusData_EmptyStatusFallsBackStopped(t *testing.T) {
	p := fakeStatusProvider{startTime: time.Now().Add(-1 * time.Hour), cfg: &config.Config{Version: "2.0"}}

	data, err := GenerateStatusData(context.Background(), p)
	require.NoError(t, err)
	require.Equal(t, "stopped", data.DaemonInfo.Status)
}

func TestGenerateStatusData_WithBuildProjection_ConvertsStagesAndPopulatesReportFields(t *testing.T) {
	store, err := eventstore.NewSQLiteStore(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })

	projection := eventstore.NewBuildHistoryProjection(store, 100)
	buildID := "test-build-1"

	startEvent, _ := eventstore.NewBuildStarted(buildID, eventstore.BuildStartedMeta{Type: "manual", Priority: 1, WorkerID: "worker-1"})
	projection.Apply(startEvent)

	reportData := eventstore.BuildReportData{
		StageDurations: map[string]int64{"clone": 1000, "discover": 500, "hugo": 2000},
		Outcome:        "success",
		Summary:        "Build completed successfully",
		RenderedPages:  42,
		Errors:         []string{"error1"},
		Warnings:       []string{"warning1", "warning2"},
		StaticRendered: true,
	}
	reportEvent, _ := eventstore.NewBuildReportGenerated(buildID, reportData)
	projection.Apply(reportEvent)

	completedEvent, _ := eventstore.NewBuildCompleted(buildID, "completed", 5*time.Second, map[string]string{})
	projection.Apply(completedEvent)

	p := fakeStatusProvider{status: "running", startTime: time.Now().Add(-1 * time.Hour), cfg: &config.Config{Version: "2.0"}, buildProj: projection}

	data, err := GenerateStatusData(context.Background(), p)
	require.NoError(t, err)
	require.Len(t, data.BuildStatus.LastBuildStages, 3)
	require.Equal(t, "1s", data.BuildStatus.LastBuildStages["clone"])
	require.Equal(t, "success", data.BuildStatus.LastBuildOutcome)
	require.Equal(t, "Build completed successfully", data.BuildStatus.LastBuildSummary)
	require.NotNil(t, data.BuildStatus.RenderedPages)
	require.Equal(t, 42, *data.BuildStatus.RenderedPages)
	require.True(t, data.BuildStatus.StaticRendered != nil && *data.BuildStatus.StaticRendered)
	require.Len(t, data.BuildStatus.LastBuildErrors, 1)
	require.Len(t, data.BuildStatus.LastBuildWarnings, 2)
}
