package config

// AuthType enumerates supported authentication methods (stringly for YAML compatibility)
type AuthType string

const (
    AuthTypeNone  AuthType = "none"
    AuthTypeSSH   AuthType = "ssh"
    AuthTypeToken AuthType = "token"
    AuthTypeBasic AuthType = "basic"
)

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
