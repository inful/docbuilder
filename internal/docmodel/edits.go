package docmodel

import (
	"git.home.luguber.info/inful/docbuilder/internal/foundation/errors"
	"git.home.luguber.info/inful/docbuilder/internal/frontmatter"
	"git.home.luguber.info/inful/docbuilder/internal/markdown"
)

// ApplyBodyEdits applies byte-range edits to the document body and returns the
// full, re-joined document bytes.
//
// Frontmatter bytes are preserved exactly.
func (d *ParsedDoc) ApplyBodyEdits(edits []markdown.Edit) ([]byte, error) {
	updatedBody, err := markdown.ApplyEdits(d.body, edits)
	if err != nil {
		return nil, errors.WrapError(err, errors.CategoryValidation, "failed to apply body edits").Build()
	}

	out := frontmatter.Join(d.fmRaw, updatedBody, d.hadFM, d.style)
	return append([]byte(nil), out...), nil
}
