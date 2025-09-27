package git

import (
    "fmt"
    "os"
    "path/filepath"

    appcfg "git.home.luguber.info/inful/docbuilder/internal/config"
    "github.com/go-git/go-git/v5/plumbing/transport"
    "github.com/go-git/go-git/v5/plumbing/transport/http"
    "github.com/go-git/go-git/v5/plumbing/transport/ssh"
)

func (c *Client) getAuthentication(auth *appcfg.AuthConfig) (transport.AuthMethod, error) {
    switch auth.Type {
    case appcfg.AuthTypeNone, "":
        return nil, nil
    case appcfg.AuthTypeSSH:
        if auth.KeyPath == "" { auth.KeyPath = filepath.Join(os.Getenv("HOME"), ".ssh", "id_rsa") }
        publicKeys, err := ssh.NewPublicKeysFromFile("git", auth.KeyPath, "")
        if err != nil { return nil, fmt.Errorf("failed to load SSH key from %s: %w", auth.KeyPath, err) }
        return publicKeys, nil
    case appcfg.AuthTypeToken:
        if auth.Token == "" { return nil, fmt.Errorf("token authentication requires a token") }
        return &http.BasicAuth{Username: "token", Password: auth.Token}, nil
    case appcfg.AuthTypeBasic:
        if auth.Username == "" || auth.Password == "" { return nil, fmt.Errorf("basic authentication requires username and password") }
        return &http.BasicAuth{Username: auth.Username, Password: auth.Password}, nil
    default:
        return nil, fmt.Errorf("unsupported authentication type: %s", auth.Type)
    }
}
