package templates

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// maxSequenceFiles is the maximum number of files to scan when computing sequences.
// This prevents excessive filesystem operations on large directories.
const maxSequenceFiles = 10000

// ErrNoSequenceDefinition is returned when a sequence definition is missing or incomplete.
var ErrNoSequenceDefinition = errors.New("sequence definition missing")

// SequenceDefinition describes how to compute a sequential number for template output paths.
//
// Sequences are used to automatically number documents (e.g., ADR-001, ADR-002, ADR-003).
// The sequence is computed by scanning existing files in a directory and finding the
// highest number, then returning the next number.
type SequenceDefinition struct {
	// Name is the sequence identifier used in templates (e.g., "adr").
	Name string

	// Dir is the directory relative to docs/ to scan for existing files.
	// Must be relative (no ".." or absolute paths).
	Dir string

	// Glob is the filename pattern to match (e.g., "adr-*.md").
	Glob string

	// Regex is the pattern to extract the sequence number from filenames.
	// Must have exactly one capture group containing the number.
	// Example: "^adr-(\\d{3})-"
	Regex string

	// Width is the display width for padding (e.g., 3 for "001", "002").
	// Used by templates with printf formatting.
	Width int

	// Start is the starting number if no existing files are found.
	// If 0, defaults to 1.
	Start int
}

// ParseSequenceDefinition parses a sequence definition from JSON metadata.
//
// The JSON is extracted from the "docbuilder:template.sequence" meta tag.
//
// Parameters:
//   - raw: JSON string containing the sequence definition
//
// Returns:
//   - A parsed SequenceDefinition
//   - ErrNoSequenceDefinition if raw is empty
//   - An error if JSON is invalid or required fields are missing
//
// Example:
//
//	json := `{"name":"adr","dir":"adr","glob":"adr-*.md","regex":"^adr-(\\d{3})-","width":3,"start":1}`
//	def, err := ParseSequenceDefinition(json)
func ParseSequenceDefinition(raw string) (*SequenceDefinition, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, ErrNoSequenceDefinition
	}

	var def SequenceDefinition
	if err := json.Unmarshal([]byte(raw), &def); err != nil {
		return nil, fmt.Errorf("parse sequence definition: %w", err)
	}
	if def.Name == "" || def.Dir == "" || def.Glob == "" || def.Regex == "" {
		return nil, errors.New("sequence definition missing required fields")
	}
	return &def, nil
}

// ComputeNextInSequence computes the next number in a sequence by scanning existing files.
//
// The function:
//  1. Validates the sequence definition (dir must be relative, under docs/)
//  2. Compiles the regex (must have exactly one capture group)
//  3. Glob matches files in the target directory
//  4. Extracts numbers from matching filenames using the regex
//  5. Returns max + 1, or Start if no files found, or 1 if Start is 0
//
// Security: The function validates that dir is relative and under docsDir to prevent
// path traversal attacks. It also limits scanning to maxSequenceFiles files.
//
// Parameters:
//   - def: The sequence definition
//   - docsDir: The base documentation directory (typically "docs/")
//
// Returns:
//   - The next sequence number
//   - An error if validation fails, regex is invalid, or scan exceeds limits
//
// Example:
//
//	def := SequenceDefinition{
//	    Name: "adr", Dir: "adr", Glob: "adr-*.md",
//	    Regex: "^adr-(\\d{3})-", Width: 3, Start: 1,
//	}
//	next, err := ComputeNextInSequence(def, "docs")
//	// If docs/adr/ contains adr-001.md and adr-003.md, returns 4
func ComputeNextInSequence(def SequenceDefinition, docsDir string) (int, error) {
	if def.Dir == "" || def.Glob == "" || def.Regex == "" {
		return 0, errors.New("sequence definition is incomplete")
	}
	if docsDir == "" {
		return 0, errors.New("docs directory is required")
	}
	if filepath.IsAbs(def.Dir) {
		return 0, errors.New("sequence dir must be relative")
	}

	cleanDir := filepath.Clean(def.Dir)
	segments := strings.Split(cleanDir, string(os.PathSeparator))
	if slices.Contains(segments, "..") {
		return 0, errors.New("sequence dir must not contain '..'")
	}

	dirPath := filepath.Join(docsDir, cleanDir)
	rel, err := filepath.Rel(docsDir, dirPath)
	if err != nil || strings.HasPrefix(rel, "..") {
		return 0, errors.New("sequence dir must be under docs")
	}

	re, err := regexp.Compile(def.Regex)
	if err != nil {
		return 0, fmt.Errorf("invalid sequence regex: %w", err)
	}
	if re.NumSubexp() != 1 {
		return 0, errors.New("sequence regex must have exactly one capture group")
	}

	matches, err := filepath.Glob(filepath.Join(dirPath, def.Glob))
	if err != nil {
		return 0, fmt.Errorf("sequence glob failed: %w", err)
	}

	if len(matches) > maxSequenceFiles {
		return 0, fmt.Errorf("sequence scan exceeded %d files", maxSequenceFiles)
	}

	maxValue := 0
	found := false
	for _, match := range matches {
		base := filepath.Base(match)
		sub := re.FindStringSubmatch(base)
		if len(sub) != 2 {
			continue
		}
		value, err := strconv.Atoi(sub[1])
		if err != nil || value <= 0 {
			continue
		}
		if value > maxValue {
			maxValue = value
			found = true
		}
	}

	if found {
		return maxValue + 1, nil
	}

	if def.Start > 0 {
		return def.Start, nil
	}
	return 1, nil
}
