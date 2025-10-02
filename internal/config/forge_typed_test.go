package config

import (
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/foundation"
)

func TestForgeTyped(t *testing.T) {
	t.Run("Valid forge types", func(t *testing.T) {
		testCases := []struct {
			input    string
			expected ForgeTyped
		}{
			{"github", ForgeTypedGitHub},
			{"GitHub", ForgeTypedGitHub},
			{"  gitlab  ", ForgeTypedGitLab},
			{"FORGEJO", ForgeTypedForgejo},
		}

		for _, tc := range testCases {
			result := ParseForgeTyped(tc.input)
			if result.IsErr() {
				t.Errorf("Expected %s to parse successfully, got error: %v", tc.input, result.UnwrapErr())
				continue
			}

			if result.Unwrap() != tc.expected {
				t.Errorf("Expected %s to parse to %v, got %v", tc.input, tc.expected, result.Unwrap())
			}
		}
	})

	t.Run("Invalid forge type", func(t *testing.T) {
		result := ParseForgeTyped("bitbucket")
		if result.IsOk() {
			t.Error("Expected bitbucket to fail parsing")
		}

		// Check error details
		err := result.UnwrapErr()
		var classified *foundation.ClassifiedError
		if foundation.AsClassified(err, &classified) {
			if classified.Code != foundation.ErrorCodeValidation {
				t.Error("Expected validation error code")
			}

			if !classified.IsUserFacing() {
				t.Error("Expected error to be user-facing")
			}
		} else {
			t.Error("Expected classified error")
		}
	})

	t.Run("Normalization fallback", func(t *testing.T) {
		normalized := NormalizeForgeTyped("invalid")
		if normalized != ForgeTypedGitHub {
			t.Error("Expected invalid input to normalize to GitHub default")
		}
	})

	t.Run("Legacy compatibility", func(t *testing.T) {
		// Test conversion to legacy
		legacy := ForgeTypedGitLab.ToLegacyForgeType()
		if legacy != ForgeGitLab {
			t.Error("Expected GitLab typed to convert to legacy GitLab")
		}

		// Test conversion from legacy
		typed := FromLegacyForgeType(ForgeGitHub)
		if typed != ForgeTypedGitHub {
			t.Error("Expected legacy GitHub to convert to typed GitHub")
		}
	})
}

