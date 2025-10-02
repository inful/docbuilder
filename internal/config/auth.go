package config

import "strings"

// AuthType enumerates supported authentication methods (stringly for YAML compatibility)
type AuthType string

const (
	AuthTypeNone  AuthType = "none"
	AuthTypeSSH   AuthType = "ssh"
	AuthTypeToken AuthType = "token"
	AuthTypeBasic AuthType = "basic"
)

// NormalizeAuthType canonicalizes an auth type string (case-insensitive) or returns empty if unknown.
func NormalizeAuthType(raw string) AuthType {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case string(AuthTypeNone):
		return AuthTypeNone
	case string(AuthTypeSSH):
		return AuthTypeSSH
	case string(AuthTypeToken):
		return AuthTypeToken
	case string(AuthTypeBasic):
		return AuthTypeBasic
	default:
		return ""
	}
}

// IsValid reports whether the AuthType is a known value.
func (a AuthType) IsValid() bool {
	return NormalizeAuthType(string(a)) != ""
}

// AuthConfig represents authentication configuration
type AuthConfig struct {
	Type     AuthType `yaml:"type"` // ssh|token|basic|none
	Username string   `yaml:"username,omitempty"`
	Password string   `yaml:"password,omitempty"`
	Token    string   `yaml:"token,omitempty"`
	KeyPath  string   `yaml:"key_path,omitempty"`
}

// IsZero reports whether no auth method specified.
func (a *AuthConfig) IsZero() bool { return a == nil || a.Type == "" || a.Type == AuthTypeNone }
