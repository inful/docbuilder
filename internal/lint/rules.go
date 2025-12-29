package lint

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

// FilenameRule validates that filenames follow Hugo/DocBuilder conventions.
type FilenameRule struct{}

// Name returns the rule identifier.
func (r *FilenameRule) Name() string {
	return "filename-conventions"
}

// AppliesTo returns true for all documentation and asset files.
func (r *FilenameRule) AppliesTo(filePath string) bool {
	return IsDocFile(filePath) || IsAssetFile(filePath)
}

// Check validates filename conventions.
func (r *FilenameRule) Check(filePath string) ([]Issue, error) {
	filename := filepath.Base(filePath)
	var issues []Issue

	// Check for whitelisted double extensions first
	if isWhitelistedDoubleExtension(filename) {
		// This is explicitly allowed, return info message
		issues = append(issues, Issue{
			FilePath:    filePath,
			Severity:    SeverityInfo,
			Rule:        r.Name(),
			Message:     "Whitelisted double extension",
			Explanation: "File uses whitelisted double extension for embedded diagrams (.drawio.png or .drawio.svg)",
		})
		return issues, nil
	}

	// Check for invalid double extensions
	if hasInvalidDoubleExtension(filename) {
		issues = append(issues, Issue{
			FilePath: filePath,
			Severity: SeverityError,
			Rule:     r.Name(),
			Message:  "Invalid double extension detected",
			Explanation: `File has non-whitelisted double extension that Hugo will attempt to process.

Whitelisted double extensions (allowed):
  • .drawio.png (Draw.io embedded PNG diagrams)
  • .drawio.svg (Draw.io embedded SVG diagrams)

Common problematic patterns:
  • .md.backup, .markdown.old (backup files)
  • .png.tmp, .jpg.bak (temporary files)`,
			Fix: "Remove backup files from docs directory or use .gitignore",
		})
		return issues, nil
	}

	// Check for uppercase letters
	if hasUppercase(filename) {
		suggested := strings.ToLower(filename)
		issues = append(issues, Issue{
			FilePath: filePath,
			Severity: SeverityError,
			Rule:     r.Name(),
			Message:  "Filename contains uppercase letters",
			Explanation: `Uppercase letters in filenames cause URL inconsistency and case-sensitivity 
issues across different platforms.

Current:  ` + filename + `
Suggested: ` + suggested + `

Why this matters:
  • Hugo converts filenames to URL slugs
  • Case sensitivity varies by OS (Linux vs macOS/Windows)
  • Creates inconsistent user experience`,
			Fix: "Rename to lowercase: " + suggested,
		})
	}

	// Check for spaces
	if strings.Contains(filename, " ") {
		suggested := suggestFilename(filename)
		issues = append(issues, Issue{
			FilePath: filePath,
			Severity: SeverityError,
			Rule:     r.Name(),
			Message:  "Filename contains spaces",
			Explanation: `Spaces in filenames create problematic URLs with %20 encoding 
and break cross-references.

Current:  ` + filename + `
Suggested: ` + suggested + `

Why this matters:
  • Spaces become %20 in URLs: /docs/my%20file/
  • Makes links harder to type and share
  • Hugo expects hyphen-separated filenames`,
			Fix: "Rename using hyphens: " + suggested,
		})
	}

	// Check for special characters (except allowed ones)
	if hasSpecialChars(filename) {
		suggested := suggestFilename(filename)
		invalidChars := findSpecialChars(filename)
		issues = append(issues, Issue{
			FilePath: filePath,
			Severity: SeverityError,
			Rule:     r.Name(),
			Message:  "Filename contains special characters: " + strings.Join(invalidChars, ", "),
			Explanation: `Special characters are unsupported by Hugo slugify and may cause 
shell escaping issues.

Current:  ` + filename + `
Suggested: ` + suggested + `

Allowed characters: [a-z0-9-_.]
Invalid characters found: ` + strings.Join(invalidChars, ", "),
			Fix: "Rename to remove special characters: " + suggested,
		})
	}

	// Check for leading/trailing hyphens or underscores
	nameWithoutExt := strings.TrimSuffix(filename, filepath.Ext(filename))
	if strings.HasPrefix(nameWithoutExt, "-") || strings.HasPrefix(nameWithoutExt, "_") ||
		strings.HasSuffix(nameWithoutExt, "-") || strings.HasSuffix(nameWithoutExt, "_") {
		suggested := suggestFilename(filename)
		issues = append(issues, Issue{
			FilePath: filePath,
			Severity: SeverityError,
			Rule:     r.Name(),
			Message:  "Filename has leading or trailing hyphens/underscores",
			Explanation: `Leading or trailing hyphens/underscores create malformed URLs.

Current:  ` + filename + `
Suggested: ` + suggested + `

Examples of problematic URLs:
  • /-docs/ or /_temp/`,
			Fix: "Rename to remove leading/trailing separators: " + suggested,
		})
	}

	return issues, nil
}

