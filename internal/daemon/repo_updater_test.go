package daemon

import (
	"context"
	"testing"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/daemon/events"
	"git.home.luguber.info/inful/docbuilder/internal/git"
	"github.com/stretchr/testify/require"
)

type fakeRemoteHeadChecker struct {
	changed bool
	sha     string
	err     error
}

func (f fakeRemoteHeadChecker) CheckRemoteChanged(_ *git.RemoteHeadCache, _ config.Repository, _ string) (bool, string, error) {
	return f.changed, f.sha, f.err
}

func TestRepoUpdater_WhenRemoteChanges_PublishesRepoUpdatedAndBuildRequested(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	bus := events.NewBus()
	defer bus.Close()

	cache, err := git.NewRemoteHeadCache("")
	require.NoError(t, err)

	checker := fakeRemoteHeadChecker{changed: true, sha: "deadbeef"}
	updater := NewRepoUpdater(bus, checker, cache, func() []config.Repository {
		return []config.Repository{{
			Name:   "repo-1",
			URL:    "https://example.invalid/repo-1.git",
			Branch: "main",
		}}
	})

	repoUpdatedCh, unsubRepoUpdated := events.Subscribe[events.RepoUpdated](bus, 10)
	defer unsubRepoUpdated()

	buildRequestedCh, unsubBuildRequested := events.Subscribe[events.BuildRequested](bus, 10)
	defer unsubBuildRequested()

	go updater.Run(ctx)
	select {
	case <-updater.Ready():
	case <-time.After(250 * time.Millisecond):
		t.Fatal("timed out waiting for repo updater ready")
	}

	require.NoError(t, bus.Publish(context.Background(), events.RepoUpdateRequested{
		JobID:     "job-1",
		Immediate: true,
		RepoURL:   "https://example.invalid/repo-1.git",
		Branch:    "main",
	}))

	select {
	case got := <-repoUpdatedCh:
		require.Equal(t, "job-1", got.JobID)
		require.True(t, got.Changed)
		require.Equal(t, "deadbeef", got.CommitSHA)
		require.True(t, got.Immediate)
	case <-time.After(250 * time.Millisecond):
		t.Fatal("timed out waiting for RepoUpdated")
	}

	select {
	case got := <-buildRequestedCh:
		require.Equal(t, "job-1", got.JobID)
		require.True(t, got.Immediate)
		require.Equal(t, "webhook", got.Reason)
		require.Equal(t, "https://example.invalid/repo-1.git", got.RepoURL)
		require.Equal(t, "main", got.Branch)
	case <-time.After(250 * time.Millisecond):
		t.Fatal("timed out waiting for BuildRequested")
	}
}

func TestRepoUpdater_WhenRemoteUnchanged_PublishesRepoUpdatedButNoBuildRequested(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	bus := events.NewBus()
	defer bus.Close()

	cache, err := git.NewRemoteHeadCache("")
	require.NoError(t, err)

	checker := fakeRemoteHeadChecker{changed: false, sha: "deadbeef"}
	updater := NewRepoUpdater(bus, checker, cache, func() []config.Repository {
		return []config.Repository{{
			Name:   "repo-1",
			URL:    "https://example.invalid/repo-1.git",
			Branch: "main",
		}}
	})

	repoUpdatedCh, unsubRepoUpdated := events.Subscribe[events.RepoUpdated](bus, 10)
	defer unsubRepoUpdated()

	buildRequestedCh, unsubBuildRequested := events.Subscribe[events.BuildRequested](bus, 10)
	defer unsubBuildRequested()

	go updater.Run(ctx)
	select {
	case <-updater.Ready():
	case <-time.After(250 * time.Millisecond):
		t.Fatal("timed out waiting for repo updater ready")
	}

	require.NoError(t, bus.Publish(context.Background(), events.RepoUpdateRequested{
		JobID:     "job-1",
		Immediate: true,
		RepoURL:   "https://example.invalid/repo-1.git",
		Branch:    "main",
	}))

	select {
	case got := <-repoUpdatedCh:
		require.Equal(t, "job-1", got.JobID)
		require.False(t, got.Changed)
	case <-time.After(250 * time.Millisecond):
		t.Fatal("timed out waiting for RepoUpdated")
	}

	select {
	case <-buildRequestedCh:
		t.Fatal("expected no BuildRequested when repo unchanged")
	case <-time.After(75 * time.Millisecond):
		// ok
	}
}
