package pipeline

import (
	"testing"

	"github.com/stretchr/testify/require"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// A link may wrap an image: "[![Logo](logo.png)](guide.md)". The image
// is rewritten by the image pass, and the link by the link pass.
func TestALinkThatWrapsAnImage(t *testing.T) {
	doc := &Document{
		Path:       "repo/section/page.md",
		Repository: "repo",
		Section:    "section",
		Content:    "[![Logo](images/logo.png)](guide.md)",
	}

	_, err := rewriteRelativeLinks(&config.Config{})(doc)
	require.NoError(t, err)
	_, err = rewriteImageLinks(doc)
	require.NoError(t, err)

	require.Equal(t, "[![Logo](/repo/section/images/logo.png)](/repo/section/guide)", doc.Content)
}
