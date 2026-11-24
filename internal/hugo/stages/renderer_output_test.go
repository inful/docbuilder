package stages

import (
	"context"
	"strings"
	"testing"
)

// Hugo writes tens of megabytes for a big site. The output goes to the
// log as it arrives, and only the last lines are kept for the message
// of a failure: holding the whole output in memory is a lot for
// nothing.
func TestTheOutputOfHugoIsNotHeldInMemory(t *testing.T) {
	w := &hugoOutputWriter{ctx: context.Background(), attr: "hugo stdout"}

	line := strings.Repeat("a", 1000) + "\n"
	for range 1000 {
		if _, err := w.Write([]byte(line)); err != nil {
			t.Fatalf("write: %v", err)
		}
	}

	tail := w.tail()
	lines := strings.Count(tail, "\n") + 1
	if lines > maxHugoOutputLines {
		t.Fatalf("the writer kept %d lines, want at most %d", lines, maxHugoOutputLines)
	}
	if !strings.Contains(tail, "aaa") {
		t.Fatal("the writer kept no line at all")
	}
}

// A line that is not finished when the process ends is kept.
func TestTheLastLineOfTheOutputIsKept(t *testing.T) {
	w := &hugoOutputWriter{ctx: context.Background(), attr: "hugo stdout"}

	if _, err := w.Write([]byte("first\nsecond without a newline")); err != nil {
		t.Fatalf("write: %v", err)
	}

	tail := w.tail()
	if !strings.Contains(tail, "first") || !strings.Contains(tail, "second without a newline") {
		t.Fatalf("the writer kept %q", tail)
	}
}
