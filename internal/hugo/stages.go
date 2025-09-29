package hugo

import "context"

// Stage is a discrete unit of work in the site build (retained here to avoid churn in existing references).
type Stage func(ctx context.Context, bs *BuildState) error
