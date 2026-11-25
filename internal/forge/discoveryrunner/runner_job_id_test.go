package discoveryrunner

import "testing"

// Two discovery runs must not get the same job id: the debouncer drops
// a request whose job id it has already emitted, and the second run
// would never build.
func TestTheDefaultJobIDIsNotTheSameTwice(t *testing.T) {
	r := New(Config{})

	first := r.newJobID()
	second := r.newJobID()

	if first == second {
		t.Fatalf("two runs got the same job id: %s", first)
	}
}
