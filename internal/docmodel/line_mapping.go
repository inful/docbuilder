package docmodel

import "strings"

// LineOffset returns the 1-based line offset to translate body line numbers
// into original file line numbers.
//
// If the document has YAML frontmatter, this accounts for:
// - opening delimiter line
// - all raw frontmatter lines
// - closing delimiter line
//
// The relationship is: fileLine = LineOffset() + bodyLine.
func (d *ParsedDoc) LineOffset() int {
	if !d.hadFM {
		return 0
	}

	// Keep parity with existing consumers: compute based on the raw frontmatter
	// bytes returned by frontmatter.Split.
	return 2 + strings.Count(string(d.fmRaw), "\n")
}

// FindNextLineContaining returns the next 1-based body line number that contains
// target, starting at startLine (1-based).
//
// It skips fenced code blocks (``` and ~~~), indented code blocks, and matches
// inside inline code spans.
func (d *ParsedDoc) FindNextLineContaining(target string, startLine int) int {
	body := string(d.body)
	if body == "" || target == "" {
		return 1
	}

	lines := strings.Split(body, "\n")
	skippable := computeSkippableLines(lines)

	if startLine < 1 {
		startLine = 1
	}
	if startLine > len(lines) {
		startLine = len(lines)
	}

	for i := startLine - 1; i < len(lines); i++ {
		if i >= 0 && i < len(skippable) && skippable[i] {
			continue
		}

		searchFrom := 0
		for {
			idx := strings.Index(lines[i][searchFrom:], target)
			if idx == -1 {
				break
			}
			idx = searchFrom + idx
			if !isInsideInlineCode(lines[i], idx) {
				return i + 1
			}
			searchFrom = idx + 1
			if searchFrom >= len(lines[i]) {
				break
			}
		}
	}

	return 1
}

// FindLinkLine returns the 1-based body line of the link that points at
// the given destination, starting at startLine (1-based).
//
// The link is found by its syntax - "](target)", "](<target>)" or
// "]: target" - and not by the text of the destination: a document that
// mentions a file in its prose and links to it would otherwise point at
// the prose, and the fixer would rewrite the words of the page.
//
// The skip says how many links with that destination are already known
// on the line startLine: two links with the same target can be on the
// same line, and the second one is on the line of the first one.
//
// It skips fenced code blocks (``` and ~~~), indented code blocks, and
// matches inside inline code spans.
func (d *ParsedDoc) FindLinkLine(destination string, startLine, skip int) int {
	body := string(d.body)
	if body == "" || destination == "" {
		return 1
	}

	lines := strings.Split(body, "\n")
	skippable := computeSkippableLines(lines)

	if startLine < 1 {
		startLine = 1
	}
	if startLine > len(lines) {
		startLine = len(lines)
	}

	needles := []string{
		"](" + destination,
		"](" + "<" + destination + ">",
		"]: " + destination,
	}

	for i := startLine - 1; i < len(lines); i++ {
		if i >= 0 && i < len(skippable) && skippable[i] {
			continue
		}
		skipHere := 0
		if i == startLine-1 {
			skipHere = skip
		}
		if countLinkMatches(lines[i], needles) > skipHere {
			return i + 1
		}
	}

	return 1
}

// countLinkMatches counts the links that point at the destination on one
// line.
func countLinkMatches(line string, needles []string) int {
	count := 0
	for _, needle := range needles {
		for offset := 0; ; {
			idx := strings.Index(line[offset:], needle)
			if idx == -1 {
				break
			}
			idx += offset
			if !isInsideInlineCode(line, idx) {
				count++
			}
			offset = idx + len(needle)
		}
	}
	return count
}

func computeSkippableLines(lines []string) []bool {
	skippable := make([]bool, len(lines))
	inCodeBlock := false
	activeFence := ""

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inCodeBlock, activeFence = toggleFencedBlock(inCodeBlock, activeFence, "```")
			skippable[i] = true
			continue
		}
		if strings.HasPrefix(trimmed, "~~~") {
			inCodeBlock, activeFence = toggleFencedBlock(inCodeBlock, activeFence, "~~~")
			skippable[i] = true
			continue
		}

		if inCodeBlock || strings.HasPrefix(line, "    ") || strings.HasPrefix(line, "\t") {
			skippable[i] = true
			continue
		}
	}

	return skippable
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

func isInsideInlineCode(line string, pos int) bool {
	backtickCount := 0
	for i := 0; i < pos && i < len(line); i++ {
		if line[i] == '`' {
			backtickCount++
		}
	}
	return backtickCount%2 == 1
}
