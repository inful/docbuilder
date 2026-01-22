package frontmatterops

import "git.home.luguber.info/inful/docbuilder/internal/frontmatter"

// Read splits a markdown document into YAML frontmatter fields and body.
//
// Contract:
// - If the input doesn't start with a frontmatter delimiter, had=false and body is the full input.
// - If the input starts with a delimiter but is missing the closing delimiter, returns ErrMissingClosingDelimiter.
// - If frontmatter is present but empty, fields is an empty map.
func Read(content []byte) (fields map[string]any, body []byte, had bool, style frontmatter.Style, err error) {
	raw, body, had, style, err := frontmatter.Split(content)
	if err != nil {
		return nil, nil, false, style, err
	}

	fields, err = frontmatter.ParseYAML(raw)
	if err != nil {
		return nil, nil, had, style, err
	}

	return fields, body, had, style, nil
}

// Write serializes YAML frontmatter fields and joins with body.
//
// If had is false, Write returns body as-is (even if fields is non-empty).
func Write(fields map[string]any, body []byte, had bool, style frontmatter.Style) ([]byte, error) {
	if !had {
		return body, nil
	}

	raw, err := frontmatter.SerializeYAML(fields, style)
	if err != nil {
		return nil, err
	}

	return frontmatter.Join(raw, body, true, style), nil
}
