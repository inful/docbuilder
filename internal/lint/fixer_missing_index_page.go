package lint

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/frontmatterops"

	"github.com/google/uuid"
)

// MissingIndexPageUpdate records a generated _index.md file.
type MissingIndexPageUpdate struct {
	DirPath string
	Path    string
	Success bool
	Error   error
}

// applyMissingIndexPageFixes generates a placeholder _index.md file in
// any directory that the rule flagged (≥3 .md children, no _index.md,
// not in the flat-collection skip list). The placeholder contains a
// short intro and a bulleted list of the first 3 child doc titles
// (frontmatter title when available, filename otherwise).
//
// The generated _index.md is intentionally minimal — it's meant to
// unblock the lint pass while leaving the substantive content to be
// authored by humans.
//
// The new files are queued for fingerprint regeneration AND for uid
// fixup so the next phases (applyFingerprintFixes, applyUIDFixes)
// populate them automatically.
func (f *Fixer) applyMissingIndexPageFixes(targets map[string]struct{}, fixResult *FixResult, fingerprintTargets map[string]struct{}, uidTargets map[string]struct{}) {
	if len(targets) == 0 {
		return
	}

	// Targets are directory paths stored as `dir/_index.md` (the
	// rule's FilePath). Strip the trailing `/<indexFilename>` so we
	// operate on the directory itself.
	dirs := make([]string, 0, len(targets))
	for issuePath := range targets {
		dir := filepath.Dir(issuePath)
		// Defensive: skip flat collections up front.
		if _, ok := missingIndexFlatCollections[filepath.Base(dir)]; ok {
			continue
		}
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)

	for _, dir := range dirs {
		indexPath := filepath.Join(dir, indexFilename)
		if _, err := os.Stat(indexPath); err == nil {
			continue
		}

		body := f.buildIndexBody(dir)
		if f.dryRun {
			fixResult.MissingIndexPages = append(fixResult.MissingIndexPages, MissingIndexPageUpdate{
				DirPath: dir,
				Path:    indexPath,
				Success: true,
			})
			// Estimate: each generated _index.md addresses one rule hit.
			fixResult.ErrorsFixed++
			continue
		}

		if err := os.WriteFile(indexPath, []byte(body), 0o600); err != nil {
			fixResult.Errors = append(fixResult.Errors, fmt.Errorf("write %s: %w", indexPath, err))
			continue
		}
		fixResult.MissingIndexPages = append(fixResult.MissingIndexPages, MissingIndexPageUpdate{
			DirPath: dir,
			Path:    indexPath,
			Success: true,
		})
		// Queue both: new files have no fingerprint (will be regenerated)
		// and no uid (will be assigned by the uid fixer). Closure-shared
		// with the caller.
		uidTargets[indexPath] = struct{}{}
		fingerprintTargets[indexPath] = struct{}{}
	}
}

// buildIndexBody builds a minimal _index.md body listing the first 3
// children of dir with their frontmatter titles when available.
func (f *Fixer) buildIndexBody(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return indexTemplate(dir, nil)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		n := e.Name()
		if n == indexFilename {
			continue
		}
		if !IsDocFile(n) {
			continue
		}
		names = append(names, n)
	}
	sort.Strings(names)
	if len(names) > 3 {
		names = names[:3]
	}
	children := make([]indexChild, 0, len(names))
	for _, n := range names {
		title := n
		fm, _ := readFrontmatter(filepath.Join(dir, n))
		if t, ok := fm["title"].(string); ok && strings.TrimSpace(t) != "" {
			title = t
		}
		children = append(children, indexChild{name: n, title: title})
	}
	return indexTemplate(dir, children)
}

// readFrontmatter reads the frontmatter fields from a markdown file
// without parsing the body. Returns an empty map on any error.
func readFrontmatter(path string) (map[string]any, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- path comes from the lint walk over controlled directories.
	if err != nil {
		return nil, err
	}
	fm, _, _, _, err := frontmatterops.Read(data)
	if err != nil {
		return nil, err
	}
	if fm == nil {
		return map[string]any{}, nil
	}
	return fm, nil
}

// indexTemplate formats the body of a generated _index.md.
// Uses the directory's expected category (from directoryToCategory)
// so that the cross-mode-category rule doesn't fire on the new file.
type indexChild struct {
	name  string
	title string
}

func indexTemplate(dir string, children []indexChild) string {
	dirName := filepath.Base(dir)
	category := directoryToCategory[dirName]
	if category == "" {
		// Fallback for directories outside the canonical map. This
		// shouldn't happen in practice since the rule only flags
		// directories in the map.
		category = "documentation"
	}
	uid := uuid.NewString()
	now := time.Now().UTC()
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("title: \"")
	b.WriteString(titleCase(dirName))
	b.WriteString("\"\n")
	b.WriteString("uid: ")
	b.WriteString(uid)
	b.WriteString("\n")
	b.WriteString("aliases:\n")
	b.WriteString("  - /_uid/")
	b.WriteString(uid)
	b.WriteString("/\n")
	b.WriteString("categories:\n")
	b.WriteString("  - ")
	b.WriteString(category)
	b.WriteString("\n")
	b.WriteString("date: ")
	b.WriteString(now.Format("2006-01-02T15:04:05Z"))
	b.WriteString("\n")
	b.WriteString("lastmod: \"")
	b.WriteString(now.Format("2006-01-02"))
	b.WriteString("\"\n")
	b.WriteString("tags: []\n")
	b.WriteString("---\n\n")
	b.WriteString("# ")
	b.WriteString(titleCase(dirName))
	b.WriteString("\n\n")
	b.WriteString("This index page was auto-generated by `docbuilder lint --fix` as a placeholder. ")
	b.WriteString("Replace this text with a curated introduction; the bullet list below is a starting point.\n\n")

	if len(children) > 0 {
		b.WriteString("## Contents\n\n")
		for _, c := range children {
			b.WriteString("- [")
			b.WriteString(c.title)
			b.WriteString("](")
			b.WriteString(c.name)
			b.WriteString(")\n")
		}
		b.WriteString("\n")
	}

	return b.String()
}

// titleCase capitalises the first letter of each hyphen-separated
// word: `how-to` → `How To`.
func titleCase(s string) string {
	parts := strings.Split(s, "-")
	for i, p := range parts {
		if p == "" {
			continue
		}
		parts[i] = strings.ToUpper(p[:1]) + p[1:]
	}
	return strings.Join(parts, " ")
}

// missingIndexFlatCollections mirrors the rule's static map.
var missingIndexFlatCollections = map[string]bool{
	"adr":      true,
	"examples": true,
}
