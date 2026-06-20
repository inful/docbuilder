package lint

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"git.home.luguber.info/inful/docbuilder/internal/docmodel"
	"git.home.luguber.info/inful/docbuilder/internal/markdown"
)

// writeMarkdownFile is a test helper that creates parent directories and writes content.
func writeMarkdownFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o750))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
}

// linkRef builds a docmodel.LinkRef at the given line with the given kind/destination.
func linkRef(fileLine int, kind markdown.LinkKind, dest string) docmodel.LinkRef {
	return docmodel.LinkRef{
		Link:     markdown.Link{Kind: kind, Destination: dest},
		FileLine: fileLine,
	}
}

func TestInferRenameMappingFromGitHead(t *testing.T) {
	// Source markdown file with one inline link on line 1.
	// [link](old-name.md)
	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "src.md")
	writeFile(t, srcFile, "[link](old-name.md)\n")

	// Existing target file at the renamed path (fileExists() must return true).
	renamed := filepath.Join(tmpDir, "renamed.md")
	writeFile(t, renamed, "# Renamed\n")

	// Sanity: confirm the parser actually emits a LinkRef with FileLine=1.
	curDoc, err := docmodel.ParseFile(srcFile, docmodel.Options{})
	require.NoError(t, err)
	curRefs, err := curDoc.LinkRefs()
	require.NoError(t, err)
	require.Len(t, curRefs, 1, "fixture must contain exactly one link")
	require.Equal(t, 1, curRefs[0].FileLine, "fixture link must be on line 1")
	require.Equal(t, markdown.LinkKindInline, curRefs[0].Link.Kind)

	const oldAbs = "/abs/path/to/old-name.md"

	cases := []struct {
		name     string
		bl       BrokenLink
		oldAbs   string
		headRefs []docmodel.LinkRef
		wantOK   bool
	}{
		{
			name:   "empty SourceFile returns false",
			bl:     BrokenLink{Target: "old-name.md"},
			oldAbs: oldAbs,
		},
		{
			name:   "zero LineNumber returns false",
			bl:     BrokenLink{SourceFile: srcFile, Target: "old-name.md"},
			oldAbs: oldAbs,
		},
		{
			name:   "negative LineNumber returns false",
			bl:     BrokenLink{SourceFile: srcFile, LineNumber: -1, Target: "old-name.md"},
			oldAbs: oldAbs,
		},
		{
			name:   "SourceFile does not exist returns false",
			bl:     BrokenLink{SourceFile: filepath.Join(tmpDir, "missing.md"), LineNumber: 1, Target: "old-name.md"},
			oldAbs: oldAbs,
		},
		{
			name:     "no link at LineNumber returns false",
			bl:       BrokenLink{SourceFile: srcFile, LineNumber: 999, Target: "old-name.md"},
			oldAbs:   oldAbs,
			headRefs: []docmodel.LinkRef{},
		},
		{
			name:     "wrong destination at LineNumber returns false",
			bl:       BrokenLink{SourceFile: srcFile, LineNumber: 1, Target: "different.md"},
			oldAbs:   oldAbs,
			headRefs: []docmodel.LinkRef{linkRef(1, markdown.LinkKindInline, "old-name.md")},
		},
		{
			name:     "ambiguous: two matching refs at same line returns false",
			bl:       BrokenLink{SourceFile: srcFile, LineNumber: 1, Target: "old-name.md"},
			oldAbs:   oldAbs,
			headRefs: []docmodel.LinkRef{
				linkRef(1, markdown.LinkKindInline, "old-name.md"),
				linkRef(1, markdown.LinkKindInline, "old-name.md"),
			},
		},
		{
			name:     "different link kind in HEAD vs current returns false",
			bl:       BrokenLink{SourceFile: srcFile, LineNumber: 1, Target: "old-name.md"},
			oldAbs:   oldAbs,
			headRefs: []docmodel.LinkRef{linkRef(1, markdown.LinkKindImage, "renamed.md")},
		},
		{
			name:     "empty destination in HEAD returns false",
			bl:       BrokenLink{SourceFile: srcFile, LineNumber: 1, Target: "old-name.md"},
			oldAbs:   oldAbs,
			headRefs: []docmodel.LinkRef{linkRef(1, markdown.LinkKindInline, "   ")},
		},
		{
			name:     "Hugo shortcode target in HEAD returns false",
			bl:       BrokenLink{SourceFile: srcFile, LineNumber: 1, Target: "old-name.md"},
			oldAbs:   oldAbs,
			headRefs: []docmodel.LinkRef{linkRef(1, markdown.LinkKindInline, "{{< myshortcode >}}")},
		},
		{
			name:     "UID alias target in HEAD returns false",
			bl:       BrokenLink{SourceFile: srcFile, LineNumber: 1, Target: "old-name.md"},
			oldAbs:   oldAbs,
			headRefs: []docmodel.LinkRef{linkRef(1, markdown.LinkKindInline, "/_uid/some-doc")},
		},
		{
			name:     "https URL in HEAD returns false",
			bl:       BrokenLink{SourceFile: srcFile, LineNumber: 1, Target: "old-name.md"},
			oldAbs:   oldAbs,
			headRefs: []docmodel.LinkRef{linkRef(1, markdown.LinkKindInline, "https://example.com/x")},
		},
		{
			name:     "mailto link in HEAD returns false",
			bl:       BrokenLink{SourceFile: srcFile, LineNumber: 1, Target: "old-name.md"},
			oldAbs:   oldAbs,
			headRefs: []docmodel.LinkRef{linkRef(1, markdown.LinkKindInline, "mailto:foo@example.com")},
		},
		{
			name:     "anchor-only link in HEAD returns false",
			bl:       BrokenLink{SourceFile: srcFile, LineNumber: 1, Target: "old-name.md"},
			oldAbs:   oldAbs,
			headRefs: []docmodel.LinkRef{linkRef(1, markdown.LinkKindInline, "#section")},
		},
		{
			name:     "HEAD destination unchanged from broken target returns false",
			bl:       BrokenLink{SourceFile: srcFile, LineNumber: 1, Target: "old-name.md"},
			oldAbs:   oldAbs,
			headRefs: []docmodel.LinkRef{linkRef(1, markdown.LinkKindInline, "old-name.md")},
		},
		{
			name:     "HEAD destination points to non-existent file returns false",
			bl:       BrokenLink{SourceFile: srcFile, LineNumber: 1, Target: "old-name.md"},
			oldAbs:   oldAbs,
			headRefs: []docmodel.LinkRef{linkRef(1, markdown.LinkKindInline, "ghost.md")},
		},
		{
			name:     "curPos out of range in headRefs returns false",
			bl:       BrokenLink{SourceFile: srcFile, LineNumber: 1, Target: "old-name.md"},
			oldAbs:   oldAbs,
			headRefs: []docmodel.LinkRef{}, // empty: curPos=0 is out of range
		},
		{
			name:     "happy path: HEAD points to existing renamed file returns true",
			bl:       BrokenLink{SourceFile: srcFile, LineNumber: 1, Target: "old-name.md"},
			oldAbs:   oldAbs,
			headRefs: []docmodel.LinkRef{linkRef(1, markdown.LinkKindInline, "renamed.md")},
			wantOK:   true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cache := map[string][]docmodel.LinkRef{srcFile: tc.headRefs}
			rm, ok := inferRenameMappingFromGitHead(tc.bl, tc.oldAbs, cache)
			require.Equal(t, tc.wantOK, ok, "ok flag mismatch")
			if tc.wantOK {
				require.Equal(t, tc.oldAbs, rm.OldAbs)
				require.Equal(t, renamed, rm.NewAbs)
				require.Equal(t, RenameSourceGitHistory, rm.Source)
			}
		})
	}
}
