package config

import "testing"

func TestNormalizeAuthType(t *testing.T) {
	tests := []struct {
		input    string
		expected AuthType
	}{
		{"ssh", AuthTypeSSH},
		{"SSH", AuthTypeSSH},
		{"token", AuthTypeToken},
		{"TOKEN", AuthTypeToken},
		{"basic", AuthTypeBasic},
		{"Basic", AuthTypeBasic},
		{"none", AuthTypeNone},
		{"NONE", AuthTypeNone},
		{"  ssh  ", AuthTypeSSH}, // trimming
		{"invalid", ""},
		{"", ""},
	}

	for _, test := range tests {
		result := NormalizeAuthType(test.input)
		if result != test.expected {
			t.Errorf("NormalizeAuthType(%q) = %q, want %q", test.input, result, test.expected)
		}
	}
}

func TestAuthType_IsValid(t *testing.T) {
	tests := []struct {
		authType AuthType
		expected bool
	}{
		{AuthTypeSSH, true},
		{AuthTypeToken, true},
		{AuthTypeBasic, true},
		{AuthTypeNone, true},
		{"invalid", false},
		{"", false},
	}

	for _, test := range tests {
		result := test.authType.IsValid()
		if result != test.expected {
			t.Errorf("AuthType(%q).IsValid() = %v, want %v", test.authType, result, test.expected)
		}
	}
}

func TestForgeType_IsValid(t *testing.T) {
	tests := []struct {
		forgeType ForgeType
		expected  bool
	}{
		{ForgeGitHub, true},
		{ForgeGitLab, true},
		{ForgeForgejo, true},
		{"invalid", false},
		{"", false},
	}

	for _, test := range tests {
		result := test.forgeType.IsValid()
		if result != test.expected {
			t.Errorf("ForgeType(%q).IsValid() = %v, want %v", test.forgeType, result, test.expected)
		}
	}
}
