package templates

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const maxSequenceFiles = 10000

// ErrNoSequenceDefinition indicates that no sequence definition was provided.
var ErrNoSequenceDefinition = errors.New("sequence definition missing")

// SequenceDefinition describes how to compute a sequential number.
type SequenceDefinition struct {
	Name  string
	Dir   string
	Glob  string
	Regex string
	Width int
	Start int
}

// ParseSequenceDefinition parses a sequence definition from JSON metadata.
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

// ComputeNextInSequence scans docsDir based on the sequence definition.
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
	for _, segment := range strings.Split(cleanDir, string(os.PathSeparator)) {
		if segment == ".." {
			return 0, errors.New("sequence dir must not contain '..'")
		}
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
