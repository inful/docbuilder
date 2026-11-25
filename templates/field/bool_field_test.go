package field

import "testing"

func TestNewBoolField(t *testing.T) {
	vTrue := true
	vFalse := false

	unset := NewBoolField(true, nil)
	if unset.State != TriUnset {
		t.Fatalf("expected unset, got %v", unset.State)
	}

	gotTrue := NewBoolField(false, &vTrue)
	if gotTrue.State != TriTrue {
		t.Fatalf("expected true, got %v", gotTrue.State)
	}

	gotFalse := NewBoolField(false, &vFalse)
	if gotFalse.State != TriFalse {
		t.Fatalf("expected false, got %v", gotFalse.State)
	}
}

func TestBoolFieldTransitions(t *testing.T) {
	field := NewBoolField(false, nil)

	field.SetTrue()
	if value, ok := field.Value(); !ok || !value {
		t.Fatalf("expected true value")
	}

	field.SetFalse()
	if value, ok := field.Value(); !ok || value {
		t.Fatalf("expected false value")
	}

	field.Clear()
	if _, ok := field.Value(); ok {
		t.Fatalf("expected unset value")
	}
}

func TestBoolFieldValidateRequired(t *testing.T) {
	required := NewBoolField(true, nil)
	if err := required.Validate(); err == nil {
		t.Fatalf("expected validation error for unset required bool")
	}

	required.SetTrue()
	if err := required.Validate(); err != nil {
		t.Fatalf("expected no validation error, got %v", err)
	}

	optional := NewBoolField(false, nil)
	if err := optional.Validate(); err != nil {
		t.Fatalf("expected no validation error for optional bool, got %v", err)
	}
}
