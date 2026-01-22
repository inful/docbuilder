package discoveryrunner

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"git.home.luguber.info/inful/docbuilder/internal/build/queue"
	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/forge"
)

func TestRunner_Run_WhenDiscoveryFails_CachesErrorAndDoesNotEnqueue(t *testing.T) {
	cache := NewCache()
	metrics := &fakeMetrics{}
	enq := &fakeEnqueuer{}

	discovery := &fakeDiscovery{
		err: forgeError("discovery failed"),
	}

	r := New(Config{
		Discovery:      discovery,
		DiscoveryCache: cache,
		Metrics:        metrics,
		BuildQueue:     enq,
		Now:            func() time.Time { return time.Unix(123, 0).UTC() },
		NewJobID:       func() string { return "job-1" },
		Config:         &config.Config{Version: "2.0"},
	})

	err := r.Run(context.Background())
	require.Error(t, err)

	_, cachedErr := cache.Get()
	require.Error(t, cachedErr)
	require.Equal(t, 0, enq.calls)
}

func TestRunner_Run_WhenReposDiscovered_UpdatesCacheAndEnqueuesBuild(t *testing.T) {
	cache := NewCache()
	metrics := &fakeMetrics{}
	enq := &fakeEnqueuer{}
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

	r := New(Config{
		Discovery:      discovery,
		DiscoveryCache: cache,
		Metrics:        metrics,
		BuildQueue:     enq,
		Now:            func() time.Time { return time.Unix(123, 0).UTC() },
		NewJobID:       func() string { return "job-1" },
		Config:         appCfg,
	})

	err := r.Run(context.Background())
	require.NoError(t, err)

	res, cachedErr := cache.Get()
	require.NoError(t, cachedErr)
	require.Same(t, discovery.result, res)
	require.Equal(t, 1, enq.calls)
	require.NotNil(t, enq.last)
	require.Equal(t, "job-1", enq.last.ID)
	require.Equal(t, queue.BuildTypeDiscovery, enq.last.Type)
	require.NotNil(t, enq.last.TypedMeta)
	require.Same(t, appCfg, enq.last.TypedMeta.V2Config)
	require.Len(t, enq.last.TypedMeta.Repositories, 2)
}

type fakeDiscovery struct {
	result    *forge.DiscoveryResult
	err       error
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

type fakeEnqueuer struct {
	calls int
	last  *queue.BuildJob
}

func (e *fakeEnqueuer) Enqueue(job *queue.BuildJob) error {
	e.calls++
	e.last = job
	return nil
}
