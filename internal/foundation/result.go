// Package foundation provides generic utilities for type-safe operations.
package foundation

import "fmt"

// Result represents an operation that can either succeed with value T or fail with error E.
// This replaces the common pattern of returning (T, error) with a more explicit type.
type Result[T any, E error] struct {
	value T
	err   E
	isOk  bool
}

// Ok creates a successful Result with the given value.
func Ok[T any, E error](value T) Result[T, E] {
	return Result[T, E]{
		value: value,
		isOk:  true,
	}
}

// Err creates a failed Result with the given error.
func Err[T any, E error](err E) Result[T, E] {
	return Result[T, E]{
		err:  err,
		isOk: false,
	}
}

// IsOk returns true if the Result represents a successful operation.
func (r Result[T, E]) IsOk() bool {
	return r.isOk
}

// IsErr returns true if the Result represents a failed operation.
func (r Result[T, E]) IsErr() bool {
	return !r.isOk
}

// Unwrap returns the value if Ok, panics if Err.
// Use this only when you're certain the Result is Ok.
func (r Result[T, E]) Unwrap() T {
	if !r.isOk {
		panic(fmt.Sprintf("called Unwrap on Err result: %v", r.err))
	}
	return r.value
}

// UnwrapOr returns the value if Ok, otherwise returns the fallback.
func (r Result[T, E]) UnwrapOr(fallback T) T {
	if r.isOk {
		return r.value
	}
	return fallback
}

// UnwrapErr returns the error if Err, panics if Ok.
func (r Result[T, E]) UnwrapErr() E {
	if r.isOk {
		panic("called UnwrapErr on Ok result")
	}
	return r.err
}

// Match executes onOk if successful, onErr if failed.
func (r Result[T, E]) Match(onOk func(T), onErr func(E)) {
	if r.isOk {
		onOk(r.value)
	} else {
		onErr(r.err)
	}
}

// Map transforms a successful Result[T, E] to Result[U, E] using the given function.
// If the Result is an error, it returns the error unchanged.
func Map[T, U any, E error](r Result[T, E], fn func(T) U) Result[U, E] {
	if r.isOk {
		return Ok[U, E](fn(r.value))
	}
	return Err[U, E](r.err)
}

// FromTuple creates a Result from the traditional Go (value, error) pattern.
func FromTuple[T any, E error](value T, err E) Result[T, E] {
	if any(err) != nil {
		return Err[T, E](err)
	}
	return Ok[T, E](value)
}
