---
aliases:
  - /_uid/29426dd7-62c7-4e24-8378-5487c13fbee7/
categories:
  - reference
date: 2026-01-01T00:00:00Z
fingerprint: ce5f1382772a6897fbcb51edfc5ca7e2643cb231cb72bcdfb1ddafb78fa1ade1
lastmod: "2026-11-24"
tags: []
title: JSON Output Schema
uid: 29426dd7-62c7-4e24-8378-5487c13fbee7
---

DocBuilder's linter writes a machine-readable report for CI/CD
integration, automated reporting and other tooling.

## Usage

```bash
# The report on stdout
docbuilder lint --format=json

# The report in a file
docbuilder lint --format=json > lint-report.json

# The errors of the report
docbuilder lint --format=json | jq '[.issues[] | select(.severity=="ERROR")] | length'
```

The command exits with 2 when the report holds errors, with 1 when it
holds warnings, and with 0 when it holds neither.

## Schema Definition

### Root Object

```json
{
  "path": "docs",
  "was_auto_detected": false,
  "files_total": 38,
  "error_count": 2,
  "warning_count": 5,
  "info_count": 0,
  "issues": [
    {
      "file_path": "docs/API Guide.md",
      "severity": "ERROR",
      "rule": "filename-conventions",
      "message": "Filename contains uppercase letters",
      "explanation": "Uppercase letters in filenames cause URL inconsistency.",
      "fix": "Rename to lowercase: api-guide.md",
      "line": 12
    }
  ]
}
```

### Root Fields

| Field | Type | Description |
|-------|------|-------------|
| `path` | string | The path that was linted. Several paths are joined with a comma. |
| `was_auto_detected` | bool | Whether the path was detected instead of given. |
| `files_total` | number | The number of files that were linted. |
| `error_count` | number | The number of issues with the severity `ERROR`. |
| `warning_count` | number | The number of issues with the severity `WARNING`. |
| `info_count` | number | The number of issues with the severity `INFO`. |
| `issues` | array | The issues of the report. |

### Issue Object

| Field | Type | Description |
|-------|------|-------------|
| `file_path` | string | The file the issue is in. |
| `severity` | string | `ERROR`, `WARNING` or `INFO`. The letters are upper case. |
| `rule` | string | The rule that reported the issue. |
| `message` | string | What is wrong, in one line. |
| `explanation` | string | Why it matters. |
| `fix` | string | What to do about it. |
| `line` | number | The line of the issue. It is missing when the issue is about the whole file. |

The broken links are issues as well: their rule is `broken-links`.

## Rule Identifiers

| Rule | What it reports |
|------|-----------------|
| `filename-conventions` | A file name that does not follow the conventions. |
| `sequence-prefix-filename` | A file of a numbered directory without its number. |
| `frontmatter-uid` | A document without a uid, or without the alias of its uid. |
| `frontmatter-fingerprint` | A document whose fingerprint is missing or stale. |
| `frontmatter-required-fields` | A document without one of the required fields. |
| `body-h1` | A body that begins with a top-level heading. |
| `tag-count` | A document with too many tags. |
| `category-naming` | A category that is not in the naming convention. |
| `cross-mode-category` | A document with two doc modes. |
| `directory-category-consistency` | A document whose category does not match its directory. |
| `template-uid-binding` | A template that does not bind the uid. |
| `broken-links` | A link that points at a file that does not exist. |

## Examples

### No issues

```json
{
  "path": "docs",
  "was_auto_detected": true,
  "files_total": 38,
  "error_count": 0,
  "warning_count": 0,
  "info_count": 0,
  "issues": []
}
```

### Errors found

```json
{
  "path": "docs",
  "was_auto_detected": false,
  "files_total": 1,
  "error_count": 2,
  "warning_count": 1,
  "info_count": 0,
  "issues": [
    {
      "file_path": "docs/API Guide.md",
      "severity": "ERROR",
      "rule": "filename-conventions",
      "message": "Filename contains uppercase letters",
      "explanation": "Uppercase letters in filenames cause URL inconsistency and case-sensitivity issues across different platforms.",
      "fix": "Rename to lowercase: api-guide.md"
    },
    {
      "file_path": "docs/API Guide.md",
      "severity": "WARNING",
      "rule": "body-h1",
      "message": "Body begins with a top-level H1",
      "explanation": "The page's H1 is rendered from the `title` frontmatter field.",
      "fix": "Remove the `# Title` line from the body, or change it to `## Title`.",
      "line": 1
    }
  ]
}
```

## Parsing Examples

### Shell (jq)

```bash
# The number of errors and of warnings
ERROR_COUNT=$(jq '[.issues[] | select(.severity=="ERROR")] | length' lint-report.json)
WARNING_COUNT=$(jq '[.issues[] | select(.severity=="WARNING")] | length' lint-report.json)

# The rules that reported the most
jq -r '.issues | group_by(.rule) | map({rule: .[0].rule, count: length}) | sort_by(-.count)[] | "\(.count) \(.rule)"' lint-report.json

# The errors of one file
jq -r '.issues[] | select(.file_path | endswith("guide.md")) | "\(.severity): \(.rule) - \(.message)"' lint-report.json
```

### Python

```python
import json

with open("lint-report.json") as handle:
    report = json.load(handle)

errors = [issue for issue in report["issues"] if issue["severity"] == "ERROR"]
for issue in errors:
    print(f"{issue['file_path']}: {issue['rule']} - {issue['message']}")
```

### JavaScript/Node.js

```javascript
const report = require("./lint-report.json");

const errors = report.issues.filter((issue) => issue.severity === "ERROR");
for (const issue of errors) {
  console.log(`${issue.file_path}: ${issue.rule} - ${issue.message}`);
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

type issue struct {
	FilePath string `json:"file_path"`
	Severity string `json:"severity"`
	Rule     string `json:"rule"`
	Message  string `json:"message"`
}

type report struct {
	ErrorCount   int     `json:"error_count"`
	WarningCount int     `json:"warning_count"`
	Issues       []issue `json:"issues"`
}

func main() {
	data, err := os.ReadFile("lint-report.json")
	if err != nil {
		panic(err)
	}
	var r report
	if err := json.Unmarshal(data, &r); err != nil {
		panic(err)
	}
	fmt.Printf("%d errors, %d warnings\n", r.ErrorCount, r.WarningCount)
	for _, i := range r.Issues {
		if i.Severity == "ERROR" {
			fmt.Printf("%s: %s - %s\n", i.FilePath, i.Rule, i.Message)
		}
	}
}
```

## See Also

- [Lint Rules Reference](./lint-rules.md) - The rules the linter enforces
- [CI/CD Linting](../how-to/ci-cd-linting.md) - The linter in a pipeline
- [Setting Up Linting](../how-to/setup-linting.md) - The pre-commit hook
