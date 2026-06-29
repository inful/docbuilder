package foundation

import (
	"errors"
	"strconv"
	"testing"
)

func TestResult(t *testing.T) {
	t.Run("Ok result", func(t *testing.T) {
		result := Ok[string, error]("success")

		if !result.IsOk() {
			t.Error("Expected result to be Ok")
		}

		if result.IsErr() {
			t.Error("Expected result to not be Err")
		}

		if result.Unwrap() != "success" {
			t.Error("Expected unwrap to return 'success'")
		}
	})

	t.Run("Err result", func(t *testing.T) {
		testErr := errors.New("test error")
		result := Err[string, error](testErr)

		if result.IsOk() {
			t.Error("Expected result to not be Ok")
		}

		if !result.IsErr() {
			t.Error("Expected result to be Err")
		}

		if !errors.Is(result.UnwrapErr(), testErr) {
			t.Error("Expected unwrap error to match test error")
		}
	})

	t.Run("Map operation", func(t *testing.T) {
		result := Ok[int, error](5)
		mapped := Map(result, func(i int) string {
			return "value is " + strconv.Itoa(i)
		})

		if !mapped.IsOk() {
			t.Error("Expected mapped result to be Ok")
		}
	})

	t.Run("FromTuple", func(t *testing.T) {
		// Test success case
		result := FromTuple[string, error]("test", nil)
		if !result.IsOk() {
			t.Error("Expected result from successful tuple to be Ok")
		}

		// Test error case
		testErr := errors.New("test error")
		result = FromTuple[string, error]("", testErr)
		if !result.IsErr() {
			t.Error("Expected result from error tuple to be Err")
		}
	})
}

func TestOption(t *testing.T) {
	t.Run("Some option", func(t *testing.T) {
		option := Some("value")

		if !option.IsSome() {
			t.Error("Expected option to be Some")
		}

		if option.IsNone() {
			t.Error("Expected option to not be None")
		}

		if option.Unwrap() != "value" {
			t.Error("Expected unwrap to return 'value'")
		}
	})

	t.Run("None option", func(t *testing.T) {
		option := None[string]()

		if option.IsSome() {
			t.Error("Expected option to not be Some")
		}

		if !option.IsNone() {
			t.Error("Expected option to be None")
		}

		if option.UnwrapOr("default") != "default" {
			t.Error("Expected unwrap or to return 'default'")
		}
	})

	t.Run("FromPointer", func(t *testing.T) {
		// Test non-nil pointer
		value := "test"
		option := FromPointer(&value)
		if !option.IsSome() {
			t.Error("Expected option from non-nil pointer to be Some")
		}

		// Test nil pointer
		var nilPtr *string
		option = FromPointer(nilPtr)
		if !option.IsNone() {
			t.Error("Expected option from nil pointer to be None")
		}
	})
}

// Note: ClassifiedError tests have been moved to internal/foundation/errors/errors_test.go
// Note: Normalizer tests have been moved to internal/foundation/normalization/normalizer_test.go
