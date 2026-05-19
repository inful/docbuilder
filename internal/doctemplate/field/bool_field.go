package field

import "errors"

// TriState represents three-valued boolean input.
type TriState int

const (
	TriUnset TriState = iota
	TriTrue
	TriFalse
)

var errRequiredBoolUnset = errors.New("required boolean is unset")

// BoolField stores state for a boolean form field.
type BoolField struct {
	Required bool
	State    TriState
}

// NewBoolField constructs a bool field with optional initial value.
func NewBoolField(required bool, initial *bool) BoolField {
	if initial == nil {
		return BoolField{Required: required, State: TriUnset}
	}
	if *initial {
		return BoolField{Required: required, State: TriTrue}
	}
	return BoolField{Required: required, State: TriFalse}
}

// SetTrue marks the state as true.
func (f *BoolField) SetTrue() {
	f.State = TriTrue
}

// SetFalse marks the state as false.
func (f *BoolField) SetFalse() {
	f.State = TriFalse
}

// Clear marks the state as unset.
func (f *BoolField) Clear() {
	f.State = TriUnset
}

// Value returns the current value and whether a value is set.
func (f BoolField) Value() (bool, bool) {
	switch f.State {
	case TriTrue:
		return true, true
	case TriFalse:
		return false, true
	case TriUnset:
		return false, false
	default:
		return false, false
	}
}

// Validate checks required-field constraints.
func (f BoolField) Validate() error {
	if !f.Required {
		return nil
	}
	if f.State == TriUnset {
		return errRequiredBoolUnset
	}
	return nil
}

// RequiredUnsetError reports whether err indicates an unset required bool.
func RequiredUnsetError(err error) bool {
	return errors.Is(err, errRequiredBoolUnset)
}
