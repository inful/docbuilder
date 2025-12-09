package frontmatter

import (
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/plugin/transforms"
)

// TestFrontmatterTransformImplementsV2Interface verifies V2 interface implementation.
func TestFrontmatterTransformImplementsV2Interface(t *testing.T) {
	var _ transforms.TransformPlugin = (*FrontmatterTransform)(nil)
}

// TestFrontmatterTransformDependencies tests the Dependencies method.
func TestFrontmatterTransformDependencies(t *testing.T) {
	transform := NewFrontmatterTransform()
	deps := transform.Dependencies()

	if len(deps.MustRunAfter) != 0 {
		t.Errorf("MustRunAfter should be empty, got %d items", len(deps.MustRunAfter))
	}

	if len(deps.MustRunBefore) != 0 {
		t.Errorf("MustRunBefore should be empty, got %d items", len(deps.MustRunBefore))
	}
}

// TestFrontmatterTransformV2Registration tests V2 registration.
func TestFrontmatterTransformV2Registration(t *testing.T) {
	registry := transforms.NewTransformRegistry()
	transform := NewFrontmatterTransform()

	// Test V2 registration
	if err := registry.Register(transform); err != nil {
		t.Errorf("Register() failed: %v", err)
	}

	// Also test V1 registration for backward compatibility
	if err := registry.Register(transform); err != nil {
		t.Errorf("Register() failed: %v", err)
	}
}
