package daemon

import (
	"fmt"
	"testing"
)

// The dispatcher must not remember every document it ever saw: the map
// would grow for the life of the daemon otherwise.
func TestUploadedMapIsBounded(t *testing.T) {
	d := &OutboundDispatcher{}
	for i := range maxUploadedEntries + 10 {
		d.rememberUploaded(fmt.Sprintf("uid-%d", i), "fingerprint")
	}

	d.uploadedMu.Lock()
	size := len(d.uploaded)
	d.uploadedMu.Unlock()

	if size > maxUploadedEntries {
		t.Fatalf("the map holds %d entries, want at most %d", size, maxUploadedEntries)
	}
	if _, ok := d.uploadedFingerprint(fmt.Sprintf("uid-%d", maxUploadedEntries+9)); !ok {
		t.Fatal("the newest entry must be kept")
	}
	if _, ok := d.uploadedFingerprint("uid-0"); ok {
		t.Fatal("the oldest entry must be dropped")
	}
}
