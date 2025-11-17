package repository

import (
	"fmt"
	"regexp"
	"strings"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// Filter decides whether repositories should be included in a build.
type Filter struct {
	include []*regexp.Regexp
	exclude []*regexp.Regexp
}

// NewFilter constructs a RepositoryFilter from glob patterns.
// Empty include slice means include all (unless excluded).
func NewFilter(includeGlobs, excludeGlobs []string) (*Filter, error) {
	compile := func(globs []string) ([]*regexp.Regexp, error) {
		out := make([]*regexp.Regexp, 0, len(globs))
		for _, g := range globs {
			if strings.TrimSpace(g) == "" {
				continue
			}
			// convert glob to regex using filepath.Match semantics approximation
			pattern := globToRegex(g)
			r, err := regexp.Compile(pattern)
			if err != nil {
				return nil, fmt.Errorf("compile glob %s: %w", g, err)
			}
			out = append(out, r)
		}
		return out, nil
	}
	incs, err := compile(includeGlobs)
	if err != nil {
		return nil, err
	}
	excs, err := compile(excludeGlobs)
	if err != nil {
		return nil, err
	}
	return &Filter{include: incs, exclude: excs}, nil
}

// Include returns true if repo passes the filter along with an exclusion reason if false.
func (f *Filter) Include(repo config.Repository) (bool, string) {
	name := repo.Name
	if f == nil {
		return true, ""
	}
	// Exclusion precedence first
	for _, rx := range f.exclude {
		if rx.MatchString(name) {
			return false, "excluded_by_pattern"
		}
	}
	// Include patterns: if none defined -> include
	if len(f.include) == 0 {
		return true, ""
	}
	for _, rx := range f.include {
		if rx.MatchString(name) {
			return true, ""
		}
	}
	return false, "not_in_includes"
}

// globToRegex converts a shell-style glob to a regex string (anchored).
func globToRegex(glob string) string {
	var b strings.Builder
	b.WriteString("^")
	for i := 0; i < len(glob); i++ {
		c := glob[i]
		switch c {
		case '*':
			b.WriteString(".*")
		case '?':
			b.WriteString(".")
		case '.', '+', '(', ')', '|', '^', '$', '{', '}', '[', ']', '\\':
			b.WriteByte('\\')
			b.WriteByte(c)
		default:
			b.WriteByte(c)
		}
	}
	b.WriteString("$")
	return b.String()
}
