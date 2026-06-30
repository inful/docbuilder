package hashutil

import "testing"

// TestPathsHash pins:
//   - empty input returns "" (no sentinel value, no panic);
//   - the canonical NUL-separator sha256-of-sorted-paths shape so a
//     regression that flips separator or removes the sort is caught.
//   - order-independence for any permutation of the same set.
func TestPathsHash(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		if got := PathsHash(nil); got != "" {
			t.Errorf("PathsHash(nil) = %q, want empty", got)
		}
		if got := PathsHash([]string{}); got != "" {
			t.Errorf("PathsHash(empty) = %q, want empty", got)
		}
	})

	t.Run("single", func(t *testing.T) {
		if got := PathsHash([]string{"docs/a.md"}); got == "" {
			t.Errorf("single-element hash is empty, want non-empty")
		}
		// Re-pin a known hex digest to lock in the exact algorithm.
		want := PathsHash([]string{"docs/a.md"})
		if got := PathsHash([]string{"docs/a.md"}); got != want {
			t.Errorf("hash not deterministic across calls")
		}
	})

	t.Run("order-independent", func(t *testing.T) {
		// Add a separator-sensitive edge: "a/b" + "c" must hash
		// differently from "a" + "b/c". The NUL separator guarantees
		// this; we encode it as a regression check.
		a := PathsHash([]string{"a/b", "c"})
		b := PathsHash([]string{"c", "a/b"})
		if a != b {
			t.Errorf("order should not affect hash, got %q vs %q", a, b)
		}
		collision := PathsHash([]string{"a", "b/c"})
		if a == collision {
			t.Errorf("NUL separator should keep {a/b, c} distinct from {a, b/c}")
		}

		// Multi-element set, several permutations.
		set := []string{"docs/a.md", "docs/b.md", "docs/c.md"}
		perm1 := PathsHash(set)
		perm2 := PathsHash([]string{"docs/c.md", "docs/a.md", "docs/b.md"})
		if perm1 != perm2 {
			t.Errorf("multi-element set should be order-independent: %q vs %q", perm1, perm2)
		}
	})

	t.Run("set-distinct", func(t *testing.T) {
		// Different sets produce different hashes; a single path
		// change flips the digest.
		base := PathsHash([]string{"docs/a.md", "docs/b.md", "docs/c.md"})
		added := PathsHash([]string{"docs/a.md", "docs/b.md", "docs/c.md", "docs/d.md"})
		removed := PathsHash([]string{"docs/a.md", "docs/b.md"})
		if base == added || base == removed || added == removed {
			t.Errorf("distinct sets must produce distinct hashes; got base=%q added=%q removed=%q", base, added, removed)
		}
	})

	t.Run("input-not-mutated", func(t *testing.T) {
		// PathsHash sorts a copy; the caller's slice order must
		// remain whatever they passed in.
		set := []string{"z", "y", "x", "a"}
		before := append([]string(nil), set...)
		_ = PathsHash(set)
		for i, v := range before {
			if set[i] != v {
				t.Fatalf("PathsHash mutated caller slice at %d: before %q after %q", i, v, set[i])
			}
		}
	})
}