func TestTypedForgeConfig(t *testing.T) {
	t.Run("Valid configuration", func(t *testing.T) {
		config := TypedForgeConfig{
			Type:    ForgeTypedGitHub,
			BaseURL: foundation.Some("https://api.github.com"),
			Token:   foundation.Some("ghp_test_token"),
		}

		result := config.Validate()
		if !result.Valid {
			t.Errorf("Expected valid config to pass validation, got errors: %v", result.Errors)
		}
	})

	t.Run("Invalid forge type", func(t *testing.T) {
		config := TypedForgeConfig{
			Type: ForgeTyped{"invalid"},
		}

		result := config.Validate()
		if result.Valid {
			t.Error("Expected invalid forge type to fail validation")
		}

		// Check that we get the right error
		found := false
		for _, err := range result.Errors {
			if err.Code == "one_of" {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected to find 'one_of' validation error")
		}
	})

	t.Run("Empty token validation", func(t *testing.T) {
		config := TypedForgeConfig{
			Type:  ForgeTypedGitHub,
			Token: foundation.Some(""), // Empty token
		}

		result := config.Validate()
		if result.Valid {
			t.Error("Expected empty token to fail validation")
		}

		// Check that we get the right error
		found := false
		for _, err := range result.Errors {
			if err.Field == "token" && err.Code == "not_empty" {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected to find token 'not_empty' validation error")
		}
	})

	t.Run("Optional fields", func(t *testing.T) {
		config := TypedForgeConfig{
			Type: ForgeTypedGitLab,
			// No optional fields provided
		}

		result := config.Validate()
		if !result.Valid {
			t.Errorf("Expected config with no optional fields to be valid, got errors: %v", result.Errors)
		}

		// Verify Option handling
		if config.Token.IsSome() {
			t.Error("Expected token to be None when not provided")
		}

		if config.BaseURL.IsSome() {
			t.Error("Expected base_url to be None when not provided")
		}
	})
}

func TestTypedForgeConfigWithTestForgeFactory(t *testing.T) {
	factory := NewTestForgeConfigFactory()

	t.Run("GitHub forge configuration validation", func(t *testing.T) {
		githubForge := factory.CreateGitHubForge("test")

		// Convert to TypedForgeConfig for validation
		typedConfig := TypedForgeConfig{
			Type:    ForgeTypedGitHub,
			BaseURL: foundation.Some(githubForge.APIURL),
			Token:   foundation.Some(githubForge.Auth.Token),
		}

		result := typedConfig.Validate()
		if !result.Valid {
			t.Errorf("Expected realistic GitHub forge config to pass validation, got errors: %v", result.Errors)
		}

		// Verify realistic values
		if typedConfig.Type != ForgeTypedGitHub {
			t.Errorf("Expected GitHub forge type, got %v", typedConfig.Type)
		}

		if !typedConfig.Token.IsSome() {
			t.Error("Expected GitHub forge to have token authentication")
		}

		tokenValue := typedConfig.Token.Unwrap()
		if tokenValue == "" {
			t.Error("Expected non-empty token for GitHub forge")
		}
	})

	t.Run("GitLab forge configuration validation", func(t *testing.T) {
		gitlabForge := factory.CreateGitLabForge("test")

		typedConfig := TypedForgeConfig{
			Type:    ForgeTypedGitLab,
			BaseURL: foundation.Some(gitlabForge.APIURL),
			Token:   foundation.Some(gitlabForge.Auth.Token),
		}

		result := typedConfig.Validate()
		if !result.Valid {
			t.Errorf("Expected realistic GitLab forge config to pass validation, got errors: %v", result.Errors)
		}

		// Verify GitLab-specific characteristics
		if typedConfig.Type != ForgeTypedGitLab {
			t.Errorf("Expected GitLab forge type, got %v", typedConfig.Type)
		}

		baseURL := typedConfig.BaseURL.Unwrap()
		if baseURL != "https://gitlab.com/api/v4" {
			t.Errorf("Expected GitLab API URL, got %s", baseURL)
		}
	})

	t.Run("Forgejo forge configuration validation", func(t *testing.T) {
		forgejoForge := factory.CreateForgejoForge("test")

		typedConfig := TypedForgeConfig{
			Type:    ForgeTypedForgejo,
			BaseURL: foundation.Some(forgejoForge.APIURL),
			Token:   foundation.Some(forgejoForge.Auth.Token),
		}

		result := typedConfig.Validate()
		if !result.Valid {
			t.Errorf("Expected realistic Forgejo forge config to pass validation, got errors: %v", result.Errors)
		}

		// Verify Forgejo-specific characteristics
		if typedConfig.Type != ForgeTypedForgejo {
			t.Errorf("Expected Forgejo forge type, got %v", typedConfig.Type)
		}

		baseURL := typedConfig.BaseURL.Unwrap()
		expectedURL := "https://test-forge.example.com/api/v1"
		if baseURL != expectedURL {
			t.Errorf("Expected Forgejo API URL %s, got %s", expectedURL, baseURL)
		}
	})

	t.Run("Multi-platform forge type conversion", func(t *testing.T) {
		testCases := []struct {
			name         string
			forgeCreator func(string) *ForgeConfig
			expectedType ForgeTyped
		}{
			{
				name:         "GitHub",
				forgeCreator: factory.CreateGitHubForge,
				expectedType: ForgeTypedGitHub,
			},
			{
				name:         "GitLab",
				forgeCreator: factory.CreateGitLabForge,
				expectedType: ForgeTypedGitLab,
			},
			{
				name:         "Forgejo",
				forgeCreator: factory.CreateForgejoForge,
				expectedType: ForgeTypedForgejo,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				forge := tc.forgeCreator("test")

				// Test legacy conversion
				legacyType := FromLegacyForgeType(forge.Type)
				if legacyType != tc.expectedType {
					t.Errorf("Expected legacy %s to convert to typed %v, got %v",
						tc.name, tc.expectedType, legacyType)
				}

				// Test round-trip conversion
				backToLegacy := tc.expectedType.ToLegacyForgeType()
				if backToLegacy != forge.Type {
					t.Errorf("Expected typed %v to convert back to legacy %v, got %v",
						tc.expectedType, forge.Type, backToLegacy)
				}
			})
		}

		t.Logf("✓ Multi-platform forge type conversions: %d platforms validated", len(testCases))
	})
}
