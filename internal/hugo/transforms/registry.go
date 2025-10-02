package transforms

// Package transforms provides a pluggable registry for content transformers.
// It is an extraction of the previously inlined pipeline in transform.go (Phase 2 roadmap).

import (
	"fmt"
	"sort"
)

// PageAdapter is a minimal abstraction to decouple registry from concrete hugo.Page.
// We accept an interface satisfied by *hugo.Page; defined here to avoid import cycle.
type PageAdapter interface{}

// Transformer defines a content transformation stage.
type Transformer interface {
	Name() string
	Transform(p PageAdapter) error
	Priority() int // lower runs first; legacy pipeline used explicit ordering
}

// simple priority list registry
var reg = map[string]Transformer{}

// Register adds a transformer (idempotent by name). Intended to be called from init() of transformer files.
func Register(t Transformer) {
	if t != nil {
		if _, ok := reg[t.Name()]; !ok {
			reg[t.Name()] = t
		}
	}
}

// List returns transformers sorted by Priority (stable by name for equal priority).
func List() []Transformer {
	items := make([]Transformer, 0, len(reg))
	for _, t := range reg {
		items = append(items, t)
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Priority() == items[j].Priority() {
			return items[i].Name() < items[j].Name()
		}
		return items[i].Priority() < items[j].Priority()
	})
	return items
}

// BuildPipeline constructs a concrete execution slice filtering by optional allowlist.
func BuildPipeline(include map[string]struct{}) ([]Transformer, error) {
	all := List()
	if len(include) == 0 {
		return all, nil
	}
	var out []Transformer
	for _, t := range all {
		if _, ok := include[t.Name()]; ok {
			out = append(out, t)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no transformers matched provided filter")
	}
	return out, nil
}

// --- Test helpers (non-exported) ---
// snapshotRegistry returns a shallow copy for test isolation.
func snapshotRegistry() map[string]Transformer {
	cp := make(map[string]Transformer, len(reg))
	for k, v := range reg {
		cp[k] = v
	}
	return cp
}

// restoreRegistry replaces the registry map (test only).
func restoreRegistry(cp map[string]Transformer) { reg = cp }

// SnapshotForTest exposes a registry snapshot (test only).
func SnapshotForTest() map[string]Transformer { return snapshotRegistry() }

// RestoreForTest restores a snapshot (test only).
func RestoreForTest(cp map[string]Transformer) { restoreRegistry(cp) }
