---
uid: 29426dd7-62c7-4e24-8378-5487c13fbee7
aliases:
  - /_uid/29426dd7-62c7-4e24-8378-5487c13fbee7/
fingerprint: dd2cc4c7e6aa9f24885bcc4aeb55515edfe0a680534abb0872c84fbfbf63efb1
---

# JSON Output Schema

DocBuilder's linter supports machine-readable JSON output for CI/CD integration, automated reporting, and tooling integration.

## Usage

```bash
# Output JSON to stdout
docbuilder lint --format=json

# Save to file
docbuilder lint --format=json > lint-report.json

# Use in CI pipeline
docbuilder lint --format=json | jq '.summary.errors'
```

## Schema Definition

### Root Object

```json
{
  "version": "1.0",
  "timestamp": "2025-12-29T20:30:00Z",
  "path": "docs",
  "auto_detected": true,
  "summary": {
    "total_files": 38,
    "errors": 2,
    "warnings": 5,
    "passed": 31
  },
  "issues": [
    {
      "file": "docs/API Guide.md",
      "line": 0,
      "column": 0,
      "severity": "error",
      "rule": "filename-convention",
      "message": "Filename contains spaces and uppercase letters",
      "suggestion": "docs/api-guide.md",
      "context": {
        "current": "API Guide.md",
        "suggested": "api-guide.md",
        "reason": "Spaces create %20 in URLs; uppercase causes case-sensitivity issues"
      }
    }
  ],
  "broken_links": [
    {
      "source_file": "docs/getting-started.md",
      "line": 45,
      "link_text": "API Reference",
      "target": "./api/overview.md",
      "link_type": "inline",
      "error": "target file does not exist"
    }
  ],
  "exit_code": 2
}
```

## Field Descriptions

### Root Fields

| Field | Type | Description |
|-------|------|-------------|
| `version` | string | Schema version (semver format) |
| `timestamp` | string | ISO 8601 timestamp of lint execution |
| `path` | string | Path that was linted |
| `auto_detected` | boolean | Whether the path was auto-detected (vs. explicitly provided) |
| `summary` | object | Summary statistics (see below) |
| `issues` | array | List of lint issues found (see below) |
| `broken_links` | array | List of broken links detected (see below) |
| `exit_code` | integer | Exit code: 0 (success), 1 (warnings), 2 (errors) |

### Summary Object

| Field | Type | Description |
|-------|------|-------------|
| `total_files` | integer | Total number of files scanned |
| `errors` | integer | Number of files with errors (blocks build) |
| `warnings` | integer | Number of files with warnings |
| `passed` | integer | Number of files that passed all checks |

**Invariant:** `errors + warnings + passed = total_files`

### Issue Object

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `file` | string | ✅ | Relative path to the file with the issue |
| `line` | integer | ✅ | Line number (0 for file-level issues) |
| `column` | integer | ✅ | Column number (0 for file-level issues) |
| `severity` | string | ✅ | `"error"` or `"warning"` |
| `rule` | string | ✅ | Rule identifier (e.g., `"filename-convention"`) |
| `message` | string | ✅ | Human-readable error message |
| `suggestion` | string | ❌ | Suggested fix (if applicable) |
| `context` | object | ❌ | Additional context about the issue |

### Broken Link Object

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `source_file` | string | ✅ | File containing the broken link |
| `line` | integer | ✅ | Line number where link appears |
| `link_text` | string | ✅ | Display text of the link |
| `target` | string | ✅ | Target URL/path that is broken |
| `link_type` | string | ✅ | Type: `"inline"`, `"reference"`, `"image"` |
| `error` | string | ✅ | Description of why link is broken |

## Rule Identifiers

Current rules that may appear in the `rule` field:

| Rule ID | Description | Severity |
|---------|-------------|----------|
| `filename-convention` | Filename violates naming conventions | error |
| `missing-title` | Frontmatter missing required `title` field | warning |
| `broken-links` | Internal link target does not exist | error |
| `invalid-extension` | File extension not in whitelist | error |

## Examples

### Success (No Issues)

```json
{
  "version": "1.0",
  "timestamp": "2025-12-29T20:30:00Z",
  "path": "docs",
  "auto_detected": true,
  "summary": {
    "total_files": 15,
    "errors": 0,
    "warnings": 0,
    "passed": 15
  },
  "issues": [],
  "broken_links": [],
  "exit_code": 0
}
```

### Errors Found

```json
{
  "version": "1.0",
  "timestamp": "2025-12-29T20:31:00Z",
  "path": "docs",
  "auto_detected": false,
  "summary": {
    "total_files": 20,
    "errors": 3,
    "warnings": 2,
    "passed": 15
  },
  "issues": [
    {
      "file": "docs/Getting Started.md",
      "line": 0,
      "column": 0,
      "severity": "error",
      "rule": "filename-convention",
      "message": "Filename contains spaces",
      "suggestion": "docs/getting-started.md",
      "context": {
        "current": "Getting Started.md",
        "suggested": "getting-started.md",
        "reason": "Spaces create %20 in URLs"
      }
    },
    {
      "file": "docs/api/_index.md",
      "line": 0,
      "column": 0,
      "severity": "warning",
      "rule": "missing-title",
      "message": "Frontmatter missing 'title' field",
      "context": {
        "recommendation": "Add title to frontmatter for better navigation"
      }
    }
  ],
  "broken_links": [
    {
      "source_file": "docs/getting-started.md",
      "line": 23,
      "link_text": "API Reference",
      "target": "./api/overview.md",
      "link_type": "inline",
      "error": "target file does not exist"
    }
  ],
  "exit_code": 2
}
```

