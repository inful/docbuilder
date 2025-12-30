package lint

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// Formatter formats linting results for output.
type Formatter interface {
	Format(w io.Writer, result *Result, detectedPath string, wasAutoDetected bool) error
}

// TextFormatter formats results as human-readable text.
type TextFormatter struct{}

// NewTextFormatter creates a text formatter.
func NewTextFormatter(useColor bool) *TextFormatter {
	return &TextFormatter{}
}

// Format outputs results in human-readable text format.
func (f *TextFormatter) Format(w io.Writer, result *Result, detectedPath string, wasAutoDetected bool) error {
	// Header
	if wasAutoDetected {
		if _, err := fmt.Fprintf(w, "Detected documentation directory: %s\n", detectedPath); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(w, "Linting documentation in: %s\n", detectedPath); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, strings.Repeat("━", 60)); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}

	// Group issues by file
	issuesByFile := make(map[string][]Issue)
	for _, issue := range result.Issues {
		issuesByFile[issue.FilePath] = append(issuesByFile[issue.FilePath], issue)
	}

	// Output issues
	for filePath, issues := range issuesByFile {
		for _, issue := range issues {
			if err := f.formatIssue(w, filePath, issue); err != nil {
				return err
			}
			if _, err := fmt.Fprintln(w); err != nil {
				return err
			}
		}
	}

	// Summary
	if _, err := fmt.Fprintln(w, strings.Repeat("━", 60)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Results:\n"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "  %d files scanned\n", result.FilesTotal); err != nil {
		return err
	}

	errorCount := result.ErrorCount()
	warningCount := result.WarningCount()

	if errorCount > 0 {
		if _, err := fmt.Fprintf(w, "  %d error%s (blocks build)\n", errorCount, pluralize(errorCount)); err != nil {
			return err
		}
	}
	if warningCount > 0 {
		if _, err := fmt.Fprintf(w, "  %d warning%s (should fix)\n", warningCount, pluralize(warningCount)); err != nil {
			return err
		}
	}

	infoCount := 0
	for _, issue := range result.Issues {
		if issue.Severity == SeverityInfo {
			infoCount++
		}
	}
	if infoCount > 0 {
		if _, err := fmt.Fprintf(w, "  %d info (explicitly allowed)\n", infoCount); err != nil {
			return err
		}
	}

	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}

	// Final message
	if err := f.printFinalMessage(w, result); err != nil {
		return err
	}

	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}
	return nil
}

// printFinalMessage prints the appropriate final message based on the result.
func (f *TextFormatter) printFinalMessage(w io.Writer, result *Result) error {
	if result.HasErrors() {
		return f.printMessages(w,
			"❌ Documentation has errors that will prevent Hugo build.",
			"   Run: docbuilder lint --fix")
	}
	if result.HasWarnings() {
		return f.printMessages(w,
			"⚠️  Documentation has warnings. Consider fixing before commit.",
			"   To auto-fix: docbuilder lint --fix")
	}
	if len(result.Issues) > 0 {
		return f.printMessages(w, "ℹ️  All issues are informational.")
	}
	return f.printMessages(w, "✨ All documentation passes linting!")
}

// printMessages prints multiple lines to the writer.
func (f *TextFormatter) printMessages(w io.Writer, messages ...string) error {
	for _, msg := range messages {
		if _, err := fmt.Fprintln(w, msg); err != nil {
			return err
		}
	}
	return nil
}

// formatIssue formats a single issue.
func (f *TextFormatter) formatIssue(w io.Writer, filePath string, issue Issue) error {
	// Icon based on severity
	var icon string
	switch issue.Severity {
	case SeverityError:
		icon = "✗"
	case SeverityWarning:
		icon = "⚠"
	case SeverityInfo:
		icon = "ℹ"
	}

	// Header
	if _, err := fmt.Fprintf(w, "%s %s\n", icon, filePath); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "  %s: %s\n", issue.Severity, issue.Message); err != nil {
		return err
	}

	// Explanation (indented)
	if issue.Explanation != "" {
		lines := strings.SplitSeq(strings.TrimSpace(issue.Explanation), "\n")
		for line := range lines {
			if _, err := fmt.Fprintf(w, "  %s\n", line); err != nil {
				return err
			}
		}
	}

	// Fix suggestion
	if issue.Fix != "" {
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "  Fix: %s\n", issue.Fix); err != nil {
			return err
		}
	}

	return nil
}

// JSONFormatter formats results as JSON.
type JSONFormatter struct{}

// NewJSONFormatter creates a JSON formatter.
func NewJSONFormatter() *JSONFormatter {
	return &JSONFormatter{}
}

// JSONOutput represents the JSON output structure.
type JSONOutput struct {
	Path            string      `json:"path"`
	WasAutoDetected bool        `json:"was_auto_detected"`
	FilesTotal      int         `json:"files_total"`
	ErrorCount      int         `json:"error_count"`
	WarningCount    int         `json:"warning_count"`
	InfoCount       int         `json:"info_count"`
	Issues          []JSONIssue `json:"issues"`
}

// JSONIssue represents a single issue in JSON format.
type JSONIssue struct {
	FilePath    string `json:"file_path"`
	Severity    string `json:"severity"`
	Rule        string `json:"rule"`
	Message     string `json:"message"`
	Explanation string `json:"explanation,omitempty"`
	Fix         string `json:"fix,omitempty"`
	Line        int    `json:"line,omitempty"`
}

// Format outputs results in JSON format.
func (f *JSONFormatter) Format(w io.Writer, result *Result, detectedPath string, wasAutoDetected bool) error {
	output := JSONOutput{
		Path:            detectedPath,
		WasAutoDetected: wasAutoDetected,
		FilesTotal:      result.FilesTotal,
		ErrorCount:      result.ErrorCount(),
		WarningCount:    result.WarningCount(),
		InfoCount:       0,
	}

	for _, issue := range result.Issues {
		if issue.Severity == SeverityInfo {
			output.InfoCount++
		}

		output.Issues = append(output.Issues, JSONIssue{
			FilePath:    issue.FilePath,
			Severity:    issue.Severity.String(),
			Rule:        issue.Rule,
			Message:     issue.Message,
			Explanation: issue.Explanation,
			Fix:         issue.Fix,
			Line:        issue.Line,
		})
	}

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}

// NewFormatter creates the appropriate formatter based on format string.
func NewFormatter(format string, useColor bool) Formatter {
	switch format {
	case "json":
		return NewJSONFormatter()
	default:
		return NewTextFormatter(useColor)
	}
}

// pluralize returns "s" if count != 1, otherwise empty string.
func pluralize(count int) string {
	if count == 1 {
		return ""
	}
	return "s"
}
