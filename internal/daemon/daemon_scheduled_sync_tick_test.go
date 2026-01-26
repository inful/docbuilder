package daemon

import (
	"context"
	"sync"
	"testing"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/build/queue"
	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/forge"
	"git.home.luguber.info/inful/docbuilder/internal/forge/discoveryrunner"
	"github.com/stretchr/testify/require"
)

type fakeDiscovery struct {
	result *forge.DiscoveryResult
}

func (f *fakeDiscovery) DiscoverAll(ctx context.Context) (*forge.DiscoveryResult, error) {
	return f.result, nil
}

func (f *fakeDiscovery) ConvertToConfigRepositories(repos []*forge.Repository, forgeManager *forge.Manager) []config.Repository {
	converted := make([]config.Repository, 0, len(repos))
	for _, repo := range repos {
		converted = append(converted, config.Repository{
			Name:   repo.Name,
			URL:    repo.CloneURL,
			Branch: repo.DefaultBranch,
			Paths:  []string{"docs"},
		})
	}
	return converted
}

type fakeBuildQueue struct {
	mu   sync.Mutex
	jobs []*queue.BuildJob
}

func (f *fakeBuildQueue) Enqueue(job *queue.BuildJob) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.jobs = append(f.jobs, job)
	return nil
}

func (f *fakeBuildQueue) Jobs() []*queue.BuildJob {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]*queue.BuildJob(nil), f.jobs...)
}

func TestDaemon_runScheduledSyncTick(t *testing.T) {
	t.Run("does nothing when daemon is not running", func(t *testing.T) {
		cfg := &config.Config{
			Daemon: &config.DaemonConfig{Sync: config.SyncConfig{Schedule: "0 */4 * * *"}},
			Forges: []*config.ForgeConfig{{Name: "forge-1", Type: config.ForgeForgejo}},
		}

		fakeQ := &fakeBuildQueue{}
		runner := NewDiscoveryRunner(DiscoveryRunnerConfig{
			Discovery:      &fakeDiscovery{result: &forge.DiscoveryResult{Repositories: []*forge.Repository{{Name: "repo-1", CloneURL: "https://example.invalid/repo-1.git", DefaultBranch: "main"}}}},
			ForgeManager:   nil,
			DiscoveryCache: discoveryrunner.NewCache(),
			Metrics:        nil,
			StateManager:   nil,
			BuildQueue:     fakeQ,
			LiveReload:     nil,
			Config:         cfg,
			Now:            func() time.Time { return time.Unix(123, 0).UTC() },
			NewJobID:       func() string { return "job-1" },
		})

		d := &Daemon{config: cfg, discoveryRunner: runner}
		d.status.Store(StatusStopped)

		d.runScheduledSyncTick(context.Background(), "0 */4 * * *")
		require.Len(t, fakeQ.Jobs(), 0)
	})

	t.Run("runs discovery and enqueues discovery build", func(t *testing.T) {
		cfg := &config.Config{
			Daemon: &config.DaemonConfig{Sync: config.SyncConfig{Schedule: "0 */4 * * *"}},
			Forges: []*config.ForgeConfig{{Name: "forge-1", Type: config.ForgeForgejo}},
		}

		fakeQ := &fakeBuildQueue{}
		runner := NewDiscoveryRunner(DiscoveryRunnerConfig{
			Discovery: &fakeDiscovery{result: &forge.DiscoveryResult{Repositories: []*forge.Repository{{
				Name:          "repo-1",
				CloneURL:      "https://example.invalid/repo-1.git",
				DefaultBranch: "main",
			}}}},
			ForgeManager:   nil,
			DiscoveryCache: discoveryrunner.NewCache(),
			Metrics:        nil,
			StateManager:   nil,
			BuildQueue:     fakeQ,
			LiveReload:     nil,
			Config:         cfg,
			Now:            func() time.Time { return time.Unix(123, 0).UTC() },
			NewJobID:       func() string { return "job-1" },
		})

		d := &Daemon{config: cfg, discoveryRunner: runner}
		d.status.Store(StatusRunning)

		d.runScheduledSyncTick(context.Background(), "0 */4 * * *")

		jobs := fakeQ.Jobs()
		require.Len(t, jobs, 1)
		require.Equal(t, "job-1", jobs[0].ID)
		require.Equal(t, queue.BuildTypeDiscovery, jobs[0].Type)
		require.Equal(t, 1, len(jobs[0].TypedMeta.Repositories))
		require.Equal(t, "repo-1", jobs[0].TypedMeta.Repositories[0].Name)
	})

	t.Run("scheduler starts and stops cleanly with scheduled jobs", func(t *testing.T) {
		s, err := NewScheduler()
		require.NoError(t, err)
		ctx := context.Background()

		cfg := &config.Config{Daemon: &config.DaemonConfig{Sync: config.SyncConfig{Schedule: "0 */4 * * *"}}}
		d := &Daemon{config: cfg, scheduler: s}
		err = d.schedulePeriodicJobs(ctx)
		require.NoError(t, err)

		s.Start(ctx)
		require.NoError(t, s.Stop(ctx))
	})
}
