package pipeline

import (
	"log/slog"
	"strings"

	"git.home.luguber.info/inful/docbuilder/internal/frontmatter"
	"github.com/inful/mdfp"
)

// fingerprintContent generates a stable content fingerprint and adds it to the frontmatter.
// It also ensures that any 'uid' field is preserved if the fingerprinting process changes the frontmatter.
//
// This transform operates on the serialized doc.Raw and should be run after serializeDocument.
func fingerprintContent(doc *Document) ([]*Document, error) {
	if !strings.HasSuffix(strings.ToLower(doc.Path), ".md") {
		return nil, nil
	}

	fmRaw, body, had, _, err := frontmatter.Split(doc.Raw)
	if err != nil {
		slog.Error("Failed to generate content fingerprint",
			slog.String("path", doc.Path),
			slog.Any("error", err))
		// We don't fail the build for fingerprinting errors, we just log it.
		return nil, nil
	}

	var fields map[string]any
	if had {
		fields, err = frontmatter.ParseYAML(fmRaw)
		if err != nil {
			slog.Error("Failed to parse frontmatter for fingerprinting",
				slog.String("path", doc.Path),
				slog.Any("error", err))
			return nil, nil
		}
	} else {
		fields = map[string]any{}
	}

	// Compute fingerprint from the exact frontmatter shape we intend to write.
	// DocBuilder's lint/fix pipeline expects fingerprints to match this canonical form,
	// even if serialization reorders keys.
	fieldsForHash := deepCopyMap(fields)
	delete(fieldsForHash, "fingerprint")
	delete(fieldsForHash, "lastmod")
	delete(fieldsForHash, "uid")
	delete(fieldsForHash, "aliases")

	style := frontmatter.Style{Newline: "\n"}
	frontmatterForHash, err := frontmatter.SerializeYAML(fieldsForHash, style)
	if err != nil {
		slog.Error("Failed to serialize frontmatter for fingerprint hashing",
			slog.String("path", doc.Path),
			slog.Any("error", err))
		return nil, nil
	}

	fmForHash := trimSingleTrailingNewline(string(frontmatterForHash))
	computed := mdfp.CalculateFingerprintFromParts(fmForHash, string(body))
	if existing, ok := fields["fingerprint"].(string); ok && existing == computed {
		return nil, nil
	}

	fields["fingerprint"] = computed

	fmOut, err := frontmatter.SerializeYAML(fields, style)
	if err != nil {
		slog.Error("Failed to serialize frontmatter for fingerprinting",
			slog.String("path", doc.Path),
			slog.Any("error", err))
		return nil, nil
	}

	doc.Raw = frontmatter.Join(fmOut, body, true, style)
	return nil, nil
}

func trimSingleTrailingNewline(s string) string {
	if before, ok := strings.CutSuffix(s, "\r\n"); ok {
		return before
	}
	if before, ok := strings.CutSuffix(s, "\n"); ok {
		return before
	}
	return s
}
