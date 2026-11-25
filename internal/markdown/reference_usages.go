package markdown

import (
	"regexp"

	"github.com/yuin/goldmark/util"
)

// referenceUsageRegex finds the usage of a reference-style link:
// "[text][label]".
var referenceUsageRegex = regexp.MustCompile(`\[[^\]\[]*\]\[([^\]\[]*)\]`)

// extractUndefinedReferences returns the usages of reference-style links
// whose label is not defined in the document.
//
// The parser leaves the usage of an undefined label as plain text: the
// page shows the text of the link, the navigation is dead, and nothing
// reports it.
func extractUndefinedReferences(body []byte, defined map[string]struct{}) []Link {
	links := make([]Link, 0)

	// The code of the document holds examples, not links.
	regions := CodeBlockRegions(body)
	ApplyOutsideCode(string(body), regions, func(part string) string {
		// The examples of the inline code are not links either.
		for _, match := range referenceUsageRegex.FindAllStringSubmatch(stripInlineCodeSpans(part), -1) {
			label := match[1]
			if label == "" {
				// The collapsed form "[text][]" holds no label of its
				// own: the text of the link is the label.
				continue
			}
			if _, ok := defined[util.ToLinkReference([]byte(label))]; ok {
				continue
			}
			links = append(links, Link{Kind: LinkKindReference, Destination: label})
		}
		return part
	})

	return links
}
