package pipeline

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// A build of one repository that is namespaced by its forge keeps the
// forge in the path of the content. A link at the root of the site
// needs it too, or it points at a page that does not exist.
func TestTheRootRelativeLinkKeepsTheForgeOfANamespacedBuild(t *testing.T) {
	doc := &Document{
		Path:         "content/github/guide.md",
		Name:         "guide",
		Repository:   "repo",
		Forge:        "github",
		Section:      "guide",
		IsSingleRepo: true,
		Content:      "[root](/reference/other.md)\n",
	}

	transform := rewriteRelativeLinks(&config.Config{})
	_, err := transform(doc)
	assert.NoError(t, err)
	assert.Contains(t, doc.Content, "[root](/github/reference/other)")
}

// The image of a namespaced build needs the forge too.
func TestTheRootRelativeImageKeepsTheForgeOfANamespacedBuild(t *testing.T) {
	doc := &Document{
		Path:         "content/github/guide.md",
		Name:         "guide",
		Repository:   "repo",
		Forge:        "github",
		Section:      "guide",
		IsSingleRepo: true,
		Content:      "![logo](/images/logo.svg)\n",
	}

	_, err := rewriteImageLinks(doc)
	assert.NoError(t, err)
	assert.Contains(t, doc.Content, "![logo](/github/images/logo.svg)")
}

// A build without a forge keeps the links as they are.
func TestTheRootRelativeLinkOfAPlainBuildIsUnchanged(t *testing.T) {
	doc := &Document{
		Path:         "content/guide.md",
		Name:         "guide",
		Repository:   "repo",
		Section:      "guide",
		IsSingleRepo: true,
		Content:      "[root](/reference/other.md)\n",
	}

	transform := rewriteRelativeLinks(&config.Config{})
	_, err := transform(doc)
	assert.NoError(t, err)
	assert.Contains(t, doc.Content, "[root](/reference/other)")
}