## Parsing Examples

### Shell (jq)

```bash
# Extract error count
ERRORS=$(jq -r '.summary.errors' lint-report.json)

# Get all error messages
jq -r '.issues[] | select(.severity=="error") | .message' lint-report.json

# List files with issues
jq -r '.issues[].file | unique' lint-report.json

# Check if any broken links exist
HAS_BROKEN_LINKS=$(jq '.broken_links | length > 0' lint-report.json)
```

### Python

```python
import json

with open('lint-report.json') as f:
    report = json.load(f)

# Check for errors
if report['summary']['errors'] > 0:
    print(f"❌ {report['summary']['errors']} errors found")
    for issue in report['issues']:
        if issue['severity'] == 'error':
            print(f"  {issue['file']}:{issue['line']} - {issue['message']}")
    exit(1)

# Check for broken links
if report['broken_links']:
    print(f"🔗 {len(report['broken_links'])} broken links found")
    for link in report['broken_links']:
        print(f"  {link['source_file']}:{link['line']} -> {link['target']}")
```

### JavaScript/Node.js

```javascript
const fs = require('fs');

const report = JSON.parse(fs.readFileSync('lint-report.json', 'utf8'));

// Group issues by severity
const errors = report.issues.filter(i => i.severity === 'error');
const warnings = report.issues.filter(i => i.severity === 'warning');

console.log(`Errors: ${errors.length}, Warnings: ${warnings.length}`);

// Build summary for PR comment
let comment = `## Documentation Lint Results\n\n`;
comment += `📊 ${report.summary.total_files} files scanned\n\n`;

if (errors.length > 0) {
    comment += `### ❌ Errors (${errors.length})\n\n`;
    errors.slice(0, 5).forEach(error => {
        comment += `- **${error.file}**: ${error.message}\n`;
    });
}
```

### Go

```go
package main

import (
    "encoding/json"
    "fmt"
    "os"
)

type LintReport struct {
    Version      string   `json:"version"`
    Timestamp    string   `json:"timestamp"`
    Path         string   `json:"path"`
    AutoDetected bool     `json:"auto_detected"`
    Summary      Summary  `json:"summary"`
    Issues       []Issue  `json:"issues"`
    BrokenLinks  []Link   `json:"broken_links"`
    ExitCode     int      `json:"exit_code"`
}

type Summary struct {
    TotalFiles int `json:"total_files"`
    Errors     int `json:"errors"`
    Warnings   int `json:"warnings"`
    Passed     int `json:"passed"`
}

type Issue struct {
    File       string                 `json:"file"`
    Line       int                    `json:"line"`
    Column     int                    `json:"column"`
    Severity   string                 `json:"severity"`
    Rule       string                 `json:"rule"`
    Message    string                 `json:"message"`
    Suggestion string                 `json:"suggestion,omitempty"`
    Context    map[string]interface{} `json:"context,omitempty"`
}

type Link struct {
    SourceFile string `json:"source_file"`
    Line       int    `json:"line"`
    LinkText   string `json:"link_text"`
    Target     string `json:"target"`
    LinkType   string `json:"link_type"`
    Error      string `json:"error"`
}

func main() {
    data, _ := os.ReadFile("lint-report.json")
    
    var report LintReport
    json.Unmarshal(data, &report)
    
    fmt.Printf("Files: %d, Errors: %d, Warnings: %d\n",
        report.Summary.TotalFiles,
        report.Summary.Errors,
        report.Summary.Warnings)
}
```

## Version History

### v1.0 (Current)

Initial JSON schema with:
- Basic issue reporting
- Broken link detection
- Summary statistics
- Exit code reporting

### Planned Enhancements

Future versions may include:
- `fixes_available` field indicating which issues can be auto-fixed
- `performance` metrics (files/second, total time)
- `rules_applied` list of rules that were checked
- JUnit XML output format for better CI integration
- SARIF (Static Analysis Results Interchange Format) support

## Integration Tips

### CI/CD Best Practices

1. **Always capture JSON output** even on failure:
   ```bash
   docbuilder lint --format=json > lint-report.json || true
   ```

2. **Upload as artifact** for debugging:
   ```yaml
   artifacts:
     when: always
     paths:
       - lint-report.json
   ```

3. **Parse before failing pipeline** to provide better feedback:
   ```bash
   EXIT_CODE=$(jq -r '.exit_code' lint-report.json)
   exit $EXIT_CODE
   ```

4. **Rate-limit PR comments** to avoid spam on large changes

5. **Cache results** for incremental linting on large repositories

### Tooling Integration

The JSON output is designed to integrate with:
- **Code review tools**: GitHub Actions, GitLab CI, BitBucket Pipelines
- **IDE extensions**: VS Code, IntelliJ, Vim/NeoVim
- **Quality dashboards**: SonarQube, CodeClimate
- **Slack/Discord bots**: Automated team notifications
- **Documentation portals**: Link to specific issues in docs

## Schema Stability

This schema follows semantic versioning:
- **Major version** changes indicate breaking changes
- **Minor version** changes add new fields (backward compatible)
- **Patch version** changes are documentation/clarification only

The `version` field in the JSON output reflects the schema version, not the DocBuilder version.
