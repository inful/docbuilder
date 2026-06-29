package httpserver

import (
	"testing"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

type stubRuntime struct{}

func (stubRuntime) GetStatus() string       { return "running" }
func (stubRuntime) GetActiveJobs() int      { return 0 }
func (stubRuntime) GetStartTime() time.Time { return time.Time{} }

func TestNewServer_TDDCompile(t *testing.T) {
	_ = New(&config.Config{}, stubRuntime{}, Options{})
}
