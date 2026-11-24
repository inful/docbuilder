package lint

import "testing"

// The prefix of the directory is part of the name: the sequence number
// brings it back. Without this the name read "adr-001-adr-something.md".
func TestTheSequenceNameDoesNotRepeatThePrefix(t *testing.T) {
	tests := []struct {
		prefix   string
		seq      int
		original string
		want     string
	}{
		{"adr-", 1, "adr-something.md", "adr-001-something.md"},
		{"adr-", 1, "something.md", "adr-001-something.md"},
		{"adr-", 7, "adr-003-something.md", "adr-007-something.md"},
		{"adr-", 2, "adr-12-something.md", "adr-002-something.md"},
	}

	for _, tt := range tests {
		got := buildSequenceName(tt.prefix, tt.seq, tt.original)
		if got != tt.want {
			t.Errorf("buildSequenceName(%q, %d, %q) = %q, want %q", tt.prefix, tt.seq, tt.original, got, tt.want)
		}
	}
}
