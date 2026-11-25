package lint

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Linter performs linting operations on documentation files.
type Linter struct {
	cfg          *Config
	rules        []Rule
	excludeGlobs []globPattern // pre-compiled from cfg.Excludes
	// excludePatternsList holds the patterns the globs were compiled
	// from, for the callers that need them.
	excludePatternsList []string
}

// globPattern is a compiled glob pattern that supports `**` as a recursive
// wildcard in addition to the standard path.Match semantics.
type globPattern struct {
	raw  string
	re   *regexp.Regexp
	base string // directory portion (before any `**`)
}

// NewLinter creates a new linter with the given configuration.
func NewLinter(cfg *Config) *Linter {
	if cfg == nil {
		cfg = &Config{}
	}

	excludes := cfg.Excludes
	if excludes == nil {
		excludes = DefaultExcludes
	}

	return &Linter{
		cfg:                 cfg,
		excludeGlobs:        compileGlobs(excludes),
		excludePatternsList: excludes,
		rules: []Rule{
			&FilenameRule{},
			&FrontmatterUIDRule{},
			&FrontmatterFingerprintRule{},
			&BodyH1Rule{},
			&SequencePrefixFilenameRule{},
			&TagCountRule{},
			&CategoryNamingRule{},
			&CrossModeCategoryRule{AllowedPairs: cfg.AllowedCategoryPairs},
			&DirectoryCategoryConsistencyRule{},
			&FrontmatterRequiredFieldsRule{},
			&TemplateUIDBindingRule{},
			// Additional rules will be added here in future phases
		},
	}
}

// LintPath lints all documentation files in the given path (file or directory).
func (l *Linter) LintPath(path string) (*Result, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	// The tree may have changed since the last run: the cached
	// listings of the directories are of no use any more.
	forgetDirectoryListings()

	result := &Result{
		Issues: []Issue{},
	}

	if info.IsDir() {
		err = l.lintDirectory(path, result)
	} else {
		// The excludes hold for a single file too: the pre-commit
		// hook lints every staged file on its own, and the readme of
		// the repository is never reported.
		if l.isExcluded(path, repositoryRoot(path)) {
			return result, nil
		}
		err = l.lintFile(path, result)
		result.FilesTotal = 1
	}

	// The error of the walk is returned here: the scan of the broken
	// links below overwrites it, and a file that cannot be read was
	// reported as a clean lint.
	if err != nil {
		return nil, err
	}

	// Broken link detection is a lint-time check (not only a fixer feature).
	// This ensures `docbuilder lint` fails the same way `docbuilder lint --fix` reports.
	//
	// The root of the tree is the root of the repository: the link
	// "/bar" points at a file of the site, and a file is never inside a
	// directory, so the walk that resolves the link stopped at once and
	// the link was reported as broken.
	var brokenLinks []BrokenLink
	if info.IsDir() {
		brokenLinks, err = detectBrokenLinks(path, l.excludePatterns())
	} else if IsDocFile(path) {
		// A file that cannot be read does not stop the lint.
		brokenLinks, _ = detectBrokenLinksInFile(path, repositoryRoot(path))
	}
	if err != nil {
		return nil, err
	}
	result.brokenLinks = brokenLinks
	for _, bl := range brokenLinks {
		result.Issues = append(result.Issues, brokenLinkIssue(bl))
	}

	return result, err
}

// repositoryRoot returns the root of the repository that holds the path:
// the directory that holds the .git of the repository.
//
// The excludes of the linter are paths of the repository ("README.md"),
// and they are matched against the path relative to that root: the same
// file is linted or not depending on the directory the linter was
// started in.
//
// Without a repository the directory of the path is the root.
func repositoryRoot(path string) string {
	dir := path
	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		dir = filepath.Dir(path)
	}

	abs, err := filepath.Abs(dir)
	if err != nil {
		return dir
	}

	for {
		if _, err := os.Stat(filepath.Join(abs, ".git")); err == nil {
			return abs
		}
		parent := filepath.Dir(abs)
		if parent == abs {
			return dir
		}
		abs = parent
	}
}

