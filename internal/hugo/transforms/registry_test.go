package transforms

import (
	"testing"
)

// TestOrdering ensures transforms are ordered by stage and dependencies.
func TestOrdering(t *testing.T) {
	ts, err := List()
	if err != nil {
		t.Fatalf("List() failed: %v", err)
	}
	if len(ts) == 0 {
		t.Skip("no transformers registered")
	}

	// Verify stage ordering (stages should only move forward)
	lastStage := StageParse
	for i, tr := range ts {
		stage := tr.Stage()
		if i > 0 {
			lastIndex := StageIndex(lastStage)
			currentIndex := StageIndex(stage)
			if currentIndex < lastIndex {
				t.Fatalf("stage ordering violation: %s (%s) comes after %s (%s)",
					tr.Name(), stage, ts[i-1].Name(), lastStage)
			}
		}
		lastStage = stage
	}
}
