package docmodel

import (
	"os"

	"git.home.luguber.info/inful/docbuilder/internal/foundation/errors"
	"git.home.luguber.info/inful/docbuilder/internal/frontmatter"
)

// Options controls parsing behavior for ParsedDoc.
//
// It is intentionally small to keep the initial API focused; it will be expanded
// in later ADR-015 steps (e.g. lazy frontmatter fields, links, AST).
type Options struct{}

// ParsedDoc represents a Markdown document split into YAML frontmatter and body.
//
// This model centralizes the split/join workflow so that callers don’t re-implement
// boundary handling and style capture.
type ParsedDoc struct {
	original []byte
	fmRaw    []byte
	body     []byte
	hadFM    bool
	style    frontmatter.Style
}

// Parse parses raw file content into a ParsedDoc.
func Parse(content []byte, _ Options) (*ParsedDoc, error) {
	fmRaw, body, had, style, err := frontmatter.Split(content)
	if err != nil {
		return nil, errors.WrapError(err, errors.CategoryValidation, "failed to split frontmatter").Build()
	}

	orig := append([]byte(nil), content...)
	bodyCopy := append([]byte(nil), body...)
	var fmCopy []byte
	if had {
		fmCopy = make([]byte, len(fmRaw))
		copy(fmCopy, fmRaw)
	}

	return &ParsedDoc{
		original: orig,
		fmRaw:    fmCopy,
		body:     bodyCopy,
		hadFM:    had,
		style:    style,
	}, nil
}

// ParseFile reads a file from disk and parses it into a ParsedDoc.
func ParseFile(path string, opts Options) (*ParsedDoc, error) {
	// #nosec G304 -- path is provided by internal callers (discovery pipelines).
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.WrapError(err, errors.CategoryFileSystem, "failed to read document").
			WithContext("path", path).
			Build()
	}

	doc, err := Parse(content, opts)
	if err != nil {
		classified, ok := errors.AsClassified(err)
		if ok {
			return nil, errors.WrapError(classified, classified.Category(), "failed to parse document").
				WithContext("path", path).
				Build()
		}
		return nil, errors.WrapError(err, errors.CategoryValidation, "failed to parse document").
			WithContext("path", path).
			Build()
	}
	return doc, nil
}

// Original returns a copy of the original bytes.
func (d *ParsedDoc) Original() []byte {
	return append([]byte(nil), d.original...)
}

// HadFrontmatter reports whether the original document contained a YAML frontmatter block.
func (d *ParsedDoc) HadFrontmatter() bool {
	return d.hadFM
}

// FrontmatterRaw returns the raw YAML frontmatter bytes (without delimiters).
//
// If the document had no frontmatter, FrontmatterRaw returns nil.
func (d *ParsedDoc) FrontmatterRaw() []byte {
	if !d.hadFM {
		return nil
	}
	out := make([]byte, len(d.fmRaw))
	copy(out, d.fmRaw)
	return out
}

// Body returns the Markdown body bytes (frontmatter removed).
func (d *ParsedDoc) Body() []byte {
	out := make([]byte, len(d.body))
	copy(out, d.body)
	return out
}

// Style returns the detected formatting style from frontmatter splitting.
func (d *ParsedDoc) Style() frontmatter.Style {
	return d.style
}

// Bytes re-joins frontmatter and body into full document bytes.
func (d *ParsedDoc) Bytes() []byte {
	fm := d.fmRaw
	if !d.hadFM {
		fm = nil
	}
	// frontmatter.Join returns body as-is when had is false.
	return frontmatter.Join(fm, d.body, d.hadFM, d.style)
}
