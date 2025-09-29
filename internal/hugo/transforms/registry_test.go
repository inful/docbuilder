package transforms

import (
  "testing"
)

// TestOrdering ensures priority ordering then name tiebreaker.
func TestOrdering(t *testing.T) {
  ts := List()
  if len(ts) == 0 { t.Skip("no transformers registered") }
  lastPri := -999
  lastName := ""
  for i, tr := range ts {
    pri := tr.Priority()
    name := tr.Name()
    if i > 0 {
      if pri < lastPri { t.Fatalf("out of order: %s (%d) preceded by higher priority %d", name, pri, lastPri) }
      if pri == lastPri && name < lastName { t.Fatalf("name tiebreaker order incorrect: %s before %s", name, lastName) }
    }
    lastPri = pri
    lastName = name
  }
}
