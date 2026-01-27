package httpserver

import (
	"testing"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

type stubRuntime struct{}

func (stubRuntime) GetStatus() string                                   { return "running" }
func (stubRuntime) GetActiveJobs() int                                  { return 0 }
func (stubRuntime) GetStartTime() time.Time                             { return time.Time{} }
func (stubRuntime) HTTPRequestsTotal() int                              { return 0 }
func (stubRuntime) RepositoriesTotal() int                              { return 0 }
func (stubRuntime) LastDiscoveryDurationSec() int                       { return 0 }
func (stubRuntime) LastBuildDurationSec() int                           { return 0 }
func (stubRuntime) TriggerDiscovery() string                            { return "" }
func (stubRuntime) TriggerBuild() string                                { return "" }
func (stubRuntime) TriggerWebhookBuild(string, string, []string) string { return "" }
func (stubRuntime) GetQueueLength() int                                 { return 0 }

func TestNewServer_TDDCompile(t *testing.T) {
	_ = New(&config.Config{}, stubRuntime{}, Options{})
}
