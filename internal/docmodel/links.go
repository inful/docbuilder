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
	searchStartLineByNeedle := make(map[string]int)

	for _, link := range links {
		dest := link.Destination
		needleKey := string(link.Kind) + "\x00" + dest

		bodyLine := d.FindNextLineContaining(dest, searchStartLineByNeedle[needleKey])
		searchStartLineByNeedle[needleKey] = bodyLine + 1

		refs = append(refs, LinkRef{
			Link:     link,
			BodyLine: bodyLine,
			FileLine: d.LineOffset() + bodyLine,
		})
	}

	return refs, nil
}
