package foundation

import (
	"errors"
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
			return "value is " + string(rune(i+'0'))
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

func TestNormalizer(t *testing.T) {
	normalizer := NewNormalizer(map[string]string{
		"github":  "github",
		"gitlab":  "gitlab",
		"forgejo": "forgejo",
	}, "github")

	t.Run("Valid values", func(t *testing.T) {
		if normalizer.Normalize("GitHub") != "github" {
			t.Error("Expected 'GitHub' to normalize to 'github'")
		}

		if normalizer.Normalize(" gitlab ") != "gitlab" {
			t.Error("Expected ' gitlab ' to normalize to 'gitlab'")
		}
	})

	t.Run("Invalid value", func(t *testing.T) {
		if normalizer.Normalize("bitbucket") != "github" {
			t.Error("Expected 'bitbucket' to return default 'github'")
		}
	})

	t.Run("With error", func(t *testing.T) {
		_, err := normalizer.NormalizeWithError("invalid")
		if err == nil {
			t.Error("Expected error for invalid value")
		}
	})
}

func TestClassifiedError(t *testing.T) {
	t.Run("Basic error creation", func(t *testing.T) {
		err := NewError(ErrorCodeValidation, "test message").
			WithSeverity(SeverityWarning).
			WithComponent("test").
			Build()

		if err.Code != ErrorCodeValidation {
			t.Error("Expected validation error code")
		}

		if err.Severity != SeverityWarning {
			t.Error("Expected warning severity")
		}

		if err.Component != "test" {
			t.Error("Expected component to be 'test'")
		}
	})

	t.Run("Error detection", func(t *testing.T) {
		err := ValidationError("test validation").Build()

		if !IsErrorCode(err, ErrorCodeValidation) {
			t.Error("Expected error to be validation error")
		}

		var classified *ClassifiedError
		if !AsClassified(err, &classified) {
			t.Error("Expected to extract classified error")
		}

		if classified.Code != ErrorCodeValidation {
			t.Error("Expected extracted error to have validation code")
		}
	})
}

func TestValidation(t *testing.T) {
	t.Run("Required validator", func(t *testing.T) {
		validator := Required[string]("name")

		result := validator("test")
		if !result.Valid {
			t.Error("Expected non-empty string to be valid")
		}

		result = validator("")
		if result.Valid {
			t.Error("Expected empty string to be invalid")
		}
	})

	t.Run("String validators", func(t *testing.T) {
		chain := NewValidatorChain(
			StringNotEmpty("field"),
			StringMinLength("field", 3),
			StringMaxLength("field", 10),
		)

		result := chain.Validate("test")
		if !result.Valid {
			t.Error("Expected 'test' to be valid")
		}

		result = chain.Validate("")
		if result.Valid {
			t.Error("Expected empty string to be invalid")
		}

		result = chain.Validate("ab")
		if result.Valid {
			t.Error("Expected string too short to be invalid")
		}

		result = chain.Validate("this is too long")
		if result.Valid {
			t.Error("Expected string too long to be invalid")
		}
	})

	t.Run("OneOf validator", func(t *testing.T) {
		validator := OneOf("forge", []string{"github", "gitlab", "forgejo"})

		result := validator("github")
		if !result.Valid {
			t.Error("Expected 'github' to be valid")
		}

		result = validator("bitbucket")
		if result.Valid {
			t.Error("Expected 'bitbucket' to be invalid")
		}
	})
}
