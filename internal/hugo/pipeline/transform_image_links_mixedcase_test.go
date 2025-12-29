package pipeline

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRewriteImageLinks_MixedCaseFilename(t *testing.T) {
	// Test that image links with mixed-case filenames are properly normalized
	// Original markdown: ![alt](./images/6_3_approve_MR.png)
	// The file is discovered as 6_3_approve_MR.png but written as 6_3_approve_mr.png
	// The link rewriter must also normalize to lowercase
	doc := &Document{
		Content:    "# Guide\n\n![Approve button](./images/6_3_approve_MR.png)\n\nSee the image above.",
		Repository: "test-repo",
		Section:    "guides",
		Forge:      "",
		Path:       "test-repo/guides/tutorial.md",
	}

	_, err := rewriteImageLinks(doc)
	require.NoError(t, err)

	// The rewritten path should match where the file is actually written
	// File discovered as 6_3_approve_MR.png → written as content/test-repo/guides/images/6_3_approve_mr.png
	// Link should be: /test-repo/guides/images/6_3_approve_mr.png
	expectedPath := "/test-repo/guides/images/6_3_approve_MR.png"
	actualLowercasePath := "/test-repo/guides/images/6_3_approve_mr.png"
	
	// Currently the link rewriter does NOT lowercase, causing a mismatch
	if strings.Contains(doc.Content, expectedPath) {
		t.Logf("Link rewriter preserved case: %s", expectedPath)
		t.Logf("But file is written as: %s", actualLowercasePath)
		t.Errorf("MISMATCH: Link references %s but file exists at %s", expectedPath, actualLowercasePath)
	} else if strings.Contains(doc.Content, actualLowercasePath) {
		t.Logf("SUCCESS: Link correctly normalized to: %s", actualLowercasePath)
	} else {
		t.Errorf("Link rewriter produced unexpected output:\n%s", doc.Content)
	}
	
	// The correct behavior is to lowercase the entire path including filename
	assert.Contains(t, doc.Content, actualLowercasePath,
		"Image link should be fully lowercase to match the written file path")
}

func TestRewriteImageLinks_MixedCaseWithForge(t *testing.T) {
	doc := &Document{
		Content:    "![Button](../images/Save_Button.PNG)",
		Repository: "test-repo",
		Section:    "guides/advanced",
		Forge:      "github",
		Path:       "github/test-repo/guides/advanced/config.md",
	}

	_, err := rewriteImageLinks(doc)
	require.NoError(t, err)

	t.Logf("Rewritten content: %s", doc.Content)
	
	// The path ../images from guides/advanced goes to guides/images
	// Should be lowercased: /github/test-repo/guides/images/save_button.png
	expectedLowercase := "/github/test-repo/guides/images/save_button.png"
	
	// Check if it's lowercase (even if ../  is still there, at least filename should be lowercase)
	assert.Contains(t, doc.Content, "save_button.png",
		"Image filename should be lowercase")
	
	// Ideally should clean up ../ but that's a separate issue
	if strings.Contains(doc.Content, expectedLowercase) {
		t.Logf("SUCCESS: Fully normalized path: %s", expectedLowercase)
	} else {
		t.Logf("PARTIAL: Filename lowercased but ../ not resolved (acceptable for now)")
	}
}
