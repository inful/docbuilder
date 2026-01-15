package lint

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractScalarFrontmatterField(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		field     string
		wantValue string
		wantOK    bool
	}{
		{
			name: "extracts fingerprint from valid frontmatter",
			content: `---
fingerprint: abc123
title: Test
---
Body content`,
			field:     "fingerprint",
			wantValue: "abc123",
			wantOK:    true,
		},
		{
			name: "extracts lastmod from valid frontmatter",
			content: `---
title: Test
lastmod: 2026-01-15
---
Body content`,
			field:     "lastmod",
			wantValue: "2026-01-15",
			wantOK:    true,
		},
		{
			name: "extracts field with spaces around value",
			content: `---
lastmod:    2026-01-15   
---
Body`,
			field:     "lastmod",
			wantValue: "2026-01-15",
			wantOK:    true,
		},
		{
			name: "extracts field with indented value",
			content: `---
  lastmod: 2026-01-15
---
Body`,
			field:     "lastmod",
			wantValue: "2026-01-15",
			wantOK:    true,
		},
		{
			name: "returns false for missing field",
			content: `---
title: Test
---
Body`,
			field:     "lastmod",
			wantValue: "",
			wantOK:    false,
		},
		{
			name: "returns false for empty field value",
			content: `---
lastmod:
title: Test
---
Body`,
			field:     "lastmod",
			wantValue: "",
			wantOK:    false,
		},
		{
			name: "returns false for whitespace-only field value",
			content: `---
lastmod:   
title: Test
---
Body`,
			field:     "lastmod",
			wantValue: "",
			wantOK:    false,
		},
		{
			name:      "returns false for content without frontmatter",
			content:   "# Title\n\nBody content",
			field:     "lastmod",
			wantValue: "",
			wantOK:    false,
		},
		{
			name: "returns false for incomplete frontmatter (missing closing delimiter)",
			content: `---
title: Test
lastmod: 2026-01-15
Body content`,
			field:     "lastmod",
			wantValue: "",
			wantOK:    false,
		},
		{
			name:      "returns false for empty content",
			content:   "",
			field:     "lastmod",
			wantValue: "",
			wantOK:    false,
		},
		{
			name: "handles field with quoted value",
			content: `---
lastmod: "2026-01-15"
---
Body`,
			field:     "lastmod",
			wantValue: `"2026-01-15"`,
			wantOK:    true,
		},
		{
			name: "handles field with single-quoted value",
			content: `---
lastmod: '2026-01-15'
---
Body`,
			field:     "lastmod",
			wantValue: `'2026-01-15'`,
			wantOK:    true,
		},
		{
			name: "handles multiple fields, extracts correct one",
			content: `---
title: Test
author: John
lastmod: 2026-01-15
fingerprint: abc123
---
Body`,
			field:     "lastmod",
			wantValue: "2026-01-15",
			wantOK:    true,
		},
		{
			name: "does not match partial field name",
			content: `---
notlastmod: 2026-01-15
---
Body`,
			field:     "lastmod",
			wantValue: "",
			wantOK:    false,
		},
		{
			name: "handles field name as substring of another field",
			content: `---
custom_lastmod: 2026-01-10
lastmod: 2026-01-15
---
Body`,
			field:     "lastmod",
			wantValue: "2026-01-15",
			wantOK:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotValue, gotOK := extractScalarFrontmatterField(tt.content, tt.field)
			assert.Equal(t, tt.wantOK, gotOK, "ok value mismatch")
			assert.Equal(t, tt.wantValue, gotValue, "extracted value mismatch")
		})
	}
}

