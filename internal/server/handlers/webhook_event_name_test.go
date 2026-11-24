package handlers

import "testing"

// The event type comes from a request header, and the answer repeats
// it. A header holds whatever the sender wants, so only the shape of a
// forge event name is kept.
func TestTheEventNameKeepsOnlyTheShapeOfAnEventName(t *testing.T) {
	tests := []struct {
		eventType string
		want      string
	}{
		{"push", "push"},
		{"pull_request", "pull_request"},
		{"release.created", "release.created"},
		{"", ""},
		{"<script>alert(1)</script>", ""},
		{"push\nX-Injected: 1", ""},
		{"push with spaces", ""},
	}

	for _, tt := range tests {
		if got := eventName(tt.eventType); got != tt.want {
			t.Errorf("eventName(%q) = %q, want %q", tt.eventType, got, tt.want)
		}
	}
}
