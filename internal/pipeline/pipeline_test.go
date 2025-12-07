package pipeline

import (
	"testing"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// TestGenerateFlow scaffolds the bus and runs a generate handler end-to-end
// using the Hextra theme to prove the pattern.
func TestGenerateFlow(t *testing.T) {
	cfg := &config.Config{}
	cfg.Hugo.Theme = "hextra"

	plan := NewBuildPlanBuilder(cfg).
		WithOutput(t.TempDir(), t.TempDir()).
		WithIncremental(false).
		Build()
	bus := NewBus()

	// Subscribe generate handler
	bus.Subscribe(EventGenerateRequested, NewGenerateHandler())

	// Publish generate requested event with no files
	if err := bus.Publish(GenerateRequested{Plan: plan, DocFiles: nil}); err != nil {
		t.Fatalf("generate flow failed: %v", err)
	}
}

// TestCloneDiscoverGenerateFlow validates the full event chain: clone → discover → generate.
func TestCloneDiscoverGenerateFlow(t *testing.T) {
	t.Skip("Skipping integration test requiring network access; run manually with real repo URL")
	cfg := &config.Config{}
	cfg.Hugo.Theme = "hextra"
	// Add a real public repo for testing (users can replace with their own)
	cfg.Repositories = []config.Repository{
		{Name: "docbuilder", URL: "https://github.com/example/docbuilder.git", Branch: "main", Paths: []string{"docs"}},
	}

	plan := NewBuildPlanBuilder(cfg).
		WithOutput(t.TempDir(), t.TempDir()).
		WithIncremental(false).
		Build()
	bus := NewBus()

	// Subscribe handlers in order
	bus.Subscribe(EventCloneRequested, NewCloneHandler(bus))
	bus.Subscribe(EventDiscoverRequested, NewDiscoverHandler(bus))
	bus.Subscribe(EventGenerateRequested, NewGenerateHandler())

	// Optional: track completion
	var discoveredCount int
	bus.Subscribe(EventDiscoverCompleted, func(e Event) error {
		dc, ok := e.(DiscoverCompleted)
		if ok {
			discoveredCount = len(dc.DocFiles)
		}
		return nil
	})

	// Start the chain by publishing CloneRequested
	if err := bus.Publish(CloneRequested{Plan: plan}); err != nil {
		t.Fatalf("clone-discover-generate flow failed: %v", err)
	}

	// Validate chain executed (discoveredCount will be 0 since we don't have real repo paths)
	t.Logf("Event chain completed; discovered files: %d", discoveredCount)
}

// TestCloneWithRetry validates that clone handler can be wrapped with retry logic.
func TestCloneWithRetry(t *testing.T) {
	cfg := &config.Config{}
	cfg.Repositories = []config.Repository{
		{Name: "fake-repo", URL: "https://invalid-domain-12345.com/repo.git", Branch: "main"},
	}

	plan := NewBuildPlanBuilder(cfg).
		WithOutput(t.TempDir(), t.TempDir()).
		WithIncremental(false).
		Build()
	bus := NewBus()
	dlq := NewDeadLetterQueue()

	// Wrap clone handler with retry policy
	policy := RetryPolicy{
		MaxAttempts: 2,
		Backoff:     time.Millisecond,
		IsRetryable: func(err error) bool {
			// In real usage, check for network/timeout errors
			return true
		},
	}
	bus.Subscribe(EventCloneRequested, WithRetry(NewCloneHandler(bus), policy, dlq))

	// Attempt to clone invalid repo (should fail and go to DLQ)
	err := bus.Publish(CloneRequested{Plan: plan})
	if err == nil {
		t.Fatal("expected clone to fail for invalid repo")
	}

	// Verify DLQ captured the failure
	if dlq.Count() != 1 {
		t.Errorf("expected 1 failed event in DLQ, got %d", dlq.Count())
	}
}