func TestSetOrUpdateLastmodInFrontmatter(t *testing.T) {
	tests := []struct {
		name       string
		content    string
		lastmod    string
		wantOutput string
	}{
		{
			name: "adds lastmod after fingerprint when not present",
			content: `---
title: Test
fingerprint: abc123
---
Body content`,
			lastmod: "2026-01-15",
			wantOutput: `---
title: Test
fingerprint: abc123
lastmod: 2026-01-15
---
Body content`,
		},
		{
			name: "updates existing lastmod",
			content: `---
title: Test
lastmod: 2000-01-01
fingerprint: abc123
---
Body content`,
			lastmod: "2026-01-15",
			wantOutput: `---
title: Test
lastmod: 2026-01-15
fingerprint: abc123
---
Body content`,
		},
		{
			name: "adds lastmod at end when fingerprint not present",
			content: `---
title: Test
author: John
---
Body content`,
			lastmod: "2026-01-15",
			wantOutput: `---
title: Test
author: John
lastmod: 2026-01-15
---
Body content`,
		},
		{
			name: "handles lastmod with whitespace in original",
			content: `---
title: Test
lastmod:   2000-01-01  
---
Body`,
			lastmod: "2026-01-15",
			wantOutput: `---
title: Test
lastmod: 2026-01-15
---
Body`,
		},
		{
			name:    "returns unchanged when lastmod is empty",
			content: "---\ntitle: Test\n---\nBody",
			lastmod: "",
			wantOutput: `---
title: Test
---
Body`,
		},
		{
			name:    "returns unchanged when lastmod is whitespace only",
			content: "---\ntitle: Test\n---\nBody",
			lastmod: "   ",
			wantOutput: `---
title: Test
---
Body`,
		},
		{
			name:       "returns unchanged when no frontmatter",
			content:    "# Title\n\nBody content",
			lastmod:    "2026-01-15",
			wantOutput: "# Title\n\nBody content",
		},
		{
			name: "returns unchanged when incomplete frontmatter",
			content: `---
title: Test
Body content`,
			lastmod: "2026-01-15",
			wantOutput: `---
title: Test
Body content`,
		},
		{
			name: "returns unchanged for empty frontmatter (limitation)",
			content: `---
---
Body content`,
			lastmod: "2026-01-15",
			wantOutput: `---
---
Body content`,
		},
		{
			name: "preserves indentation in other fields",
			content: `---
title: Test
  nested: value
fingerprint: abc123
---
Body`,
			lastmod: "2026-01-15",
			wantOutput: `---
title: Test
  nested: value
fingerprint: abc123
lastmod: 2026-01-15
---
Body`,
		},
		{
			name: "handles multiline body content",
			content: `---
title: Test
fingerprint: abc123
---
# Heading

Paragraph 1

Paragraph 2`,
			lastmod: "2026-01-15",
			wantOutput: `---
title: Test
fingerprint: abc123
lastmod: 2026-01-15
---
# Heading

Paragraph 1

Paragraph 2`,
		},
		{
			name: "handles body with frontmatter-like content",
			content: `---
title: Test
fingerprint: abc123
---
Body with ---
in the middle`,
			lastmod: "2026-01-15",
			wantOutput: `---
title: Test
fingerprint: abc123
lastmod: 2026-01-15
---
Body with ---
in the middle`,
		},
		{
			name: "inserts lastmod after fingerprint with multiple fields before",
			content: `---
title: Test
author: John
date: 2025-01-01
fingerprint: abc123
tags: [test]
---
Body`,
			lastmod: "2026-01-15",
			wantOutput: `---
title: Test
author: John
date: 2025-01-01
fingerprint: abc123
lastmod: 2026-01-15
tags: [test]
---
Body`,
		},
		{
			name: "updates lastmod when it appears before fingerprint",
			content: `---
title: Test
lastmod: 2000-01-01
fingerprint: abc123
---
Body`,
			lastmod: "2026-01-15",
			wantOutput: `---
title: Test
lastmod: 2026-01-15
fingerprint: abc123
---
Body`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := setOrUpdateLastmodInFrontmatter(tt.content, tt.lastmod)
			require.Equal(t, tt.wantOutput, got)
		})
	}
}

func TestExtractFingerprintFromFrontmatter(t *testing.T) {
	content := `---
fingerprint: abc123
title: Test
---
Body`
	val, ok := extractFingerprintFromFrontmatter(content)
	assert.True(t, ok)
	assert.Equal(t, "abc123", val)
}

func TestExtractLastmodFromFrontmatter(t *testing.T) {
	content := `---
title: Test
lastmod: 2026-01-15
---
Body`
	val, ok := extractLastmodFromFrontmatter(content)
	assert.True(t, ok)
	assert.Equal(t, "2026-01-15", val)
}
