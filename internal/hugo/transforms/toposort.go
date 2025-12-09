package transforms

import (
	"fmt"
	"sort"
)

// topologicalSort performs dependency resolution within a stage using Kahn's algorithm.
// Returns transforms in execution order or an error if dependencies cannot be satisfied.
func topologicalSort(transforms []Transformer) ([]Transformer, error) {
	if len(transforms) == 0 {
		return []Transformer{}, nil
	}

	// Build name -> transform map
	byName := make(map[string]Transformer)
	for _, t := range transforms {
		name := t.Name()
		if _, exists := byName[name]; exists {
			return nil, fmt.Errorf("duplicate transformer name: %q", name)
		}
		byName[name] = t
	}

	// Build adjacency list (dependency graph) and calculate in-degrees
	graph := make(map[string][]string)
	inDegree := make(map[string]int)

	// Initialize all nodes
	for _, t := range transforms {
		name := t.Name()
		graph[name] = []string{}
		if _, exists := inDegree[name]; !exists {
			inDegree[name] = 0
		}
	}

	// Build edges from dependencies
	for _, t := range transforms {
		name := t.Name()
		deps := t.Dependencies()

		// Process MustRunAfter: dep -> current
		// (current must run after dep, so dep points to current)
		// Skip if dependency is not in this stage (cross-stage deps are OK, enforced by stage order)
		for _, dep := range deps.MustRunAfter {
			if _, exists := byName[dep]; exists {
				graph[dep] = append(graph[dep], name)
				inDegree[name]++
			}
			// If dep doesn't exist in this stage, it's either in an earlier stage (OK)
			// or missing entirely (will be caught by ValidateDependencies)
		}

		// Process MustRunBefore: current -> after
		// (current must run before after, so current points to after)
		// Skip if target is not in this stage (cross-stage deps are OK)
		for _, after := range deps.MustRunBefore {
			if _, exists := byName[after]; exists {
				graph[name] = append(graph[name], after)
				inDegree[after]++
			}
			// If after doesn't exist in this stage, it's either in a later stage (OK)
			// or missing entirely (will be caught by ValidateDependencies)
		}
	}

	// Kahn's algorithm: start with nodes that have no dependencies
	var queue []string
	for _, t := range transforms {
		name := t.Name()
		if inDegree[name] == 0 {
			queue = append(queue, name)
		}
	}

	// Sort queue for deterministic ordering when multiple nodes have same priority
	sort.Strings(queue)

	var result []Transformer
	visited := make(map[string]bool)

	for len(queue) > 0 {
		// Pop from queue
		current := queue[0]
		queue = queue[1:]

		if visited[current] {
			continue
		}
		visited[current] = true

		result = append(result, byName[current])

		// Process neighbors (transforms that depend on current)
		neighbors := graph[current]
		sort.Strings(neighbors) // Deterministic order

		for _, neighbor := range neighbors {
			inDegree[neighbor]--
			if inDegree[neighbor] == 0 {
				queue = append(queue, neighbor)
				sort.Strings(queue) // Keep queue sorted for deterministic ordering
			}
		}
	}

	// Check for cycles: if we didn't visit all nodes, there's a cycle
	if len(result) != len(transforms) {
		// Find the unvisited nodes to provide a helpful error message
		unvisited := []string{}
		for _, t := range transforms {
			if !visited[t.Name()] {
				unvisited = append(unvisited, t.Name())
			}
		}
		sort.Strings(unvisited)
		return nil, fmt.Errorf("circular dependency detected involving transforms: %v", unvisited)
	}

	return result, nil
}

// BuildPipeline constructs the execution order using stages and dependencies.
// Transforms are first grouped by stage, then sorted within each stage by dependencies.
func BuildPipeline(transforms []Transformer) ([]Transformer, error) {
	if len(transforms) == 0 {
		return []Transformer{}, nil
	}

	// Validate stages
	for _, t := range transforms {
		if !IsValidStage(t.Stage()) {
			return nil, fmt.Errorf("transform %q has invalid stage: %q", t.Name(), t.Stage())
		}
	}

	// Group by stage
	byStage := make(map[TransformStage][]Transformer)
	for _, t := range transforms {
		stage := t.Stage()
		byStage[stage] = append(byStage[stage], t)
	}

	// Sort each stage by dependencies and combine
	var result []Transformer
	for _, stage := range StageOrder {
		stageTransforms, exists := byStage[stage]
		if !exists {
			continue
		}

		sorted, err := topologicalSort(stageTransforms)
		if err != nil {
			return nil, fmt.Errorf("stage %s: %w", stage, err)
		}

		result = append(result, sorted...)
	}

	return result, nil
}

// ValidateDependencies checks for common dependency issues without building the full pipeline.
// Returns nil if dependencies are valid, error otherwise.
func ValidateDependencies(transforms []Transformer) error {
	if len(transforms) == 0 {
		return nil
	}

	// Build name set for quick lookup
	names := make(map[string]bool)
	for _, t := range transforms {
		names[t.Name()] = true
	}

	// Check each transform's dependencies
	for _, t := range transforms {
		deps := t.Dependencies()

		// Validate MustRunAfter
		for _, dep := range deps.MustRunAfter {
			if !names[dep] {
				return fmt.Errorf("transform %q depends on missing transform %q", t.Name(), dep)
			}
		}

		// Validate MustRunBefore
		for _, after := range deps.MustRunBefore {
			if !names[after] {
				return fmt.Errorf("transform %q requires missing transform %q", t.Name(), after)
			}
		}
	}

	// Try to build pipeline to detect cycles
	_, err := BuildPipeline(transforms)
	return err
}
