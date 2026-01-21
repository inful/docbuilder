package markdown

// Options controls how Markdown is parsed for internal analysis.
//
// For now this is intentionally small; it exists so we can evolve parsing behavior
// (extensions/settings) without rewriting call sites.
type Options struct{}

type LinkKind string

const (
	LinkKindInline              LinkKind = "inline"
	LinkKindImage               LinkKind = "image"
	LinkKindAuto                LinkKind = "auto"
	LinkKindReference           LinkKind = "reference"
	LinkKindReferenceDefinition LinkKind = "reference_definition"
)

type Link struct {
	Kind        LinkKind
	Destination string
}
