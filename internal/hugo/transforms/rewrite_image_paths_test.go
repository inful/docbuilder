package transforms

import (
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/docs"
)

func TestRewriteImagePaths(t *testing.T) {
	tests := []struct {
		name           string
		fileName       string
		content        string
		expectedOutput string
	}{
		{
			name:     "rewrite relative image path in regular file",
			fileName: "gitlab_guide",
			content:  "![alt text](./images/screenshot.png)",
			expectedOutput: "![alt text](../images/screenshot.png)",
		},
		{
			name:     "multiple images in content",
			fileName: "guide",
			content: `# Guide
![image1](./images/img1.png)
Some text
![image2](./images/img2.jpg)`,
			expectedOutput: `# Guide
![image1](../images/img1.png)
Some text
![image2](../images/img2.jpg)`,
		},
		{
			name:     "index file should not rewrite paths",
			fileName: "index",
			content:  "![alt](./images/screenshot.png)",
			expectedOutput: "![alt](./images/screenshot.png)",
		},
		{
			name:     "README file should not rewrite paths",
			fileName: "README",
			content:  "![alt](./images/screenshot.png)",
			expectedOutput: "![alt](./images/screenshot.png)",
		},
		{
			name:     "already relative path with ../ stays as is",
			fileName: "guide",
			content:  "![alt](../images/screenshot.png)",
			expectedOutput: "![alt](../images/screenshot.png)",
		},
		{
			name:     "markdown link should not be changed",
			fileName: "guide",
			content:  "[link text](./other-page.md)",
			expectedOutput: "[link text](./other-page.md)",
		},
		{
			name:     "mixed content - only images changed",
			fileName: "guide",
			content: `[link](./page.md)
![image](./images/screenshot.png)
[another link](../other.md)`,
			expectedOutput: `[link](./page.md)
![image](../images/screenshot.png)
[another link](../other.md)`,
		},
		{
			name:     "various image formats",
			fileName: "guide",
			content: `![png](./img/test.png)
![jpg](./img/test.jpg)
![gif](./img/test.gif)
![svg](./img/test.svg)`,
			expectedOutput: `![png](../img/test.png)
![jpg](../img/test.jpg)
![gif](../img/test.gif)
![svg](../img/test.svg)`,
		},
		{
			name:     "image with complex alt text",
			fileName: "guide",
			content:  "![Picture showing where to find Code and Branches in the left side menu.](./images/1_1_selecting_branches.png)",
			expectedOutput: "![Picture showing where to find Code and Branches in the left side menu.](../images/1_1_selecting_branches.png)",
		},
		{
			name:     "no images to rewrite",
			fileName: "guide",
			content:  "Just some plain text without any images",
			expectedOutput: "Just some plain text without any images",
		},
		{
			name:     "mixed case filename should be lowercased",
			fileName: "guide",
			content:  "![alt text](./images/5_2_MR_ready.png)",
			expectedOutput: "![alt text](../images/5_2_mr_ready.png)",
		},
		{
			name:     "multiple mixed case filenames",
			fileName: "guide",
			content: `![img1](./images/File_A_B.png)
![img2](./images/Another_MR_Screenshot.png)`,
			expectedOutput: `![img1](../images/file_a_b.png)
![img2](../images/another_mr_screenshot.png)`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shim := &PageShim{
				Doc: docs.DocFile{
					Name: tt.fileName,
				},
				Content: tt.content,
			}

			transform := rewriteImagePathsTransform{}
			err := transform.Transform(shim)
			if err != nil {
				t.Fatalf("transform failed: %v", err)
			}

			if shim.Content != tt.expectedOutput {
				t.Errorf("Expected:\n%s\n\nGot:\n%s", tt.expectedOutput, shim.Content)
			}
		})
	}
}
