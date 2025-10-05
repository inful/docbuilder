package git

import (
	"git.home.luguber.info/inful/docbuilder/internal/auth"
	appcfg "git.home.luguber.info/inful/docbuilder/internal/config"
	"github.com/go-git/go-git/v5/plumbing/transport"
)

func (c *Client) getAuthentication(authConfig *appcfg.AuthConfig) (transport.AuthMethod, error) {
	// Use the auth manager to create authentication
	return auth.CreateAuth(authConfig)
}
