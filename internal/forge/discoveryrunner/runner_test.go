package discoveryrunner

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/forge"
)

func TestRunner_Run_WhenDiscoveryFails_CachesErrorAndDoesNotEnqueue(t *testing.T) {
	const jobID = "job-1"

	cache := NewCache()
	metrics := &fakeMetrics{}

	discovery := &fakeDiscovery{
		err: forgeError("discovery failed"),
	}

	r := New(Config{
		Discovery:      discovery,
		DiscoveryCache: cache,
		Metrics:        metrics,
		Now:            func() time.Time { return time.Unix(123, 0).UTC() },
		NewJobID:       func() string { return jobID },
		Config:         &config.Config{Version: "2.0"},
	})

	err := r.Run(context.Background())
	require.Error(t, err)

	_, cachedErr := cache.Get()
	require.Error(t, cachedErr)
}

func TestRunner_Run_WhenReposDiscovered_UpdatesCacheAndRequestsBuild(t *testing.T) {
	const jobID = "job-1"

	cache := NewCache()
	metrics := &fakeMetrics{}
	appCfg := &config.Config{Version: "2.0"}

	r1 := &forge.Repository{Name: "r1", CloneURL: "https://example.com/r1.git", Metadata: map[string]string{"forge_name": "f"}}
	r2 := &forge.Repository{Name: "r2", CloneURL: "https://example.com/r2.git", Metadata: map[string]string{"forge_name": "f"}}

	discovery := &fakeDiscovery{
		result: &forge.DiscoveryResult{
			Repositories: []*forge.Repository{r1, r2},
			Filtered:     []*forge.Repository{},
			Errors:       map[string]error{},
			Timestamp:    time.Unix(100, 0).UTC(),
			Duration:     2 * time.Second,
		},
		converted: []config.Repository{{Name: "r1"}, {Name: "r2"}},
	}

	var (
		calledID     string
		calledReason string
	)

	r := New(Config{
		Discovery:      discovery,
		DiscoveryCache: cache,
		Metrics:        metrics,
		BuildRequester: func(_ context.Context, jobID, reason string) {
			calledID = jobID
			calledReason = reason
		},
		Now:      func() time.Time { return time.Unix(123, 0).UTC() },
		NewJobID: func() string { return "job-1" },
		Config:   appCfg,
	})

	err := r.Run(context.Background())
	require.NoError(t, err)

	res, cachedErr := cache.Get()
	require.NoError(t, cachedErr)
	require.Same(t, discovery.result, res)
	require.Equal(t, jobID, calledID)
	require.Equal(t, "discovery", calledReason)
}

func TestRunner_Run_WhenBuildOnDiscoveryDisabled_UpdatesCacheAndDoesNotEnqueueBuild(t *testing.T) {
	const jobID = "job-1"

	cache := NewCache()
	metrics := &fakeMetrics{}
	appCfg := &config.Config{Version: "2.0"}
	if appCfg.Daemon == nil {
		appCfg.Daemon = &config.DaemonConfig{}
	}
	appCfg.Daemon.Sync.BuildOnDiscovery = ptr(false)

	discovery := &fakeDiscovery{
		result: &forge.DiscoveryResult{
			Repositories: []*forge.Repository{
				{Name: "r1", CloneURL: "https://example.com/r1.git", Metadata: map[string]string{"forge_name": "f"}},
			},
			Filtered:  []*forge.Repository{},
			Errors:    map[string]error{},
			Timestamp: time.Unix(100, 0).UTC(),
		},
		converted: []config.Repository{{Name: "r1"}},
	}

	r := New(Config{
		Discovery:      discovery,
		DiscoveryCache: cache,
		Metrics:        metrics,
		Now:            func() time.Time { return time.Unix(123, 0).UTC() },
		NewJobID:       func() string { return jobID },
		Config:         appCfg,
	})

	require.NoError(t, r.Run(context.Background()))

	_, cachedErr := cache.Get()
	require.NoError(t, cachedErr)
}

func TestRunner_Run_WhenBuildRequesterProvided_DoesNotEnqueueBuild(t *testing.T) {
	const jobID = "job-1"

	cache := NewCache()
	metrics := &fakeMetrics{}
	appCfg := &config.Config{Version: "2.0"}

	discovery := &fakeDiscovery{
		result: &forge.DiscoveryResult{
			Repositories: []*forge.Repository{
				{Name: "r1", CloneURL: "https://example.com/r1.git", Metadata: map[string]string{"forge_name": "f"}},
			},
			Filtered:  []*forge.Repository{},
			Errors:    map[string]error{},
			Timestamp: time.Unix(100, 0).UTC(),
		},
		converted: []config.Repository{{Name: "r1"}},
	}

	calls := 0
	r := New(Config{
		Discovery:      discovery,
		DiscoveryCache: cache,
		Metrics:        metrics,
		BuildRequester: func(_ context.Context, _, _ string) { calls++ },
		Now:            func() time.Time { return time.Unix(123, 0).UTC() },
		NewJobID:       func() string { return jobID },
		Config:         appCfg,
	})

	require.NoError(t, r.Run(context.Background()))
	require.Equal(t, 1, calls)
}

type fakeDiscovery struct {
	err       error
	result    *forge.DiscoveryResult
	converted []config.Repository
}

func (f *fakeDiscovery) DiscoverAll(_ context.Context) (*forge.DiscoveryResult, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.result, nil
}

func (f *fakeDiscovery) ConvertToConfigRepositories(_ []*forge.Repository, _ *forge.Manager) []config.Repository {
	return f.converted
}

type fakeMetrics struct {
	counters map[string]int
}

func (m *fakeMetrics) IncrementCounter(name string) {
	if m.counters == nil {
		m.counters = map[string]int{}
	}
	m.counters[name]++
}

func (m *fakeMetrics) RecordHistogram(string, float64) {}
func (m *fakeMetrics) SetGauge(string, int64)          {}

func ptr[T any](v T) *T { return &v }
