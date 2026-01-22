package discoveryrunner

import (
	"testing"

	"github.com/stretchr/testify/require"

	"git.home.luguber.info/inful/docbuilder/internal/forge"
)

func TestCache_Empty(t *testing.T) {
	c := NewCache()

	res, err := c.Get()
	require.NoError(t, err)
	require.Nil(t, res)
	require.False(t, c.HasResult())
	require.Nil(t, c.GetError())
}

func TestCache_UpdateThenSetErrorPreservesResult(t *testing.T) {
	c := NewCache()

	r := &forge.DiscoveryResult{}
	c.Update(r)
	require.True(t, c.HasResult())

	someErr := forgeError("boom")
	c.SetError(someErr)

	res, err := c.Get()
	require.Same(t, r, res)
	require.Equal(t, someErr, err)
}

type forgeError string

func (e forgeError) Error() string { return string(e) }
