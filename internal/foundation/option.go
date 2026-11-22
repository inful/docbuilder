package foundation

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Option represents a value that may or may not be present.
// This replaces nullable pointers and provides explicit handling of missing values.
type Option[T any] struct {
	value   T
	present bool
}

// Some creates an Option with a value.
func Some[T any](value T) Option[T] {
	return Option[T]{
		value:   value,
		present: true,
	}
}

// None creates an empty Option.
func None[T any]() Option[T] {
	return Option[T]{
		present: false,
	}
}

// IsSome returns true if the Option contains a value.
func (o Option[T]) IsSome() bool {
	return o.present
}

// IsNone returns true if the Option is empty.
func (o Option[T]) IsNone() bool {
	return !o.present
}

// Unwrap returns the value if present, panics if None.
// Use this only when you're certain the Option contains a value.
func (o Option[T]) Unwrap() T {
	if !o.present {
		panic("called Unwrap on None option")
	}
	return o.value
}

// UnwrapOr returns the value if present, otherwise returns the fallback.
func (o Option[T]) UnwrapOr(fallback T) T {
	if o.present {
		return o.value
	}
	return fallback
}

// Filter returns the Option if the predicate is true, otherwise None.
func (o Option[T]) Filter(predicate func(T) bool) Option[T] {
	if o.present && predicate(o.value) {
		return o
	}
	return None[T]()
}

// ToPointer returns a pointer to the value if present, nil if None.
func (o Option[T]) ToPointer() *T {
	if o.present {
		return &o.value
	}
	return nil
}

// FromPointer creates an Option from a pointer.
// Returns Some(value) if pointer is non-nil, None if nil.
func FromPointer[T any](ptr *T) Option[T] {
	if ptr != nil {
		return Some(*ptr)
	}
	return None[T]()
}

// String provides a string representation of the Option.
func (o Option[T]) String() string {
	if o.present {
		return fmt.Sprintf("Some(%v)", o.value)
	}
	return "None"
}

// MarshalJSON writes the value of the Option, or null when it is empty.
//
// Without this the JSON encoder would write "{}": the fields of the
// Option are unexported, so the encoder skips them. Every value the
// daemon keeps in its state file - the commit of a repository, the time
// of its last build - would be lost the moment the file was written.
func (o Option[T]) MarshalJSON() ([]byte, error) {
	if !o.present {
		return []byte("null"), nil
	}
	return json.Marshal(o.value)
}

// UnmarshalJSON reads a value into the Option.
//
// An empty value leaves the Option empty: null, and the "{}" the state
// files written before this method existed hold for every Option.
func (o *Option[T]) UnmarshalJSON(data []byte) error {
	switch strings.TrimSpace(string(data)) {
	case "", "null", "{}":
		*o = None[T]()
		return nil
	}

	var value T
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	*o = Some(value)
	return nil
}
