package transforms

// The existing hugo.Page struct already exposes exported fields used by transformers; we use
// a lightweight type assertion inside each Transform call to access concrete data without
// introducing a direct dependency (avoids cycle since registry is used by hugo generator).

// Helper to safely cast.
func asPage[T any](p PageAdapter) (T, bool) { v, ok := p.(T); return v, ok }
