package pipeline

import (
	"log/slog"
	"strings"

	"git.home.luguber.info/inful/docbuilder/internal/docmodel"
	"git.home.luguber.info/inful/docbuilder/internal/frontmatterops"
)

// fingerprintContent generates a stable content fingerprint and adds it to the frontmatter.
// It also ensures that any 'uid' field is preserved if the fingerprinting process changes the frontmatter.
//
// This transform operates on the serialized doc.Raw and should be run after serializeDocument.
func fingerprintContent(doc *Document) ([]*Document, error) {
	if !strings.HasSuffix(strings.ToLower(doc.Path), ".md") {
		return nil, nil
	}

	parsed, err := docmodel.Parse(doc.Raw, docmodel.Options{})
	if err != nil {
		slog.Error("Failed to generate content fingerprint",
			slog.String("path", doc.Path),
			slog.Any("error", err))
		// We don't fail the build for fingerprinting errors, we just log it.
		return nil, nil
	}

	var fields map[string]any
	if parsed.HadFrontmatter() {
		fields, err = parsed.FrontmatterFields()
		if err != nil {
			slog.Error("Failed to parse frontmatter for fingerprinting",
				slog.String("path", doc.Path),
				slog.Any("error", err))
			return nil, nil
		}
	} else {
		fields = map[string]any{}
	}

	computed, err := frontmatterops.ComputeFingerprint(fields, parsed.Body())
	if err != nil {
		slog.Error("Failed to compute fingerprint",
			slog.String("path", doc.Path),
			slog.Any("error", err))
		return nil, nil
	}
	if existing, ok := fields["fingerprint"].(string); ok && existing == computed {
		return nil, nil
	}

	fields["fingerprint"] = computed

	out, err := frontmatterops.Write(fields, parsed.Body(), true, parsed.Style())
	if err != nil {
		slog.Error("Failed to write frontmatter for fingerprinting",
			slog.String("path", doc.Path),
			slog.Any("error", err))
		return nil, nil
	}

	doc.Raw = out
	return nil, nil
}
