package daemon

import (
	"testing"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/build/queue"
	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/daemon/events"
	"git.home.luguber.info/inful/docbuilder/internal/forge"
	"git.home.luguber.info/inful/docbuilder/internal/forge/discoveryrunner"
	"git.home.luguber.info/inful/docbuilder/internal/server/httpserver"
	"git.home.luguber.info/inful/docbuilder/internal/state"
)

// A daemon that is started again after a stop needs a new stop
// channel: the old one is closed, and the main loop of the new run
// would exit at once.
func TestTheDaemonThatIsStartedAgainHasANewStopChannel(t *testing.T) {
	cfg := &config.Config{
		Monitoring: &config.MonitoringConfig{
			Health:  config.MonitoringHealth{Path: "/health"},
			Metrics: config.MonitoringMetrics{Enabled: true, Path: "/metrics"},
		},
		Daemon: &config.DaemonConfig{
			HTTP: config.HTTPConfig{DocsPort: 0, WebhookPort: 0, AdminPort: 0},
			Sync: config.SyncConfig{Schedule: "0 0 * * *"},
		},
	}
	bus := events.NewBus()
	defer bus.Close()

	runner := NewDiscoveryRunner(DiscoveryRunnerConfig{
		Discovery:      &fakeDiscovery{result: &forge.DiscoveryResult{}},
		DiscoveryCache: discoveryrunner.NewCache(),
		BuildQueue:     &fakeBuildQueue{},
		Config:         cfg,
		Now:            func() time.Time { return time.Unix(123, 0).UTC() },
		NewJobID:       func() string { return "job-1" },
	})

	d := daemonForTest(t, daemonForTestOpts{
		Config:           cfg,
		OrchestrationBus: bus,
		StateManager:     state.NewServiceAdapter(state.NewService(t.TempDir()).Unwrap()),
		DiscoveryRunner:  runner,
	})
	d.serve.metrics = NewMetricsCollector()
	d.serve.httpServer = httpserver.New(cfg, d, httpserver.Options{})
	scheduler, err := NewScheduler()
	if err != nil {
		t.Fatalf("the scheduler: %v", err)
	}
	d.serve.scheduler = scheduler
	d.build.buildQueue = queue.NewBuildQueue(4, 1, &stubBuilder{})

	// The daemon ran and stopped: its stop channel is closed.
	close(d.lifecycle.stopChan)
	d.lifecycle.status.Store(StatusStopped)

	// The start blocks in the main loop: it runs until the daemon
	// stops.
	started := make(chan error, 1)
	go func() { started <- d.Start(t.Context()) }()

	// The daemon is up: the stop channel of the new run is open.
	deadline := time.Now().Add(10 * time.Second)
	for d.GetStatus() != StatusRunning {
		if time.Now().After(deadline) {
			t.Fatal("the daemon did not start")
		}
		time.Sleep(10 * time.Millisecond)
	}

	select {
	case <-d.lifecycle.stopChan:
		t.Fatal("the stop channel of the new run is closed: the main loop would exit at once")
	default:
	}

	_ = d.Stop(t.Context())
	<-started
}
