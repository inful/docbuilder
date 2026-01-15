package lint

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFrontmatterUIDRule_AppliesTo(t *testing.T) {
	rule := &FrontmatterUIDRule{}

	tests := []struct {
		name     string
		filePath string
		want     bool
	}{
		{
			name:     "applies to markdown file",
			filePath: "/path/to/document.md",
			want:     true,
		},
		{
			name:     "excludes _index.md (Unix path)",
			filePath: "/path/to/section/_index.md",
			want:     false,
		},
		{
			name:     "excludes _index.md (Windows path)",
			filePath: "C:\\path\\to\\section\\_index.md",
			want:     false,
		},
		{
			name:     "excludes _index.md at root",
			filePath: "_index.md",
			want:     false,
		},
		{
			name:     "applies to regular index.md",
			filePath: "/path/to/index.md",
			want:     true,
		},
		{
			name:     "excludes non-markdown files",
			filePath: "/path/to/image.png",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rule.AppliesTo(tt.filePath)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFrontmatterUIDRule_Check_MissingUID(t *testing.T) {
	rule := &FrontmatterUIDRule{}
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test.md")

	content := `---
title: "Test Document"
---

# Test
`
	err := os.WriteFile(filePath, []byte(content), 0o600)
	require.NoError(t, err)

	issues, err := rule.Check(filePath)
	require.NoError(t, err)
	require.Len(t, issues, 1)

	assert.Equal(t, SeverityError, issues[0].Severity)
	assert.Equal(t, frontmatterUIDRuleName, issues[0].Rule)
	assert.Contains(t, issues[0].Message, "Missing uid")
}

func TestFrontmatterUIDRule_Check_InvalidUIDFormat(t *testing.T) {
	rule := &FrontmatterUIDRule{}
	tempDir := t.TempDir()

	tests := []struct {
		name    string
		uid     string
		wantErr string
	}{
		{
			name:    "empty uid",
			uid:     `uid: ""`,
			wantErr: "uid is empty",
		},
		{
			name:    "whitespace only",
			uid:     "uid: \"  \"",
			wantErr: "uid is empty",
		},
		{
			name:    "invalid uuid format",
			uid:     "uid: not-a-uuid",
			wantErr: "uid must be a valid GUID/UUID",
		},
		{
			name:    "numeric uid",
			uid:     "uid: 12345",
			wantErr: "uid must be a string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filePath := filepath.Join(tempDir, tt.name+".md")
			content := "---\n" + tt.uid + "\n---\n\n# Test\n"
			err := os.WriteFile(filePath, []byte(content), 0o600)
			require.NoError(t, err)

			issues, err := rule.Check(filePath)
			require.NoError(t, err)
			require.Len(t, issues, 1)

			assert.Equal(t, SeverityError, issues[0].Severity)
			assert.Contains(t, issues[0].Explanation, tt.wantErr)
		})
	}
}

func TestFrontmatterUIDRule_Check_MissingAlias(t *testing.T) {
	rule := &FrontmatterUIDRule{}
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test.md")

	content := `---
uid: 550e8400-e29b-41d4-a716-446655440000
title: "Test Document"
---

# Test
`
	err := os.WriteFile(filePath, []byte(content), 0o600)
	require.NoError(t, err)

	issues, err := rule.Check(filePath)
	require.NoError(t, err)
	require.Len(t, issues, 1)

	assert.Equal(t, SeverityError, issues[0].Severity)
	assert.Equal(t, frontmatterUIDRuleName, issues[0].Rule)
	assert.Contains(t, issues[0].Message, "Missing uid-based alias")
	assert.Contains(t, issues[0].Explanation, "/_uid/550e8400-e29b-41d4-a716-446655440000/")
}

func TestFrontmatterUIDRule_Check_AliasAsString(t *testing.T) {
	rule := &FrontmatterUIDRule{}
	tempDir := t.TempDir()

	tests := []struct {
		name      string
		aliases   string
		shouldErr bool
	}{
		{
			name:      "correct alias as string",
			aliases:   "aliases: /_uid/550e8400-e29b-41d4-a716-446655440000/",
			shouldErr: false,
		},
		{
			name:      "wrong alias as string",
			aliases:   "aliases: /some/other/path/",
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filePath := filepath.Join(tempDir, tt.name+".md")
			content := `---
uid: 550e8400-e29b-41d4-a716-446655440000
` + tt.aliases + `
title: "Test Document"
---

# Test
`
			err := os.WriteFile(filePath, []byte(content), 0o600)
			require.NoError(t, err)

			issues, err := rule.Check(filePath)
			require.NoError(t, err)

			if tt.shouldErr {
				require.Len(t, issues, 1)
				assert.Contains(t, issues[0].Message, "Missing uid-based alias")
			} else {
				assert.Empty(t, issues)
			}
		})
	}
}

func TestFrontmatterUIDRule_Check_AliasAsArray(t *testing.T) {
	rule := &FrontmatterUIDRule{}
	tempDir := t.TempDir()

	tests := []struct {
		name      string
		content   string
		shouldErr bool
	}{
		{
			name: "correct alias in array",
			content: `---
uid: 550e8400-e29b-41d4-a716-446655440000
aliases:
  - /_uid/550e8400-e29b-41d4-a716-446655440000/
---

# Test
`,
			shouldErr: false,
		},
		{
			name: "correct alias with other aliases",
			content: `---
uid: 550e8400-e29b-41d4-a716-446655440000
aliases:
  - /old/path/
  - /_uid/550e8400-e29b-41d4-a716-446655440000/
  - /another/path/
---

# Test
`,
			shouldErr: false,
		},
		{
			name: "missing uid-based alias",
			content: `---
uid: 550e8400-e29b-41d4-a716-446655440000
aliases:
  - /old/path/
  - /another/path/
---

# Test
`,
			shouldErr: true,
		},
		{
			name: "empty aliases array",
			content: `---
uid: 550e8400-e29b-41d4-a716-446655440000
aliases: []
---

# Test
`,
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filePath := filepath.Join(tempDir, tt.name+".md")
			err := os.WriteFile(filePath, []byte(tt.content), 0o600)
			require.NoError(t, err)

			issues, err := rule.Check(filePath)
			require.NoError(t, err)

			if tt.shouldErr {
				require.Len(t, issues, 1)
				assert.Contains(t, issues[0].Message, "Missing uid-based alias")
			} else {
				assert.Empty(t, issues)
			}
		})
	}
}