// brokenLinkIssue is the issue of a link that points at a file that does
// not exist.
func brokenLinkIssue(bl BrokenLink) Issue {
	return Issue{
		FilePath: bl.SourceFile,
		Severity: SeverityError,
		Rule:     "broken-links",
		Message:  "broken link target does not exist",
		Explanation: strings.TrimSpace(strings.Join([]string{
			"The documentation contains a link to a file that does not exist.",
			"This will break navigation and may cause build/link-check failures.",
			"",
			"Target: " + bl.Target,
		}, "\n")),
		Fix:  "Update the link target or add the missing file.",
		Line: bl.LineNumber,
	}
}

// lintDirectory recursively lints all documentation files in a directory.
func (l *Linter) lintDirectory(dirPath string, result *Result) error {
	return filepath.WalkDir(dirPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip hidden directories and files
		if d.Name()[0] == '.' && d.Name() != "." {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}

		// Skip directories
		if d.IsDir() {
			return nil
		}

		// Skip files matched by the exclude list (git-forge-conventional files)
		// The excludes are paths of the repository: the same file was
		// linted or not depending on the directory the linter was
		// started in.
		if l.isExcluded(path, repositoryRoot(dirPath)) {
			return nil
		}

		// Only process documentation and asset files
		if !IsDocFile(path) && !IsAssetFile(path) {
			return nil
		}

		result.FilesTotal++
		return l.lintFile(path, result)
	})
}

// lintFile applies all applicable rules to a single file.
func (l *Linter) lintFile(filePath string, result *Result) error {
	for _, rule := range l.rules {
		if !rule.AppliesTo(filePath) {
			continue
		}

		issues, err := rule.Check(filePath)
		if err != nil {
			return err
		}

		// Filter issues based on configuration
		for _, issue := range issues {
			// Skip info and warnings in quiet mode
			if l.cfg.Quiet && issue.Severity != SeverityError {
				continue
			}
			result.Issues = append(result.Issues, issue)
		}
	}

	return nil
}

// LintPaths lints the given files and directories.
//
// The broken links are detected in the tree that holds them: a link of a
// file that is linted points at a file of the tree.
func (l *Linter) LintPaths(paths []string) (*Result, error) {
	if len(paths) == 1 {
		return l.LintPath(paths[0])
	}

	result := &Result{
		Issues:     []Issue{},
		FilesTotal: 0,
	}
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		if info.IsDir() {
			if err := l.lintDirectory(path, result); err != nil {
				return nil, err
			}
			continue
		}
		if l.isExcluded(path, "") {
			continue
		}
		if !IsDocFile(path) && !IsAssetFile(path) {
			continue
		}
		result.FilesTotal++
		if err := l.lintFile(path, result); err != nil {
			return nil, err
		}
	}

	brokenLinks, blErr := detectBrokenLinks(commonRoot(paths), l.excludePatterns())
	if blErr != nil {
		return nil, blErr
	}
	result.brokenLinks = brokenLinks
	for _, bl := range brokenLinks {
		result.Issues = append(result.Issues, brokenLinkIssue(bl))
	}

	return result, nil
}

// commonRoot returns the directory that holds all the paths.
func commonRoot(paths []string) string {
	root := ""
	for _, path := range paths {
		dir := path
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			dir = filepath.Dir(path)
		}
		abs, err := filepath.Abs(dir)
		if err != nil {
			continue
		}
		if root == "" {
			root = abs
			continue
		}
		// The root climbs until it holds the other path.
		for root != filepath.Dir(root) && !strings.HasPrefix(abs, root+string(filepath.Separator)) {
			root = filepath.Dir(root)
		}
	}
	if root == "" {
		return "."
	}
	return root
}

