package config

import (
	"git.home.luguber.info/inful/docbuilder/internal/foundation/normalization"
)

// AuthType enumerates supported authentication methods (stringly for YAML compatibility)
type AuthType string

const (
	AuthTypeNone  AuthType = "none"
	AuthTypeSSH   AuthType = "ssh"
	AuthTypeToken AuthType = "token"
	AuthTypeBasic AuthType = "basic"
)

// NormalizeAuthType canonicalizes an auth type string (case-insensitive) or returns empty if unknown.
var authTypeNormalizer = normalization.NewNormalizer(map[string]AuthType{
       "none":  AuthTypeNone,
       "ssh":   AuthTypeSSH,
       "token": AuthTypeToken,
       "basic": AuthTypeBasic,
}, AuthTypeNone)

// NormalizeAuthType canonicalizes an auth type string (case-insensitive) or returns empty if unknown.
func NormalizeAuthType(raw string) AuthType {
       return authTypeNormalizer.Normalize(raw)
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
