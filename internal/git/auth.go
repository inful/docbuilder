package git

import (
	"github.com/go-git/go-git/v5/plumbing/transport"

	"git.home.luguber.info/inful/docbuilder/internal/auth"
	appcfg "git.home.luguber.info/inful/docbuilder/internal/config"
)

// getAuth returns a go-git AuthMethod for the given AuthConfig using the DocBuilder auth manager.
func (c *Client) getAuth(authCfg *appcfg.AuthConfig) (transport.AuthMethod, error) {
	// Use the auth manager to create authentication
	return auth.CreateAuth(authCfg)
}
