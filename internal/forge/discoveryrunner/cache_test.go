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

func TestCache_MergeUpdatesResultAndClearsError(t *testing.T) {
	c := NewCache()

	initial := &forge.DiscoveryResult{Repositories: []*forge.Repository{{CloneURL: "https://example.com/one.git"}}}
	c.Update(initial)
	c.SetError(forgeError("stale"))

	c.Merge(func(current *forge.DiscoveryResult) *forge.DiscoveryResult {
		require.Same(t, initial, current)
		next := &forge.DiscoveryResult{}
		*next = *current
		next.Repositories = append([]*forge.Repository(nil), current.Repositories...)
		next.Repositories = append(next.Repositories, &forge.Repository{CloneURL: "https://example.com/two.git"})
		return next
	})

	res, err := c.Get()
	require.NoError(t, err)
	require.Len(t, res.Repositories, 2)
	require.Equal(t, "https://example.com/one.git", res.Repositories[0].CloneURL)
	require.Equal(t, "https://example.com/two.git", res.Repositories[1].CloneURL)
}

func TestCache_MergeNilResultIsNoOp(t *testing.T) {
	c := NewCache()

	initial := &forge.DiscoveryResult{Repositories: []*forge.Repository{{CloneURL: "https://example.com/one.git"}}}
	c.Update(initial)
	errBefore := forgeError("boom")
	c.SetError(errBefore)

	c.Merge(func(current *forge.DiscoveryResult) *forge.DiscoveryResult {
		require.Same(t, initial, current)
		return nil
	})

	res, err := c.Get()
	require.Equal(t, errBefore, err)
	require.Same(t, initial, res)
}

type forgeError string

func (e forgeError) Error() string { return string(e) }
