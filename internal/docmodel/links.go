package docmodel

import (
	"git.home.luguber.info/inful/docbuilder/internal/foundation/errors"
	"git.home.luguber.info/inful/docbuilder/internal/markdown"
)

type LinkRef struct {
	Link     markdown.Link
	BodyLine int
	FileLine int
}

func (d *ParsedDoc) Links() ([]markdown.Link, error) {
	d.linksOnce.Do(func() {
		links, err := markdown.ExtractLinks(d.body, markdown.Options{})
		if err != nil {
			d.linksErr = errors.WrapError(err, errors.CategoryValidation, "failed to extract markdown links").Build()
			return
		}
		d.links = links
	})

	if d.linksErr != nil {
		return nil, d.linksErr
	}

	out := make([]markdown.Link, len(d.links))
	copy(out, d.links)
	return out, nil
}

func (d *ParsedDoc) LinkRefs() ([]LinkRef, error) {
	links, err := d.Links()
	if err != nil {
		return nil, err
	}

	refs := make([]LinkRef, 0, len(links))
	// The state of every needle: the line of the last link with that
	// needle and how many of them are on that line. Two links with the
	// same target can be on the same line, and the second one is on the
	// line of the first one.
	type needleState struct {
		line int
		skip int
	}
	stateByNeedle := make(map[string]needleState)

	for _, link := range links {
		dest := link.Destination
		needleKey := string(link.Kind) + "\x00" + dest

		state := stateByNeedle[needleKey]
		startLine := max(state.line, 1)
		bodyLine := d.FindLinkLine(dest, startLine, state.skip)
		if bodyLine == state.line {
			state.skip++
		} else {
			state.line = bodyLine
			state.skip = 1
		}
		stateByNeedle[needleKey] = state

		refs = append(refs, LinkRef{
			Link:     link,
			BodyLine: bodyLine,
			FileLine: d.LineOffset() + bodyLine,
		})
	}

	return refs, nil
}
