package frontmatter

import (
	"bytes"
	"errors"

	"gopkg.in/yaml.v3"
)

// Style captures formatting details needed for stable rewriting.
//
// It intentionally focuses on newline/trailing newline shape and does not
// attempt to preserve original YAML formatting.
type Style struct {
	Newline            string
	HasTrailingNewline bool
}

// Split separates YAML frontmatter (`---` delimited) from the Markdown body.
//
// If the document does not start with a YAML frontmatter delimiter, had is false
// and body is the full input.
func Split(content []byte) (frontmatter []byte, body []byte, had bool, style Style, err error) {
	style = detectStyle(content)

	nl := style.Newline
	open := []byte("---" + nl)
	if !bytes.HasPrefix(content, open) {
		return nil, content, false, style, nil
	}

	frontmatterStart := len(open)
	closeLine := []byte("---" + nl)
	if bytes.HasPrefix(content[frontmatterStart:], closeLine) {
		bodyStart := frontmatterStart + len(closeLine)
		return []byte{}, content[bodyStart:], true, style, nil
	}

	closeSeq := []byte(nl + "---" + nl)
	idx := bytes.Index(content[frontmatterStart:], closeSeq)
	if idx < 0 {
		return nil, nil, false, style, ErrMissingClosingDelimiter
	}

	frontmatterEnd := frontmatterStart + idx + len(nl)
	bodyStart := frontmatterStart + idx + len(closeSeq)
	return content[frontmatterStart:frontmatterEnd], content[bodyStart:], true, style, nil
}

// Join reassembles a document from raw frontmatter and body.
//
// If had is false, Join returns body as-is.
// If had is true, Join emits YAML frontmatter using `---` delimiters and the
// newline style captured in Style.
func Join(frontmatter []byte, body []byte, had bool, style Style) []byte {
	if !had {
		return body
	}

	nl := style.Newline
	if nl == "" {
		nl = "\n"
	}

	open := []byte("---" + nl)
	closing := []byte("---" + nl)

	out := make([]byte, 0, len(open)+len(frontmatter)+len(closing)+len(body))
	out = append(out, open...)
	out = append(out, frontmatter...)
	out = append(out, closing...)
	out = append(out, body...)
	return out
}

// ParseYAML parses raw YAML frontmatter (without --- delimiters) into a map.
func ParseYAML(frontmatter []byte) (map[string]any, error) {
	if len(frontmatter) == 0 {
		return map[string]any{}, nil
	}

	var fields map[string]any
	if err := yaml.Unmarshal(frontmatter, &fields); err != nil {
		return nil, err
	}
	if fields == nil {
		fields = map[string]any{}
	}
	return fields, nil
}

// ErrMissingClosingDelimiter indicates the document started with a YAML
// frontmatter delimiter but did not contain a closing delimiter.
var ErrMissingClosingDelimiter = errors.New("yaml frontmatter start delimiter found but closing delimiter is missing")

func detectStyle(content []byte) Style {
	newline := "\n"
	for i := 0; i+1 < len(content); i++ {
		if content[i] == '\r' && content[i+1] == '\n' {
			newline = "\r\n"
			break
		}
		if content[i] == '\n' {
			newline = "\n"
			break
		}
	}

	hasTrailingNewline := len(content) > 0 && (content[len(content)-1] == '\n')

	return Style{
		Newline:            newline,
		HasTrailingNewline: hasTrailingNewline,
	}
}
