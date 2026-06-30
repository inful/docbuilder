package docmodel

import (
	"path/filepath"
	"slices"
	"strings"
)

// Markdown extension set, evaluated case-insensitively. We include the
// four extensions the discovery pipeline has historically accepted so
// a single helper can replace the three separate predicates that lived
// in discovery, lint, and the pipeline. StripMarkdownExt only acts on
// the .md / .markdown pair (the two real-world extensions); the
// rarer .mdown / .mkd forms are valid as input but never written by us.
var markdownExtensions = []string{".md", ".markdown", ".mdown", ".mkd"}

// strippableMarkdownExtensions are the extensions StripMarkdownExt
// recognizes when removing a markdown suffix. The discovery-only
// extensions .mdown / .mkd are excluded because callers that strip
// them today do not exist; if one is added later, add it here.
var strippableMarkdownExtensions = []string{".md", ".markdown"}

// IsMarkdownFile reports whether path ends with a known markdown
// extension (case-insensitive). The check is based on the lower-cased
// extension only — the rest of the path is left alone, so callers can
// pass relative paths, absolute paths, or paths with query/fragment
// markers.
//
// This consolidates:
//   - docs.isMarkdownFile (case-insensitive, 4 extensions)
//   - lint.IsDocFile (was case-sensitive, 2 extensions)
//   - pipeline/transform_fingerprint inline check (case-insensitive, 1 extension)
//
// Lint now agrees with discovery on every case-sensitive input. Files
// such as README.MD on case-insensitive filesystems (macOS, Windows
// default) are no longer dropped on the floor by lint.
//
// Implementation note: filepath.Ext is used directly rather than a
// custom URL-fragment-stripping helper. Filenames like "tutorial#1.md"
// embed a literal '#' that we must NOT confuse with a URL fragment
// marker; filepath.Ext returns ".md" for both that filename and
// "tutorial#1.md#section", giving consistent results across the two.
func IsMarkdownFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return slices.Contains(markdownExtensions, ext)
}

// StripMarkdownExt removes a trailing .md / .markdown extension from
// s if present. The case of the rest of the string is preserved: only
// the suffix case check is lower-cased. Returns s unchanged when no
// known extension is present.
//
// Consolidates:
//   - lint/link_target_rewrite.stripKnownMarkdownExtension
//   - pipeline/transform_links inline strip
//
// Both prior implementations were case-insensitive on the suffix and
// preserved the input casing elsewhere; this preserves that contract.
func StripMarkdownExt(s string) string {
	lower := strings.ToLower(s)
	for _, ext := range strippableMarkdownExtensions {
		if strings.HasSuffix(lower, ext) {
			return s[:len(s)-len(ext)]
		}
	}
	return s
}
