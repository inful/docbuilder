package foundation

import (
	"encoding/json"
	"testing"
	"time"
)

// An Option must survive a trip through JSON. The fields of the Option
// are unexported, so without a MarshalJSON the encoder writes "{}" and
// the value is gone the moment it is written to a file.
func TestOptionSurvivesJSON(t *testing.T) {
	commit := "abc123"

	cases := []struct {
		name string
		in   Option[string]
		out  Option[string]
	}{
		{"a value", Some(commit), Some(commit)},
		{"no value", None[string](), None[string]()},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data, err := json.Marshal(tc.in)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			if tc.in.IsSome() && string(data) == "{}" {
				t.Fatalf("the value was written as an empty object: %s", data)
			}

			var back Option[string]
			if err := json.Unmarshal(data, &back); err != nil {
				t.Fatalf("unmarshal %s: %v", data, err)
			}
			if back.IsSome() != tc.out.IsSome() {
				t.Fatalf("read back %v, want %v", back, tc.out)
			}
			if back.IsSome() && back.Unwrap() != tc.out.Unwrap() {
				t.Fatalf("read back %q, want %q", back.Unwrap(), tc.out.Unwrap())
			}
		})
	}
}

// A time must survive the trip as well: the state file keeps the time
// of the last build of a repository in an Option.
func TestOptionTimeSurvivesJSON(t *testing.T) {
	when := time.Date(2026, 11, 22, 10, 30, 0, 0, time.UTC)

	data, err := json.Marshal(Some(when))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var back Option[time.Time]
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatalf("unmarshal %s: %v", data, err)
	}
	if back.IsNone() {
		t.Fatal("the time was lost")
	}
	if !back.Unwrap().Equal(when) {
		t.Fatalf("read back %v, want %v", back.Unwrap(), when)
	}
}

// The state files written before the Option could be written hold "{}"
// for every Option. They must still load: an empty Option, not an
// error that throws the whole file away.
func TestOptionReadsTheOldEmptyObject(t *testing.T) {
	var back Option[string]
	if err := json.Unmarshal([]byte(`{}`), &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if back.IsSome() {
		t.Fatalf("read back %v, want no value", back)
	}
}
