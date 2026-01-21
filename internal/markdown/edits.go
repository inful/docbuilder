package markdown

import (
	"errors"
	"fmt"
	"sort"
)

// Edit represents a targeted byte-range replacement.
//
// Start and End are byte offsets into the original source, with End exclusive.
// Replacement replaces source[Start:End].
//
// This is used to implement minimal-diff modifications without re-rendering Markdown.
type Edit struct {
	Start       int
	End         int
	Replacement []byte
}

// ApplyEdits applies a set of byte-range edits to source and returns the updated content.
//
// Edits must be non-overlapping and refer to offsets in the original source.
// ApplyEdits sorts edits and applies them from the end of the file toward the beginning
// so earlier edits do not invalidate offsets for later edits.
func ApplyEdits(source []byte, edits []Edit) ([]byte, error) {
	if len(edits) == 0 {
		return source, nil
	}

	sorted := make([]Edit, len(edits))
	copy(sorted, edits)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Start == sorted[j].Start {
			return sorted[i].End > sorted[j].End
		}
		return sorted[i].Start > sorted[j].Start
	})

	for i, e := range sorted {
		if e.Start < 0 || e.End < 0 {
			return nil, fmt.Errorf("invalid edit[%d]: negative range", i)
		}
		if e.End < e.Start {
			return nil, fmt.Errorf("invalid edit[%d]: end before start", i)
		}
		if e.End > len(source) {
			return nil, fmt.Errorf("invalid edit[%d]: range out of bounds", i)
		}
		if i > 0 {
			prev := sorted[i-1]
			// Because edits are sorted by Start descending, the current edit must end
			// at or before the previous edit's start to avoid overlap.
			if e.End > prev.Start {
				return nil, errors.New("invalid edits: overlapping ranges")
			}
		}
	}

	out := append([]byte(nil), source...)
	for _, e := range sorted {
		prefix := out[:e.Start]
		suffix := out[e.End:]
		next := make([]byte, 0, len(prefix)+len(e.Replacement)+len(suffix))
		next = append(next, prefix...)
		next = append(next, e.Replacement...)
		next = append(next, suffix...)
		out = next
	}

	return out, nil
}
