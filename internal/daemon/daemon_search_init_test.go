package daemon

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// The daemon serves the site it builds, so when nobody chose a provider
// the daemon chooses the server-side one: the site must ship the client
// that asks the server.
func TestInitSearchChoosesTheServerForTheDaemon(t *testing.T) {
	cfg := &config.Config{
		Version: "2.0",
		Daemon: &config.DaemonConfig{
			Storage: config.StorageConfig{StateDir: t.TempDir()},
		},
		Search: &config.SearchConfig{},
	}

	d := &Daemon{optional: &optionalStack{}}
	require.NoError(t, d.initSearch(cfg))

	assert.Equal(t, config.SearchProviderServer, cfg.Search.Provider,
		"the daemon serves the site, so the search runs there")
	assert.NotNil(t, d.optional.search, "the daemon must keep the index")
}

// The operator chose the browser-side search: the daemon keeps no
// index.
func TestInitSearchKeepsTheOperatorsChoice(t *testing.T) {
	cfg := &config.Config{
		Version: "2.0",
		Daemon: &config.DaemonConfig{
			Storage: config.StorageConfig{StateDir: t.TempDir()},
		},
		Search: &config.SearchConfig{Provider: config.SearchProviderLunr},
	}

	d := &Daemon{optional: &optionalStack{}}
	require.NoError(t, d.initSearch(cfg))

	assert.Equal(t, config.SearchProviderLunr, cfg.Search.Provider)
	assert.Nil(t, d.optional.search, "the daemon must keep no index")
}
