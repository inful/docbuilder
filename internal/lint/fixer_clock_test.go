package lint

import (
	"testing"
	"time"
)

// The clock of the fixer is UTC. The dates it writes are documented as
// UTC, and the fingerprint of a document has to be the same on every
// machine.
func TestTheClockOfTheFixerIsUTC(t *testing.T) {
	f := &Fixer{}
	if loc := f.now().Location(); loc != time.UTC {
		t.Fatalf("the clock is not UTC: %s", loc)
	}

	// The clock of a test is a fixed instant in a zone of its own: it is
	// converted as well.
	zone := time.FixedZone("plus thirteen", 13*60*60)
	injected := &Fixer{nowFn: func() time.Time {
		return time.Date(2026, time.November, 24, 12, 0, 0, 0, zone)
	}}
	got := injected.now()
	if loc := got.Location(); loc != time.UTC {
		t.Fatalf("the injected clock is not UTC: %s", loc)
	}
	if got.Hour() != 23 || got.Day() != 23 {
		t.Fatalf("the injected clock was not converted: %s", got)
	}
}
