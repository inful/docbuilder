package pipeline

import (
	"log/slog"
	"strings"

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

	original := string(doc.Raw)
	updated, err := mdfp.ProcessContent(original)
	if err != nil {
		slog.Error("Failed to generate content fingerprint",
			slog.String("path", doc.Path),
			slog.Any("error", err))
		// We don't fail the build for fingerprinting errors, we just log it
		return nil, nil
	}

	if original != updated {
		// Use preservation logic to ensure 'uid' isn't lost if it existed
		updated = preserveUIDAcrossFingerprintRewrite(original, updated)
		doc.Raw = []byte(updated)
	}

	return nil, nil
}

// preserveUIDAcrossFingerprintRewrite ensures the 'uid' field is kept if it was in the original frontmatter.
// Some frontmatter processors might drop unknown fields or reorder them in ways that drop information.
func preserveUIDAcrossFingerprintRewrite(original, updated string) string {
	uid, ok := extractUIDFromFrontmatter(original)
	if !ok {
		return updated
	}
	// Re-insert uid if it was lost.
	withUID, changed := addUIDIfMissingWithValue(updated, uid)
	if !changed {
		return updated
	}
	return withUID
}

func extractUIDFromFrontmatter(content string) (string, bool) {
	if !strings.HasPrefix(content, "---\n") {
		return "", false
	}
	endIdx := strings.Index(content[4:], "\n---\n")
	if endIdx == -1 {
		return "", false
	}
	frontmatter := content[4 : endIdx+4]
	for line := range strings.SplitSeq(frontmatter, "\n") {
		trim := strings.TrimSpace(line)
		after, ok := strings.CutPrefix(trim, "uid:")
		if !ok {
			continue
		}
		val := strings.TrimSpace(after)
		if val != "" {
			return val, true
		}
		return "", false
	}
	return "", false
}

func addUIDIfMissingWithValue(content, uid string) (string, bool) {
	if strings.TrimSpace(uid) == "" {
		return content, false
	}
	if !strings.HasPrefix(content, "---\n") {
		fm := "---\nuid: " + uid + "\n---\n\n"
		return fm + content, true
	}
	endIdx := strings.Index(content[4:], "\n---\n")
	if endIdx == -1 {
		return content, false
	}
	frontmatter := content[4 : endIdx+4]
	body := content[endIdx+9:]
	lines := strings.Split(frontmatter, "\n")

	for _, line := range lines {
		if _, ok := strings.CutPrefix(strings.TrimSpace(line), "uid:"); ok {
			return content, false
		}
	}

	kept := make([]string, 0, len(lines)+1)
	inserted := false
	for _, line := range lines {
		trim := strings.TrimSpace(line)
		kept = append(kept, line)
		if !inserted && strings.HasPrefix(trim, "fingerprint:") {
			kept = append(kept, "uid: "+uid)
			inserted = true
		}
	}
	if !inserted {
		out := make([]string, 0, len(kept)+1)
		added := false
		for _, line := range kept {
			trim := strings.TrimSpace(line)
			if !added && trim != "" {
				out = append(out, "uid: "+uid)
				added = true
			}
			out = append(out, line)
		}
		if !added {
			out = append(out, "uid: "+uid)
		}
		kept = out
	}
	newFM := strings.TrimSpace(strings.Join(kept, "\n"))
	if newFM == "" {
		newFM = "uid: " + uid
	}
	return "---\n" + newFM + "\n---\n" + body, true
}
