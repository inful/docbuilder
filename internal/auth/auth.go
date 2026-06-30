package auth

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	derrors "git.home.luguber.info/inful/docbuilder/internal/foundation/errors"
)

// CreateAuth builds a go-git transport.AuthMethod from the given config.
//
// Behavior:
//   - nil cfg → returns (nil, nil); the git client treats nil as "no auth".
//   - AuthTypeNone or empty type → (nil, nil).
//   - AuthTypeSSH → loads the SSH key (cfg.KeyPath, defaults to
//     $HOME/.ssh/id_rsa) and returns an ssh.PublicKeys.
//   - AuthTypeToken → returns an http.BasicAuth with username "token"
//     (or cfg.Username if set) and the token as the password.
//   - AuthTypeBasic → returns an http.BasicAuth with cfg.Username and
//     cfg.Password.
//
// Errors are wrapped with %w so callers can errors.Is/As them.
func CreateAuth(authCfg *config.AuthConfig) (transport.AuthMethod, error) {
	if authCfg == nil {
		return transport.AuthMethod(nil), nil
	}

	switch authCfg.Type {
	case config.AuthTypeNone, "":
		return transport.AuthMethod(nil), nil
	case config.AuthTypeSSH:
		return newSSHAuth(authCfg)
	case config.AuthTypeToken:
		return newTokenAuth(authCfg)
	case config.AuthTypeBasic:
		return newBasicAuth(authCfg)
	default:
		return nil, derrors.NewError(derrors.CategoryConfig, "auth: unsupported authentication type").
			WithContext("type", authCfg.Type).
			Build()
	}
}

func newSSHAuth(authCfg *config.AuthConfig) (transport.AuthMethod, error) {
	keyPath := authCfg.KeyPath
	if keyPath == "" {
		keyPath = filepath.Join(os.Getenv("HOME"), ".ssh", "id_rsa")
	}

	// Validate up front so callers get a clear error before go-git does.
	if _, err := os.Stat(keyPath); os.IsNotExist(err) { //nolint:gosec // keyPath is a local, user-configured file path
		return nil, derrors.NewError(derrors.CategoryNotFound, "auth: SSH key file does not exist").
			WithContext("path", keyPath).
			Build()
	}

	publicKeys, err := ssh.NewPublicKeysFromFile("git", keyPath, "")
	if err != nil {
		return nil, derrors.WrapError(err, derrors.CategoryAuth, "auth: failed to load SSH key").
			WithContext("path", keyPath).
			Build()
	}
	return publicKeys, nil
}

func newTokenAuth(authCfg *config.AuthConfig) (transport.AuthMethod, error) {
	if authCfg.Token == "" {
		return nil, errors.New("auth: token authentication requires a token")
	}

	username := authCfg.Username
	if username == "" {
		// Most Git hosting services use "token" as the username for token
		// auth. Some GitLab setups expect "oauth2" instead; allowing override
		// via config keeps tokens out of clone URLs (safer) while supporting
		// those servers.
		username = "token"
	}

	return &http.BasicAuth{Username: username, Password: authCfg.Token}, nil
}

func newBasicAuth(authCfg *config.AuthConfig) (transport.AuthMethod, error) {
	if authCfg.Username == "" {
		return nil, errors.New("auth: basic authentication requires a username")
	}
	if authCfg.Password == "" {
		return nil, errors.New("auth: basic authentication requires a password")
	}
	return &http.BasicAuth{Username: authCfg.Username, Password: authCfg.Password}, nil
}
