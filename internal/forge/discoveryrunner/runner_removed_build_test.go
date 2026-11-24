package discoveryrunner

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/forge"
)

// A repository that was removed has to leave the site. The build that
// publishes the site without it is the one that takes its pages away,
// so a removal asks for a build - even when no repository is left.
func TestRunner_Run_WhenTheLastRepositoryWasRemoved_EnqueuesBuild(t *testing.T) {
	cache := NewCache()
	metrics := &fakeMetrics{}
	enq := &fakeEnqueuer{}

	// The last discovery found one repository.
	prev := &forge.Repository{Name: "r1", CloneURL: "https://example.com/r1.git"}
	cache.Update(&forge.DiscoveryResult{Repositories: []*forge.Repository{prev}})

	// The repository is gone: the discovery finds nothing.
	discovery := &fakeDiscovery{
		result: &forge.DiscoveryResult{
			Repositories: []*forge.Repository{},
			Filtered:     []*forge.Repository{},
			Errors:       map[string]error{},
			Timestamp:    time.Unix(200, 0).UTC(),
			Duration:     time.Second,
		},
	}

	removed := 0
	r := New(Config{
		Discovery:      discovery,
		DiscoveryCache: cache,
		Metrics:        metrics,
		BuildQueue:     enq,
		RepoRemoved: func(_ context.Context, _, _ string) {
			removed++
		},
		Now:    func() time.Time { return time.Unix(123, 0).UTC() },
		Config: &config.Config{Version: "2.0"},
	})

	err := r.Run(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, removed, "the removal was not reported")
	require.Equal(t, 1, enq.calls, "the build that takes the pages away was not asked for")
}
