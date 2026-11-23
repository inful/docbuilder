package daemon

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/daemon/events"
)

// The state manager of a daemon that removes a repository.
type removalStateManager struct {
	fakeDaemonStateManager
}

func (*removalStateManager) RemoveRepositoryState(string) {}

// The URL of a repository can carry a token, and the log line ends up
// in a log collector.
func TestTheRemovedRepositoryIsLoggedWithoutItsCredentials(t *testing.T) {
	var logged bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logged, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	d := &Daemon{
		state: &stateStack{stateManager: &removalStateManager{}},
		build: &buildStack{},
	}
	//nolint:gosec // the token of the test is not a credential
	d.handleRepoRemoved(events.RepoRemoved{
		RepoURL:  "https://oauth2:secret-token@example.com/org/repo.git",
		RepoName: "repo",
	})

	if strings.Contains(logged.String(), "secret-token") {
		t.Fatalf("the token of the repository URL is in the log: %s", logged.String())
	}
	if !strings.Contains(logged.String(), "example.com/org/repo.git") {
		t.Fatalf("the repository is not in the log: %s", logged.String())
	}
}
