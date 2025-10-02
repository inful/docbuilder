package foundation

import "fmt"

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

// UnwrapOrElse returns the value if present, otherwise calls the function and returns its result.
func (o Option[T]) UnwrapOrElse(fn func() T) T {
	if o.present {
		return o.value
	}
	return fn()
}

// Match executes onSome if the Option has a value, onNone if empty.
func (o Option[T]) Match(onSome func(T), onNone func()) {
	if o.present {
		onSome(o.value)
	} else {
		onNone()
	}
}

// Map transforms an Option[T] to Option[U] using the given function.
// If the Option is None, it returns None[U].
func MapOption[T, U any](o Option[T], fn func(T) U) Option[U] {
	if o.present {
		return Some(fn(o.value))
	}
	return None[U]()
}

// FlatMap transforms an Option[T] to Option[U] using a function that returns Option[U].
// This prevents Option[Option[U]].
func FlatMapOption[T, U any](o Option[T], fn func(T) Option[U]) Option[U] {
	if o.present {
		return fn(o.value)
	}
	return None[U]()
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
