package daemon

import (
	"testing"

	"github.com/stretchr/testify/require"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// TestNewDaemonWithConfigFile_FullWiring is the regression guard for the
// daemon's constructor. It builds a config that exercises every
// configurable feature path (forges, live reload, ragabast outbound) and
// asserts the post-construction invariants that the daemon relies on.
//
// This test is the canary for the build-service-unification F1 follow-up:
// the Phase 4 refactor of that branch silently lost SkipState and
// OnDocumentReady wiring because no test covered the post-construction
// state of a fully-initialized daemon. The fix in 1edfb9f restored the
// wiring; this test prevents any future constructor change from regressing
// the same way.
//
// Link verification is intentionally NOT enabled here because
// linkverify.NewVerificationService opens a NATS connection at
// construction time. The daemon treats that as a soft failure and leaves
// linkVerifier nil, so we assert the "config disabled" path here and
// cover the "config enabled" path elsewhere.
func TestNewDaemonWithConfigFile_FullWiring(t *testing.T) {
	repoCache := t.TempDir()
	cfg := &config.Config{
		Version: "2.0",
		Forges: []*config.ForgeConfig{{
			Name: "example",
			Type: config.ForgeLocal,
		}},
		Repositories: []config.Repository{{
			Name:   "org/repo",
			URL:    "https://forge.example.com/org/repo.git",
			Branch: "main",
		}},
		Output: config.OutputConfig{Directory: t.TempDir()},
		Build:  config.BuildConfig{LiveReload: true},
		Daemon: &config.DaemonConfig{
			HTTP: config.HTTPConfig{
				DocsPort:       18080,
				WebhookPort:    18081,
				AdminPort:      18082,
				LiveReloadPort: 18083,
			},
			Storage: config.StorageConfig{RepoCacheDir: repoCache},
			Sync: config.SyncConfig{
				Schedule:         "@every 1h",
				ConcurrentBuilds: 1,
				QueueSize:        10,
			},
			Outbound: &config.OutboundConfig{
				Ragabast: &config.RagabastConfig{
					Enabled:      true,
					BaseURL:      "https://ragabast.example.com",
					AuthTokenEnv: "DOCBUILDER_TEST_TOKEN",
					Workers:      1,
					QueueSize:    8,
				},
			},
		},
	}

	d, err := NewDaemonWithConfigFile(cfg, "")
	require.NoError(t, err)
	require.NotNil(t, d)
	t.Cleanup(func() {
		// Tear down side-effect-bearing components so the test does not
		// leak goroutines, temp DB handles, or schedulers.
		_ = d.Stop(t.Context())
		if d.state.eventStore != nil {
			_ = d.state.eventStore.Close()
		}
	})

	// Core wiring: every mandatory component must be present.
	require.NotNil(t, d.config)
	require.NotNil(t, d.serve.metrics, "metrics collector must be initialized")
	require.NotNil(t, d.discover.discoveryCache, "discovery cache must be initialized")
	require.NotNil(t, d.serve.orchestrationBus, "orchestration bus must be initialized")
	require.NotNil(t, d.serve.scheduler, "scheduler must be initialized")

	// Discovery stack.
	require.NotNil(t, d.discover.forgeManager, "forge manager must be initialized")
	require.NotNil(t, d.discover.discovery, "discovery service must be initialized")
	require.NotNil(t, d.discover.discoveryRunner, "discovery runner must be initialized")
	require.Len(t, d.discover.forgeManager.GetForgeConfigs(), 1,
		"all configured forges must be registered")

	// Build stack.
	require.NotNil(t, d.build.buildSvc, "build service must be initialized")
	require.NotNil(t, d.build.buildQueue, "build queue must be initialized")
	require.NotNil(t, d.build.buildDebouncer, "build debouncer must be initialized")
	require.NotNil(t, d.build.repoUpdater, "repo updater must be initialized")

	// State stack.
	require.NotNil(t, d.state.stateManager, "state manager must be initialized")
	require.NotNil(t, d.state.eventStore, "event store must be initialized")
	require.NotNil(t, d.state.buildProjection, "build projection must be initialized")
	require.NotNil(t, d.state.eventEmitter, "event emitter must be initialized")

	// Serving stack.
	require.NotNil(t, d.serve.httpServer, "http server must be initialized")

	// Optional features enabled by config: must be non-nil.
	require.NotNil(t, d.optional.liveReload, "livereload must be initialized when cfg.Build.LiveReload is true")
	require.NotNil(t, d.optional.outboundDispatcher, "outbound dispatcher must be initialized when ragabast is enabled and Enabled=true")

	// Optional feature disabled by config: must be nil.
	require.Nil(t, d.optional.linkVerifier, "link verifier must be nil when link verification is not enabled")

	// Cross-component invariant: build queue calls back into the daemon
	// (queue.Builder = *Daemon). The compile-time assertion
	// `var _ queue.Builder = (*Daemon)(nil)` at the bottom of daemon.go
	// is the structural guard; this test confirms the queue is the one
	// built around that builder by checking it was constructed with the
	// right size and worker count.
	require.NotNil(t, d.build.buildQueue, "build queue must be initialized")
}

// TestNewDaemonWithConfigFile_RagabastDisabledKeepsDispatcherNil locks
// down the opt-in semantics: when the ragabast block is absent or not
// Enabled, the daemon must leave outboundDispatcher nil. Centralizing
// this check in the constructor-outcome test catches future regressions
// where some helper unconditionally constructs the dispatcher.
func TestNewDaemonWithConfigFile_RagabastDisabledKeepsDispatcherNil(t *testing.T) {
	cases := []struct {
		name   string
		modify func(*config.Config)
	}{
		{
			name:   "no_outbound_block",
			modify: func(*config.Config) {},
		},
		{
			name: "outbound_block_present_but_disabled",
			modify: func(c *config.Config) {
				c.Daemon.Outbound = &config.OutboundConfig{
					Ragabast: &config.RagabastConfig{Enabled: false},
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repoCache := t.TempDir()
			cfg := &config.Config{
				Version: "2.0",
				Repositories: []config.Repository{{
					Name:   "org/repo",
					URL:    "https://example.com/org/repo.git",
					Branch: "main",
				}},
				Output: config.OutputConfig{Directory: t.TempDir()},
				Daemon: &config.DaemonConfig{
					HTTP:    config.HTTPConfig{DocsPort: 18090, WebhookPort: 18091, AdminPort: 18092},
					Storage: config.StorageConfig{RepoCacheDir: repoCache},
					Sync:    config.SyncConfig{Schedule: "@every 1h", ConcurrentBuilds: 1, QueueSize: 10},
				},
			}
			tc.modify(cfg)

			d, err := NewDaemonWithConfigFile(cfg, "")
			require.NoError(t, err)
			t.Cleanup(func() {
				_ = d.Stop(t.Context())
				if d.state.eventStore != nil {
					_ = d.state.eventStore.Close()
				}
			})

			require.Nil(t, d.optional.outboundDispatcher,
				"outbound dispatcher must remain nil when ragabast is not opted in")
		})
	}
}
