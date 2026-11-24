package pipeline

import (
	"strings"

	"git.home.luguber.info/inful/docbuilder/internal/markdown"
)

// escapeShortcodesInCodeBlocks escapes Hugo shortcodes within markdown code blocks
// to prevent Hugo from trying to process them as actual shortcodes.
// This is essential for documentation that explains or demonstrates shortcode usage.
//
// The code blocks are the fenced ones and the indented ones: a shortcode
// in them is an example. The rest of the document is left exactly as it
// was - a shortcode outside the code is meant to run.
func escapeShortcodesInCodeBlocks(doc *Document) ([]*Document, error) {
	// The inline code spans count as code as well: Hugo processes a
	// shortcode before the markdown renders it, so a shortcode in an
	// inline span takes the whole site build down.
	regions := markdown.CodeRegions([]byte(doc.Content))
	doc.Content = markdown.ApplyToCode(doc.Content, regions, escapeShortcodes)

	// This transform only modifies content, doesn't generate new documents
	return nil, nil
}

// escapeShortcodes escapes the shortcodes of a piece of code: every
// {{< ... >}} becomes {{</* ... */>}}. The shortcodes that are escaped
// already are left alone, and every one of them is escaped on its own: a
// line that holds an escaped shortcode and another one holds both.
//
// A ">}}" that closes no shortcode is left alone: it is not a shortcode,
// and escaping it would put the escape marker into the example.
func escapeShortcodes(code string) string {
	if !strings.Contains(code, "{{<") && !strings.Contains(code, "{{%") {
		return code
	}

	var out strings.Builder
	out.Grow(len(code))

	// pending says that an opening was escaped and its closing is still
	// to come.
	pending := false

	for i := 0; i < len(code); {
		switch {
		case strings.HasPrefix(code[i:], "{{</*"):
			// Escaped already.
			out.WriteString("{{</*")
			i += len("{{</*")
		case strings.HasPrefix(code[i:], "{{<"):
			out.WriteString("{{</*")
			i += len("{{<")
			pending = true
		case strings.HasPrefix(code[i:], "{{%"):
			// The other form of a shortcode: Hugo reads it as one
			// too.
			out.WriteString("{{</*")
			i += len("{{%")
			pending = true
		case strings.HasPrefix(code[i:], "*/>}}"):
			// Escaped already.
			out.WriteString("*/>}}")
			i += len("*/>}}")
			pending = false
		case pending && strings.HasPrefix(code[i:], ">}}"):
			out.WriteString("*/>}}")
			i += len(">}}")
			pending = false
		case pending && strings.HasPrefix(code[i:], "%}}"):
			out.WriteString("*/>}}")
			i += len("%}}")
			pending = false
		default:
			out.WriteByte(code[i])
			i++
		}
	}

	return out.String()
}
