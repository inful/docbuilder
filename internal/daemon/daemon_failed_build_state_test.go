package daemon

import (
	"context"
	"testing"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/hugo/models"
)

// The state is updated after every build, also after a failed one.
//
// The state was only updated after a successful build: after a failed
// build every repository still reported the result of the last
// successful build, and the time of the last build stayed the time of
// the last successful one.
func TestAFailedBuildUpdatesTheState(t *testing.T) {
	d := &Daemon{
		config: &config.Config{},
		lifecycle: lifecycleStack{
			startTime: time.Now(),
		},
		state:    &stateStack{stateManager: &fakeDaemonStateManager{}},
		optional: &optionalStack{},
		serve:    &serveStack{},
	}

	report := &models.BuildReport{Outcome: models.OutcomeFailed}
	if err := d.onBuildReportEmitted(context.Background(), "build-1", report); err != nil {
		t.Fatalf("the failed build reported an error: %v", err)
	}

	if d.lifecycle.lastBuild == nil {
		t.Fatal("the time of the last build was not raised by the failed build")
	}
}
