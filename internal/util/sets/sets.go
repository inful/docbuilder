package sets

// Set is a simple generic hash set for comparable keys.
// Intentionally minimal: no reflection, no iteration helpers beyond range.
// Usage: s := sets.New[string]("a","b"); s.Add("c"); if s.Has("b") {...}
// Kept internal to avoid committing to external API stability pre-1.0.
type Set[T comparable] map[T]struct{}

// New creates a set pre-populated with the provided values.
func New[T comparable](vals ...T) Set[T] {
	s := make(Set[T], len(vals))
	for _, v := range vals {
		s[v] = struct{}{}
	}
	return s
}

// Add inserts value into the set.
func (s Set[T]) Add(v T) { s[v] = struct{}{} }

// Has returns true if v is present.
func (s Set[T]) Has(v T) bool {
	_, ok := s[v]
	return ok
}

// Delete removes v if present.
func (s Set[T]) Delete(v T) { delete(s, v) }

// Clone returns a shallow copy.
func (s Set[T]) Clone() Set[T] {
	out := make(Set[T], len(s))
	for k := range s {
		out[k] = struct{}{}
	}
	return out
}
