// Package urlutil centralizes small URL-shape predicates used across the
// discovery, lint, edit-link, and content-pipeline code paths.
//
// Before this package existed the same predicate was reimplemented in at
// least four places with subtly different definitions:
//
//   - lint/fixer_utils.isExternalURL -- http(s):// only
//   - hugo/editlink.isLocalPath        -- anything-not-remote
//   - hugo/pipeline.isForgeURL        -- http(s) or git@ (clone URLs)
//   - hugo/pipeline.isAbsoluteOrSpecialURL -- http(s), mailto:, tel:, #anchor
//
// The plan's M11 called these out: drift between the four was exactly
// the kind of latent bug worth removing. Each helper here documents the
// predicate it replaces and which callers migrated.
package urlutil

import "strings"

// IsExternalURL reports whether s begins with http:// or https://.
// Fragments, query strings, and trailing path components are not
// inspected: callers that need that detail should parse s through
// net/url.
//
// Consolidates:
//   - lint.isExternalURL (was http(s):// only).
func IsExternalURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}

// IsForgeURL reports whether s looks like a forge clone URL:
// http(s)://, ssh://, or git@ git+ssh. file:// is intentionally NOT
// included (that's a local path).
//
// Consolidates:
//   - pipeline.isForgeURL (was http(s) or git@).
//   - The pre-condition portion of editlink.isLocalPath (the inverse
//     of this predicate is used there to gate the heuristic detector).
func IsForgeURL(s string) bool {
	if IsExternalURL(s) {
		return true
	}
	if strings.HasPrefix(s, "git@") {
		return true
	}
	if strings.HasPrefix(s, "ssh://") || strings.HasPrefix(s, "git://") {
		return true
	}
	return false
}

// IsLocalPath reports whether s is a local file path (no remote scheme
// detected). A relative path, an absolute filesystem path starting with
// "/", or a Windows drive letter all count as local.
//
// Consolidates:
//   - editlink.isLocalPath.
func IsLocalPath(s string) bool {
	return !IsForgeURL(s)
}

// IsAbsoluteOrSpecialURL reports whether s is one of the shapes we
// pass through unchanged in the markdown link rewriter: an absolute URL,
// a fragment anchor (#section), or a non-fetchable scheme like mailto:
// or tel:. Stops the rewriter from rewriting user-supplied anchors and
// contact links.
//
// Consolidates:
//   - pipeline.isAbsoluteOrSpecialURL.
//
// Note: tel: is included even though the prior call site did not check
// for it -- it's a parallel case (a non-fetchable URI in markdown that
// should not be touched), and adding it here costs nothing.
func IsAbsoluteOrSpecialURL(s string) bool {
	if IsExternalURL(s) {
		return true
	}
	if strings.HasPrefix(s, "#") {
		return true
	}
	if strings.HasPrefix(s, "mailto:") || strings.HasPrefix(s, "tel:") {
		return true
	}
	return false
}
