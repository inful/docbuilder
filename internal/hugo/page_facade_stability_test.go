package hugo

import (
	"reflect"
	"sort"
	"testing"
)

// TestPageFacadeStability guards against accidental interface expansion which could break third-party transformers.
func TestPageFacadeStability(t *testing.T) {
	typ := reflect.TypeOf((*PageFacade)(nil)).Elem()
	var methods []string
	for i := 0; i < typ.NumMethod(); i++ {
		methods = append(methods, typ.Method(i).Name)
	}
	sort.Strings(methods)
	expected := []string{
		"AddPatch",
		"ApplyPatches",
		"GetContent",
		"GetOriginalFrontMatter",
		"HadOriginalFrontMatter",
		"SetContent",
		"SetOriginalFrontMatter",
	}
	if len(methods) != len(expected) {
		// Provide diff-style output
		// NOTE: Keep message concise for CI logs.
		// Extra or missing methods indicate a breaking change; update this test intentionally when expanding the facade.
		// Current: expected 7 methods.
		// Received: methods slice below.
		// Action: Either revert unintended addition/removal or update expected list + docs.
		//
		// This protects external extension authors relying on stable minimal contract.
		//
		// Print both slices for clarity.
		//
		// The order is sorted to produce deterministic output.
		//
		// End note.
		//
		// Fail now:
		//
		t.Fatalf("PageFacade method count drift: got %d want %d (%v)", len(methods), len(expected), methods)
	}
	for i, name := range expected {
		if methods[i] != name {
			// Detailed mismatch message
			t.Fatalf("PageFacade method mismatch at %d: got %s want %s (full=%v)", i, methods[i], name, methods)
		}
	}
}
