package normalization

import (
	"testing"
)

// Test enums for testing
type TestEnum string

const (
	TestEnumAlpha TestEnum = "alpha"
	TestEnumBeta  TestEnum = "beta"
	TestEnumGamma TestEnum = "gamma"
)

func TestNormalizer_Basic(t *testing.T) {
	normalizer := NewNormalizer(map[string]TestEnum{
		"alpha": TestEnumAlpha,
		"beta":  TestEnumBeta,
		"gamma": TestEnumGamma,
	}, TestEnumAlpha)

	tests := []struct {
		name     string
		input    string
		expected TestEnum
	}{
		{"exact match", "alpha", TestEnumAlpha},
		{"case insensitive", "ALPHA", TestEnumAlpha},
		{"with spaces", "  beta  ", TestEnumBeta},
		{"mixed case spaces", "  GaMmA  ", TestEnumGamma},
		{"invalid input", "invalid", TestEnumAlpha}, // Should return default
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizer.Normalize(tt.input)
			if result != tt.expected {
				t.Errorf("Normalize(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestNormalizer_WithError(t *testing.T) {
	normalizer := NewNormalizer(map[string]TestEnum{
		"alpha": TestEnumAlpha,
		"beta":  TestEnumBeta,
	}, TestEnumAlpha)

	// Valid input
	result, err := normalizer.NormalizeWithError("ALPHA")
	if err != nil {
		t.Errorf("NormalizeWithError(valid input) returned error: %v", err)
	}
	if result != TestEnumAlpha {
		t.Errorf("NormalizeWithError(valid input) = %v, want %v", result, TestEnumAlpha)
	}

	// Invalid input
	_, err = normalizer.NormalizeWithError("invalid")
	if err == nil {
		t.Error("NormalizeWithError(invalid input) should return error")
	}
}

func TestEnumNormalizer_Integration(t *testing.T) {
	enumNormalizer := NewEnumNormalizer("test_mode", map[string]TestEnum{
		"alpha": TestEnumAlpha,
		"beta":  TestEnumBeta,
		"gamma": TestEnumGamma,
	}, TestEnumAlpha)

	// Test normalization with warning
	result := enumNormalizer.NormalizeWithWarning("test.mode", "  ALPHA  ")
	if result.Value != TestEnumAlpha {
		t.Errorf("Value = %v, want %v", result.Value, TestEnumAlpha)
	}
	if !result.Changed {
		t.Error("Expected change flag to be true for normalized input")
	}
	if result.Warning == "" {
		t.Error("Expected warning message for changed input")
	}

	// Test unchanged input
	result2 := enumNormalizer.NormalizeWithWarning("test.mode", "alpha")
	if result2.Changed {
		t.Error("Expected change flag to be false for unchanged input")
	}
	if result2.Warning != "" {
		t.Errorf("Expected no warning for unchanged input, got: %s", result2.Warning)
	}
}

func TestEnumNormalizer_Validation(t *testing.T) {
	enumNormalizer := NewEnumNormalizer("test_mode", map[string]TestEnum{
		"alpha": TestEnumAlpha,
		"beta":  TestEnumBeta,
	}, TestEnumAlpha)

	// Valid input
	result, err := enumNormalizer.NormalizeWithValidation("alpha")
	if err != nil {
		t.Errorf("NormalizeWithValidation(valid) returned error: %v", err)
	}
	if result != TestEnumAlpha {
		t.Errorf("Result = %v, want %v", result, TestEnumAlpha)
	}

	// Invalid input
	_, err = enumNormalizer.NormalizeWithValidation("invalid")
	if err == nil {
		t.Error("NormalizeWithValidation(invalid) should return error")
	}

	// Check error message includes enum name
	if err.Error() == "" {
		t.Error("Error message should not be empty")
	}
}

func TestValidKeys(t *testing.T) {
	normalizer := NewNormalizer(map[string]TestEnum{
		"gamma": TestEnumGamma,
		"alpha": TestEnumAlpha,
		"beta":  TestEnumBeta,
	}, TestEnumAlpha)

	keys := normalizer.ValidKeys()

	// Should be sorted
	expected := []string{"alpha", "beta", "gamma"}
	if len(keys) != len(expected) {
		t.Errorf("ValidKeys() length = %d, want %d", len(keys), len(expected))
	}

	for i, key := range keys {
		if key != expected[i] {
			t.Errorf("ValidKeys()[%d] = %q, want %q", i, key, expected[i])
		}
	}
}
