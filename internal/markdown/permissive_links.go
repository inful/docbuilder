package markdown

import "strings"

func extractPermissiveLinks(body []byte) []Link {
	lines := strings.Split(string(body), "\n")

	inCodeBlock := false
	activeFence := ""

	out := make([]Link, 0)
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inCodeBlock, activeFence = toggleFencedBlock(inCodeBlock, activeFence, "```")
			continue
		}
		if strings.HasPrefix(trimmed, "~~~") {
			inCodeBlock, activeFence = toggleFencedBlock(inCodeBlock, activeFence, "~~~")
			continue
		}
		if inCodeBlock || strings.HasPrefix(line, "    ") || strings.HasPrefix(line, "\t") {
			continue
		}

		clean := stripInlineCodeSpans(line)

		out = append(out, extractImageLinksPermissive(clean)...)
		out = append(out, extractInlineLinksPermissive(clean)...)
		out = append(out, extractReferenceDefinitionsPermissive(clean)...)
	}

	return out
}

func containsWhitespace(s string) bool {
	return strings.ContainsAny(s, " \t")
}

func toggleFencedBlock(inCodeBlock bool, activeFence string, fence string) (bool, string) {
	if !inCodeBlock {
		return true, fence
	}
	if activeFence == fence {
		return false, ""
	}
	return inCodeBlock, activeFence
}

func stripInlineCodeSpans(s string) string {
	if !strings.Contains(s, "`") {
		return s
	}

	var out strings.Builder
	out.Grow(len(s))

	for i := 0; i < len(s); {
		if s[i] != '`' {
			out.WriteByte(s[i])
			i++
			continue
		}

		run := 1
		for i+run < len(s) && s[i+run] == '`' {
			run++
		}

		marker := strings.Repeat("`", run)
		closeRel := strings.Index(s[i+run:], marker)
		if closeRel == -1 {
			// Unclosed code span; keep the backticks and continue.
			out.WriteString(marker)
			i += run
			continue
		}

		// Skip the entire code span, including delimiters.
		i = i + run + closeRel + run
	}

	return out.String()
}

type inlineLinkInfo struct {
	target string
}

func extractImageLinksPermissive(line string) []Link {
	links := make([]Link, 0)

	for i := 0; i+2 < len(line); i++ {
		if line[i] != '!' || line[i+1] != '[' {
			continue
		}

		info := extractImageLink(line, i)
		if info == nil {
			continue
		}

		if containsWhitespace(info.target) {
			links = append(links, Link{Kind: LinkKindImage, Destination: info.target})
		}
	}

	return links
}

func extractImageLink(line string, i int) *inlineLinkInfo {
	closeBracket := strings.Index(line[i+2:], "]")
	if closeBracket == -1 {
		return nil
	}
	closeBracket += i + 2

	if closeBracket+1 >= len(line) || line[closeBracket+1] != '(' {
		return nil
	}

	end := strings.Index(line[closeBracket+2:], ")")
	if end == -1 {
		return nil
	}
	end += closeBracket + 2

	linkTarget := line[closeBracket+2 : end]
	return &inlineLinkInfo{target: linkTarget}
}

func extractInlineLinksPermissive(line string) []Link {
	links := make([]Link, 0)

	for i := 0; i+1 < len(line); i++ {
		if line[i] != ']' || line[i+1] != '(' {
			continue
		}

		info := extractInlineLink(line, i)
		if info == nil {
			continue
		}

		if containsWhitespace(info.target) {
			links = append(links, Link{Kind: LinkKindInline, Destination: info.target})
		}
	}

	return links
}

func extractInlineLink(line string, closeBracketPos int) *inlineLinkInfo {
	start := findLinkTextStart(line, closeBracketPos)
	if start == -1 {
		return nil
	}

	end := findLinkEnd(line, closeBracketPos+2)
	if end == -1 {
		return nil
	}

	linkTarget := line[closeBracketPos+2 : end]
	return &inlineLinkInfo{target: linkTarget}
}

func findLinkTextStart(line string, closeBracketPos int) int {
	for j := closeBracketPos - 1; j >= 0; j-- {
		if line[j] == '[' {
			// Make sure it's not an image link (preceded by !)
			if j > 0 && line[j-1] == '!' {
				return -1
			}
			return j
		}
	}
	return -1
}

func findLinkEnd(line string, startPos int) int {
	end := strings.Index(line[startPos:], ")")
	if end == -1 {
		return -1
	}
	return startPos + end
}

func extractReferenceDefinitionsPermissive(line string) []Link {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "[") {
		return nil
	}

	label, after, ok := strings.Cut(trimmed, "]:")
	if !ok {
		return nil
	}

	// Footnote definitions look like: [^1]: ...
	// They are not Markdown reference link definitions and must not be treated as links.
	if strings.HasPrefix(strings.TrimSpace(label), "[^") {
		return nil
	}

	rest := strings.TrimSpace(after)
	if rest == "" {
		return nil
	}

	linkTarget := rest
	if before, _, ok := strings.Cut(rest, " \""); ok {
		linkTarget = before
	} else if before, _, ok := strings.Cut(rest, " '"); ok {
		linkTarget = before
	}

	linkTarget = strings.TrimSpace(linkTarget)
	if linkTarget == "" {
		return nil
	}

	if !containsWhitespace(linkTarget) {
		return nil
	}

	return []Link{{Kind: LinkKindReferenceDefinition, Destination: linkTarget}}
}
