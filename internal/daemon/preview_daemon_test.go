package daemon

import (
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// The preview daemon has no discovery runner: the trigger would
// dereference a nil pointer inside a goroutine and kill the process.
func TestThePreviewDaemonHasNoDiscoveryToTrigger(t *testing.T) {
	d := NewPreviewDaemon(&config.Config{})

	if jobID := d.TriggerDiscovery(); jobID != "" {
		t.Fatalf("the preview daemon triggered a discovery: %q", jobID)
	}
}
