package main

// allTools returns the full set of MCP clients this binary knows how to
// configure. Adding a new tool = implement toolSpec + register here.
//
// Order matters for display only: `init` and `print` sort by ID before
// iterating, so users see deterministic output regardless of registration
// order.
func allTools() []toolSpec {
	return []toolSpec{
		opencodeTool{},
		vscodeTool{},
	}
}
