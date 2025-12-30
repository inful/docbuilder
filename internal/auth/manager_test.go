package auth

import (
	"testing"

	"github.com/go-git/go-git/v5/plumbing/transport/http"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

func TestManager_CreateAuth(t *testing.T) {
	manager := NewManager()

	tests := []struct {
		name        string
		authConfig  *config.AuthConfig
		expectNil   bool
		expectError bool
		description string
	}{
		{
			name:        "nil config",
			authConfig:  nil,
			expectNil:   true,
			expectError: false,
			description: "nil config should result in no authentication",
		},
		{
			name: "none auth",
			authConfig: &config.AuthConfig{
				Type: config.AuthTypeNone,
			},
			expectNil:   true,
			expectError: false,
			description: "none auth should result in no authentication",
		},
		{
			name: "token auth - valid",
			authConfig: &config.AuthConfig{
				Type:  config.AuthTypeToken,
				Token: "test-token",
			},
			expectNil:   false,
			expectError: false,
			description: "valid token auth should create http.BasicAuth",
		},
		{
			name: "token auth - missing token",
			authConfig: &config.AuthConfig{
				Type: config.AuthTypeToken,
			},
			expectNil:   true,
			expectError: true,
			description: "token auth without token should fail",
		},
		{
			name: "basic auth - valid",
			authConfig: &config.AuthConfig{
				Type:     config.AuthTypeBasic,
				Username: "testuser",
				Password: "testpass",
			},
			expectNil:   false,
			expectError: false,
			description: "valid basic auth should create http.BasicAuth",
		},
		{
			name: "basic auth - missing username",
			authConfig: &config.AuthConfig{
				Type:     config.AuthTypeBasic,
				Password: "testpass",
			},
			expectNil:   true,
			expectError: true,
			description: "basic auth without username should fail",
		},
		{
			name: "unsupported auth type",
			authConfig: &config.AuthConfig{
				Type: "unsupported",
			},
			expectNil:   true,
			expectError: true,
			description: "unsupported auth type should fail",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			auth, err := manager.CreateAuth(tt.authConfig)

			if tt.expectError && err == nil {
				t.Errorf("CreateAuth() expected error but got none - %s", tt.description)
			}
			if !tt.expectError && err != nil {
				t.Errorf("CreateAuth() unexpected error: %v - %s", err, tt.description)
			}
			if tt.expectNil && auth != nil {
				t.Errorf("CreateAuth() expected nil auth but got %T - %s", auth, tt.description)
			}
			if !tt.expectNil && !tt.expectError && auth == nil {
				t.Errorf("CreateAuth() expected non-nil auth but got nil - %s", tt.description)
			}

			// Additional type-specific checks
			if !tt.expectError && !tt.expectNil {
				switch tt.authConfig.Type {
				case config.AuthTypeToken:
					if basicAuth, ok := auth.(*http.BasicAuth); ok {
						if basicAuth.Username != "token" {
							t.Errorf("Token auth should use 'token' as username, got: %s", basicAuth.Username)
						}
						if basicAuth.Password != tt.authConfig.Token {
							t.Errorf("Token auth password should match token")
						}
					} else {
						t.Errorf("Token auth should create http.BasicAuth, got: %T", auth)
					}
				case config.AuthTypeBasic:
					if basicAuth, ok := auth.(*http.BasicAuth); ok {
						if basicAuth.Username != tt.authConfig.Username {
							t.Errorf("Basic auth username mismatch")
						}
						if basicAuth.Password != tt.authConfig.Password {
							t.Errorf("Basic auth password mismatch")
						}
					} else {
						t.Errorf("Basic auth should create http.BasicAuth, got: %T", auth)
					}
				}
			}
		})
	}
}

func TestConvenienceFunctions(t *testing.T) {
	// Test package-level convenience function
	authConfig := &config.AuthConfig{
		Type:  config.AuthTypeToken,
		Token: "test-token",
	}

	// Test CreateAuth convenience function
	auth, err := CreateAuth(authConfig)
	if err != nil {
		t.Errorf("CreateAuth() convenience function error: %v", err)
	}
	if auth == nil {
		t.Errorf("CreateAuth() convenience function returned nil")
	}
}
