package lint

import "strings"

func extractFingerprintFromFrontmatter(content string) (string, bool) {
	return extractScalarFrontmatterField(content, "fingerprint")
}

func extractLastmodFromFrontmatter(content string) (string, bool) {
	return extractScalarFrontmatterField(content, "lastmod")
}

func extractScalarFrontmatterField(content, field string) (string, bool) {
	if !strings.HasPrefix(content, "---\n") {
		return "", false
	}
	endIdx := strings.Index(content[4:], "\n---\n")
	if endIdx == -1 {
		return "", false
	}
	frontmatter := content[4 : endIdx+4]
	prefix := field + ":"
	for line := range strings.SplitSeq(frontmatter, "\n") {
		trim := strings.TrimSpace(line)
		after, ok := strings.CutPrefix(trim, prefix)
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

func setOrUpdateLastmodInFrontmatter(content, lastmod string) string {
	if strings.TrimSpace(lastmod) == "" {
		return content
	}
	if !strings.HasPrefix(content, "---\n") {
		return content
	}
	endIdx := strings.Index(content[4:], "\n---\n")
	if endIdx == -1 {
		return content
	}

	frontmatter := content[4 : endIdx+4]
	body := content[endIdx+9:]

	lines := make([]string, 0, 8)
	for line := range strings.SplitSeq(frontmatter, "\n") {
		lines = append(lines, line)
	}

	hasLastmod := false
	kept := make([]string, 0, len(lines)+1)
	for _, line := range lines {
		trim := strings.TrimSpace(line)
		if _, ok := strings.CutPrefix(trim, "lastmod:"); ok {
			kept = append(kept, "lastmod: "+lastmod)
			hasLastmod = true
			continue
		}
		kept = append(kept, line)
	}

	if !hasLastmod {
		out := make([]string, 0, len(kept)+1)
		inserted := false
		for _, line := range kept {
			out = append(out, line)
			if !inserted && strings.HasPrefix(strings.TrimSpace(line), "fingerprint:") {
				out = append(out, "lastmod: "+lastmod)
				inserted = true
			}
		}
		if !inserted {
			out = append(out, "lastmod: "+lastmod)
		}
		kept = out
	}

	newFM := strings.TrimSpace(strings.Join(kept, "\n"))
	if newFM == "" {
		newFM = "lastmod: " + lastmod
	}
	return "---\n" + newFM + "\n---\n" + body
}
