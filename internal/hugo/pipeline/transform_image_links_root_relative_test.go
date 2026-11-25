package pipeline

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A path that starts with a slash is relative to the root of the
// repository, and the documentation says that the repository (and the
// forge) are added to it. The image was published in the tree of its
// repository and the link pointed at the root of the site: the image
// was broken.
func TestRewriteImageLinks_RootRelativePathCarriesTheRepository(t *testing.T) {
	doc := &Document{
		Repository: "test-repo",
		Forge:      "gitlab",
		Section:    "guides",
		Content: `# Guide

![Logo](/images/logo.png)
<img src="/images/banner.png" alt="Banner" />
`,
	}

	_, err := rewriteImageLinks(doc)
	require.NoError(t, err)

	expected := `# Guide

![Logo](/gitlab/test-repo/images/logo.png)
<img src="/gitlab/test-repo/images/banner.png" alt="Banner" />
`

	assert.Equal(t, expected, doc.Content)
}