func TestFrontmatterUIDRule_Check_ValidDocument(t *testing.T) {
	rule := &FrontmatterUIDRule{}
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test.md")

	content := `---
uid: 550e8400-e29b-41d4-a716-446655440000
title: "Test Document"
aliases:
  - /_uid/550e8400-e29b-41d4-a716-446655440000/
---

# Test Document

This is valid content.
`
	err := os.WriteFile(filePath, []byte(content), 0o600)
	require.NoError(t, err)

	issues, err := rule.Check(filePath)
	require.NoError(t, err)
	assert.Empty(t, issues)
}

func TestFrontmatterUIDRule_Check_NoFrontmatter(t *testing.T) {
	rule := &FrontmatterUIDRule{}
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test.md")

	content := `# Test Document

No frontmatter at all.
`
	err := os.WriteFile(filePath, []byte(content), 0o600)
	require.NoError(t, err)

	issues, err := rule.Check(filePath)
	require.NoError(t, err)
	require.Len(t, issues, 1)
	assert.Contains(t, issues[0].Message, "Missing uid")
}

func TestFrontmatterUIDRule_Check_MalformedFrontmatter(t *testing.T) {
	rule := &FrontmatterUIDRule{}
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test.md")

	content := `---
this is not valid yaml: [broken
---

# Test
`
	err := os.WriteFile(filePath, []byte(content), 0o600)
	require.NoError(t, err)

	issues, err := rule.Check(filePath)
	require.NoError(t, err)
	require.Len(t, issues, 1)
	assert.Contains(t, issues[0].Message, "Missing uid")
}