// isWhitelistedDoubleExtension checks if filename has an allowed double extension.
func isWhitelistedDoubleExtension(filename string) bool {
	whitelisted := []string{".drawio.png", ".drawio.svg"}
	lower := strings.ToLower(filename)
	for _, ext := range whitelisted {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}

// hasInvalidDoubleExtension checks for non-whitelisted double extensions.
func hasInvalidDoubleExtension(filename string) bool {
	// If it's whitelisted, it's not invalid
	if isWhitelistedDoubleExtension(filename) {
		return false
	}

	// Check for common double extension patterns
	parts := strings.Split(filename, ".")
	if len(parts) >= 3 {
		// Has at least two extensions (name.ext1.ext2)
		secondToLastExt := "." + parts[len(parts)-2]
		// Check if second-to-last looks like a file extension
		commonExts := []string{".md", ".markdown", ".png", ".jpg", ".jpeg", ".gif", ".svg", ".tmp", ".bak", ".backup", ".old", ".yaml", ".yml", ".json", ".toml"}
		for _, ext := range commonExts {
			if strings.EqualFold(secondToLastExt, ext) {
				return true
			}
		}
	}
	return false
}

// hasUppercase checks if filename contains uppercase letters.
func hasUppercase(filename string) bool {
	for _, r := range filename {
		if unicode.IsUpper(r) {
			return true
		}
	}
	return false
}

// hasSpecialChars checks if filename contains characters outside [a-z0-9-_.]
func hasSpecialChars(filename string) bool {
	// Pattern matches valid characters: lowercase, digits, hyphen, underscore, dot
	validPattern := regexp.MustCompile(`^[a-z0-9\-_.]+$`)
	return !validPattern.MatchString(filename)
}

// findSpecialChars returns list of special characters found in filename.
func findSpecialChars(filename string) []string {
	seen := make(map[string]bool)
	var chars []string

	for _, r := range filename {
		char := string(r)
		// Skip valid characters
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			continue
		}
		if !seen[char] {
			chars = append(chars, char)
			seen[char] = true
		}
	}
	return chars
}

// suggestFilename returns a suggested filename following conventions.
func suggestFilename(filename string) string {
	// Separate extension
	ext := filepath.Ext(filename)
	name := strings.TrimSuffix(filename, ext)

	// Handle whitelisted double extensions
	if isWhitelistedDoubleExtension(filename) {
		return filename // Already valid
	}

	// Handle invalid double extensions - keep only the last extension
	parts := strings.Split(filename, ".")
	if len(parts) >= 3 {
		name = strings.Join(parts[:len(parts)-1], ".")
		ext = "." + parts[len(parts)-1]
	}

	// Convert to lowercase
	name = strings.ToLower(name)
	ext = strings.ToLower(ext)

	// Replace spaces with hyphens
	name = strings.ReplaceAll(name, " ", "-")

	// Remove special characters (keep only alphanumeric, hyphen, underscore)
	validPattern := regexp.MustCompile(`[^a-z0-9\-_]`)
	name = validPattern.ReplaceAllString(name, "")

	// Replace multiple hyphens with single hyphen
	multiHyphen := regexp.MustCompile(`-+`)
	name = multiHyphen.ReplaceAllString(name, "-")

	// Remove leading/trailing hyphens and underscores
	name = strings.Trim(name, "-_")

	return name + ext
}

// DetectDefaultPath detects the documentation directory using intelligent defaults.
func DetectDefaultPath() (string, bool) {
	// Check for docs/ directory
	if info, err := os.Stat("docs"); err == nil && info.IsDir() {
		return "docs", true
	}

	// Check for documentation/ directory
	if info, err := os.Stat("documentation"); err == nil && info.IsDir() {
		return "documentation", true
	}

	// Fallback to current directory
	return ".", false
}