// LintFiles lints a specific list of files (useful for Git hooks).
func (l *Linter) LintFiles(files []string) (*Result, error) {
	result := &Result{
		Issues:     []Issue{},
		FilesTotal: 0,
	}

	for _, file := range files {
		// Skip files matched by the exclude list (git-forge-conventional files)
		if l.isExcluded(file, "") {
			continue
		}

		// Only process documentation and asset files
		if !IsDocFile(file) && !IsAssetFile(file) {
			continue
		}

		// Check if file exists
		if _, err := os.Stat(file); os.IsNotExist(err) {
			continue
		}

		result.FilesTotal++
		if err := l.lintFile(file, result); err != nil {
			return result, err
		}
	}

	return result, nil
}

// excludePatterns returns the exclude patterns of the linter.
func (l *Linter) excludePatterns() []string {
	if l == nil {
		return nil
	}
	return l.excludePatternsList
}

// isExcluded reports whether the given file path matches any of the
// configured exclude glob patterns. baseDir is used to compute a
// repo-root-relative path; pass "" to fall back to the file's own path.
func (l *Linter) isExcluded(filePath, baseDir string) bool {
	if len(l.excludeGlobs) == 0 {
		return false
	}
	rel := filePath
	if baseDir != "" {
		if r, err := filepath.Rel(baseDir, filePath); err == nil {
			rel = r
		}
	}
	rel = filepath.ToSlash(rel)
	for _, g := range l.excludeGlobs {
		if matchGlob(g, rel) {
			return true
		}
	}
	return false
}

// compileGlobs compiles a list of glob patterns into the internal form
// used by isExcluded. Each pattern is translated to a regular expression
// that supports `**` as a recursive wildcard and standard path.Match
// semantics for the rest.
func compileGlobs(patterns []string) []globPattern {
	out := make([]globPattern, 0, len(patterns))
	for _, p := range patterns {
		out = append(out, compileGlob(p))
	}
	return out
}

func compileGlob(p string) globPattern {
	// Split on the first `**` to find the literal base prefix.
	base := p
	if before, _, ok := strings.Cut(p, "**"); ok {
		base = before
	}
	base = strings.TrimRight(base, "/")
	re := globToRegexp(p)
	return globPattern{raw: p, re: re, base: base}
}

// globToRegexp converts a path glob to a regular expression.
// Supports `**` (recursive wildcard) and standard path.Match semantics.
func globToRegexp(glob string) *regexp.Regexp {
	var b strings.Builder
	b.WriteString(`^`)
	i := 0
	for i < len(glob) {
		c := glob[i]
		switch c {
		case '*':
			if i+1 < len(glob) && glob[i+1] == '*' {
				// `**` matches any number of path segments (including zero)
				b.WriteString(`.*`)
				i += 2
				// Consume an optional following `/`
				if i < len(glob) && glob[i] == '/' {
					i++
				}
			} else {
				// `*` matches any chars except `/`
				b.WriteString(`[^/]*`)
				i++
			}
		case '?':
			b.WriteString(`[^/]`)
			i++
		case '.', '+', '(', ')', '|', '^', '$', '{', '}', '[', ']', '\\':
			b.WriteByte('\\')
			b.WriteByte(c)
			i++
		default:
			b.WriteByte(c)
			i++
		}
	}
	b.WriteString(`$`)
	return regexp.MustCompile(b.String())
}

// matchGlob returns true if the relative path matches the compiled glob.
//
// Semantics:
//   - Globs containing `**` are recursive and match anywhere inside the
//     base prefix (or anywhere if no prefix).
//   - Globs without `**` are exact and match only when the relative path
//     equals the glob string exactly. This avoids `README.md` accidentally
//     matching `docs/README.md` — git forges only recognize these filenames
//     at the repo root.
func matchGlob(g globPattern, rel string) bool {
	if g.raw == "" {
		return false
	}
	if strings.Contains(g.raw, "**") {
		// Recursive globs: the path must be inside the base prefix.
		if g.base != "" && !strings.HasPrefix(rel, g.base+"/") && rel != g.base {
			return false
		}
		return g.re.MatchString(rel)
	}
	// Exact globs: relative path must equal the glob exactly.
	return rel == g.raw
}
