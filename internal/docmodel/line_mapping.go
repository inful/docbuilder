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
