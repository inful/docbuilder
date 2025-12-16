package hugo

import "context"

// stageIndexes is now a no-op since the new pipeline (ADR-003) generates all
// indexes during content processing. Kept as empty function to maintain build
// stage compatibility.
func stageIndexes(_ context.Context, bs *BuildState) error {
// New pipeline already generates all indexes - nothing to do here
return nil
}
