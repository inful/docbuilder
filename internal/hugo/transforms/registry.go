package transforms

// Package transforms provides a pluggable registry for content transformers.

import (
	"fmt"
)

// PageAdapter is a minimal abstraction to decouple registry from concrete hugo.Page.
// We accept an interface satisfied by *hugo.Page; defined here to avoid import cycle.
type PageAdapter interface{}

// Transformer is defined in dependencies.go (dependency-based interface)

// Registry for dependency-based transformers
var registry = map[string]Transformer{}

// Register adds a dependency-based transformer to the registry.
func Register(t Transformer) {
	if t != nil {
		if _, ok := registry[t.Name()]; !ok {
			registry[t.Name()] = t
		}
	}
}

// List returns transformers sorted by stages and dependencies.
func List() ([]Transformer, error) {
	items := make([]Transformer, 0, len(registry))
	for _, t := range registry {
		items = append(items, t)
	}
	return BuildPipeline(items)
}

// BuildPipelineWithFilter constructs a dependency-based pipeline with optional filtering.
func BuildPipelineWithFilter(include map[string]struct{}) ([]Transformer, error) {
	all, err := List()
	if err != nil {
		return nil, err
	}

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
	cp := make(map[string]Transformer, len(registry))
	for k, v := range registry {
		cp[k] = v
	}
	return cp
}

// restoreRegistry replaces the registry map (test only).
func restoreRegistry(cp map[string]Transformer) { registry = cp }

// SnapshotForTest exposes a registry snapshot (test only).
func SnapshotForTest() map[string]Transformer { return snapshotRegistry() }

// RestoreForTest restores a snapshot (test only).
func RestoreForTest(cp map[string]Transformer) { restoreRegistry(cp) }
