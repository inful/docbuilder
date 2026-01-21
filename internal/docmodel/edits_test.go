package docmodel

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"git.home.luguber.info/inful/docbuilder/internal/frontmatter"
	"git.home.luguber.info/inful/docbuilder/internal/markdown"
)

func TestParsedDoc_ApplyBodyEdits_EmptyEditsIsNoop(t *testing.T) {
	src := "---\n" +
		"title: x\n" +
		"---\n" +
		"Hello world\n"

	doc, err := Parse([]byte(src), Options{})
	require.NoError(t, err)

	out, err := doc.ApplyBodyEdits(nil)
	require.NoError(t, err)
	require.Equal(t, doc.Bytes(), out)
}

func TestParsedDoc_ApplyBodyEdits_PreservesFrontmatterBytes(t *testing.T) {
	src := "---\n" +
		"title: x\n" +
		"weird: ' spacing  '   \n" +
		"---\n" +
		"Hello world\n"

	doc, err := Parse([]byte(src), Options{})
	require.NoError(t, err)

	body := doc.Body()
	idx := bytes.Index(body, []byte("world"))
	require.NotEqual(t, -1, idx)

	out, err := doc.ApplyBodyEdits([]markdown.Edit{{
		Start:       idx,
		End:         idx + len("world"),
		Replacement: []byte("there"),
	}})
	require.NoError(t, err)

	fmRawBefore, _, hadBefore, styleBefore, err := frontmatter.Split(doc.Bytes())
	require.NoError(t, err)
	fmRawAfter, bodyAfter, hadAfter, styleAfter, err := frontmatter.Split(out)
	require.NoError(t, err)

	require.True(t, hadBefore)
	require.True(t, hadAfter)
	require.Equal(t, fmRawBefore, fmRawAfter)
	require.Equal(t, styleBefore, styleAfter)
	require.Contains(t, string(bodyAfter), "Hello there")
}
