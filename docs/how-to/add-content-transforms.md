# Add Content Transforms

This guide shows you how to add custom transformations to markdown files before Hugo renders them.

## Overview

DocBuilder uses a **pluggable transform pipeline** that processes markdown files during the content copy stage. Each file goes through a series of transformers that can:

- Modify markdown content
- Add or modify front matter metadata
- Rewrite links
- Convert syntax between formats
- Add custom fields

## Transform Pipeline Architecture

Transformers run in dependency-based order organized by **stages** during the **CopyContent** stage:

```
Stage: parse
  1. Front Matter Parser          - Extract YAML headers

Stage: build
  2. Front Matter Builder         - Add repository metadata

Stage: enrich
  3. Edit Link Injector           - Generate edit URLs

Stage: merge
  4. Front Matter Merge           - Combine metadata

Stage: transform
  5. Relative Link Rewriter       - Fix relative links
  6. [Your Custom Transform]      - Your transformation

Stage: finalize
  7. Strip First Heading          - Remove duplicate titles
  8. Shortcode Escaper            - Escape Hugo shortcodes
  9. Hextra Type Enforcer         - Set page types

Stage: serialize
  10. Serializer                  - Write final YAML + content
```

Within each stage, transforms are ordered by their declared dependencies.

## Creating a Custom Transformer

### Step 1: Create the Transformer File

Create a new file in `internal/hugo/transforms/`:

```go
// internal/hugo/transforms/my_custom.go
package transforms

import (
    "fmt"
    "strings"
)

type MyCustomTransform struct{}

func (t MyCustomTransform) Name() string {
    return "my_custom_transform"
}

func (t MyCustomTransform) Stage() TransformStage {
    return StageTransform // Runs during the transform stage
}

func (t MyCustomTransform) Dependencies() TransformDependencies {
    return TransformDependencies{
        MustRunAfter: []string{"relative_link_rewriter"}, // Run after link rewriting
    }
}

func (t MyCustomTransform) Transform(p PageAdapter) error {
    // Type assert to access page data
    pg, ok := p.(*PageShim)
    if !ok {
        return fmt.Errorf("unexpected page adapter type")
    }
    
    // Transform the markdown content
    pg.Content = strings.ReplaceAll(pg.Content, "{{OLD}}", "{{NEW}}")
    
    return nil
}

func init() {
    // Auto-register on package load
    Register(MyCustomTransform{})
}
```

### Step 2: Build the Project

```bash
go build ./...
```

The transformer is automatically registered and will run on all markdown files.

### Step 3: Test Your Transform

```bash
docbuilder build -c config.yaml -v
```

Check the output files in `content/` to verify your transformation.

## Common Use Cases

### Example 1: Content Replacement

Replace specific text patterns across all files:

```go
type ContentReplacer struct{}

func (t ContentReplacer) Name() string { return "content_replacer" }
func (t ContentReplacer) Stage() TransformStage { return StageTransform }
func (t ContentReplacer) Dependencies() TransformDependencies {
    return TransformDependencies{
        MustRunAfter: []string{"relative_link_rewriter"},
    }
}

func (t ContentReplacer) Transform(p PageAdapter) error {
    pg := p.(*PageShim)
    
    // Replace deprecated syntax
    pg.Content = strings.ReplaceAll(pg.Content, ":warning:", "⚠️")
    pg.Content = strings.ReplaceAll(pg.Content, ":information_source:", "ℹ️")
    
    return nil
}

func init() {
    Register(ContentReplacer{})
}
```

### Example 2: Add Custom Metadata

Inject additional front matter fields:

```go
type CustomMetadata struct{}

func (t CustomMetadata) Name() string { return "custom_metadata" }
func (t CustomMetadata) Stage() TransformStage { return StageBuild }
func (t CustomMetadata) Dependencies() TransformDependencies {
    return TransformDependencies{
        MustRunAfter: []string{"front_matter_builder_v2"},
    }
}

func (t CustomMetadata) Transform(p PageAdapter) error {
    pg := p.(*PageShim)
    
    // Add custom fields via patches
    pg.AddPatch(fmcore.FrontMatterPatch{
        Key:   "last_modified",
        Value: time.Now().Format(time.RFC3339),
    })
    
    pg.AddPatch(fmcore.FrontMatterPatch{
        Key:   "version",
        Value: "1.0",
    })
    
    return nil
}

func init() {
    Register(CustomMetadata{})
}
```

### Example 3: Convert GitHub Alerts to Hugo Shortcodes

Transform GitHub-style alert syntax:

```go
type AdmonitionConverter struct{}

func (t AdmonitionConverter) Name() string { return "admonition_converter" }
func (t AdmonitionConverter) Stage() TransformStage { return StageTransform }
func (t AdmonitionConverter) Dependencies() TransformDependencies {
    return TransformDependencies{
        MustRunAfter: []string{"relative_link_rewriter"},
    }
}

func (t AdmonitionConverter) Transform(p PageAdapter) error {
    pg := p.(*PageShim)
    
    // Convert: > [!NOTE] → {{</* callout type="note" */>}}
    re := regexp.MustCompile(`(?m)^> \[!(NOTE|TIP|WARNING|IMPORTANT|CAUTION)\]\s*\n((?:> .+\n?)+)`)
    
    pg.Content = re.ReplaceAllStringFunc(pg.Content, func(match string) string {
        parts := re.FindStringSubmatch(match)
        alertType := strings.ToLower(parts[1])
        content := strings.ReplaceAll(parts[2], "> ", "")
        return fmt.Sprintf(`{{</* callout type=%q */>}}%s{{</* /callout */>}}`, alertType, content)
    })
    
    return nil
}

func init() {
    Register(AdmonitionConverter{})
}
```

### Example 4: Process Code Blocks

Add metadata or transform code block syntax:

```go
type CodeBlockEnhancer struct{}

func (t CodeBlockEnhancer) Name() string { return "code_block_enhancer" }
func (t CodeBlockEnhancer) Stage() TransformStage { return StageTransform }
func (t CodeBlockEnhancer) Dependencies() TransformDependencies {
    return TransformDependencies{
        MustRunAfter: []string{"relative_link_rewriter"},
    }
}

func (t CodeBlockEnhancer) Transform(p PageAdapter) error {
    pg := p.(*PageShim)
    
    // Add line numbers to all code blocks
    re := regexp.MustCompile("(?s)```(\\w+)\\n(.*?)```")
    pg.Content = re.ReplaceAllString(pg.Content, "```$1 {linenos=true}\n$2```")
    
    return nil
}

func init() {
    Register(CodeBlockEnhancer{})
}
```

### Example 5: Conditional Transforms

Apply transformations based on file properties:

```go
type ConditionalTransform struct{}

func (t ConditionalTransform) Name() string { return "conditional_transform" }
func (t ConditionalTransform) Stage() TransformStage { return StageTransform }
func (t ConditionalTransform) Dependencies() TransformDependencies {
    return TransformDependencies{
        MustRunAfter: []string{"relative_link_rewriter"},
    }
}

func (t ConditionalTransform) Transform(p PageAdapter) error {
    pg := p.(*PageShim)
    
    // Only transform API documentation
    if strings.Contains(pg.FilePath, "/api/") {
        pg.AddPatch(fmcore.FrontMatterPatch{
            Key:   "type",
            Value: "api-reference",
        })
        
        // Add API-specific styling
        pg.Content = "{{</* api-layout */>}}\n" + pg.Content + "\n{{</* /api-layout */>}}"
    }
    
    return nil
}

func init() {
    Register(ConditionalTransform{})
}
```

## PageShim Interface

Your transform receives a `PageAdapter` which you type-assert to `*PageShim`:

```go
type PageShim struct {
    FilePath            string              // Relative file path
    Content             string              // Markdown content (no front matter)
    OriginalFrontMatter map[string]any      // Parsed YAML front matter
    HadFrontMatter      bool                // Whether file had front matter
    Doc                 docs.DocFile        // Full document metadata
    
    // Methods
    AddPatch(patch FrontMatterPatch)        // Add front matter field
    ApplyPatches()                          // Merge patches
    RewriteLinks(content string) string     // Fix relative links
    Serialize() error                       // Write final output
}
```

**Important:** Don't call `Serialize()` in your transform - it's automatically called by the pipeline.

## Setting Transform Stage and Dependencies

Transforms are organized by **stages** and **dependencies** (not priorities):

### Available Stages

| Stage | Purpose | Example Transforms |
|-------|---------|-------------------|
| `StageParse` | Extract/parse source content | Front matter parsing |
| `StageBuild` | Generate base metadata | Repository info, titles |
| `StageEnrich` | Add computed fields | Edit links, custom metadata |
| `StageMerge` | Combine/merge data | Merge user + generated data |
| `StageTransform` | Modify content | Link rewriting, syntax conversion |
| `StageFinalize` | Post-process | Strip headings, escape shortcodes |
| `StageSerialize` | Output generation | Write final YAML + content |

### Declaring Dependencies

Use `Dependencies()` to specify ordering within a stage:

```go
func (t MyTransform) Dependencies() TransformDependencies {
    return TransformDependencies{
        // This transform must run after these transforms
        MustRunAfter: []string{"front_matter_merge", "relative_link_rewriter"},
        
        // This transform must run before these transforms
        MustRunBefore: []string{"front_matter_serialize"},
        
        // Capability flags (for documentation)
        RequiresOriginalFrontMatter: false,
        ModifiesContent:             true,
        ModifiesFrontMatter:         false,
    }
}
```

**Guidelines:**
- **StageParse:** Early processing (parsing, reading)
- **StageBuild-StageEnrich:** Metadata manipulation
- **StageTransform:** Content modification
- **StageFinalize:** Cleanup and validation
- **StageSerialize:** Output serialization

**Within each stage**, transforms are ordered by their dependency declarations using topological sort.

## Controlling Transforms via Configuration

You can enable/disable transforms in `config.yaml`:

```yaml
hugo:
  transforms:
    enable:
      - front_matter_parser
      - front_matter_builder_v2
      - my_custom_transform      # Your custom transform
      - edit_link_injector_v2
      - merge_front_matter
      - relative_link_rewriter
      - serializer
    disable:
      - some_transform_to_skip
```

**Enable mode:** Only listed transforms run (allowlist)  
**Disable mode:** All except listed transforms run (denylist)

If neither is specified, all registered transforms run.

## Best Practices

### 1. Name Transforms Descriptively

The name is used for logging and enable/disable filtering:

```go
func (t MyTransform) Name() string {
    return "descriptive_snake_case_name"
}
```

### 2. Handle Errors Gracefully

Return errors with context:

```go
if err != nil {
    return fmt.Errorf("failed to process %s: %w", pg.FilePath, err)
}
```

### 3. Log Important Operations

Use structured logging for debugging:

```go
import "log/slog"

slog.Debug("Processing file", "path", pg.FilePath, "transform", t.Name())
```

### 4. Don't Modify Raw Content

Only modify `pg.Content` (without front matter). The serializer handles combining it with front matter.

### 5. Test Thoroughly

Create test files in `internal/hugo/transforms/`:

```go
func TestMyCustomTransform(t *testing.T) {
    pg := &PageShim{
        Content: "original content",
    }
    
    transform := MyCustomTransform{}
    err := transform.Transform(pg)
    
    assert.NoError(t, err)
    assert.Equal(t, "expected content", pg.Content)
}
```

### 6. Consider Performance

Transforms run on every markdown file:

```go
// Good: Compile regex once
var alertRegex = regexp.MustCompile(`pattern`)

func (t Transform) Transform(p PageAdapter) error {
    pg := p.(*PageShim)
    pg.Content = alertRegex.ReplaceAllString(pg.Content, "replacement")
    return nil
}

// Bad: Compile regex every time
func (t Transform) Transform(p PageAdapter) error {
    pg := p.(*PageShim)
    re := regexp.MustCompile(`pattern`) // ❌ Slow!
    pg.Content = re.ReplaceAllString(pg.Content, "replacement")
    return nil
}
```

## Debugging Transforms

### Enable Verbose Logging

```bash
docbuilder build -c config.yaml -v
```

### Check Transform Order

The registry logs the execution order at startup with verbose logging enabled.

### Test Individual Transforms

Create a unit test with sample content:

```go
func TestTransformOutput(t *testing.T) {
    input := `---
title: Test
---

# Content

> [!NOTE]
> This is a note
`
    
    pg := &PageShim{Content: input}
    transform := AdmonitionConverter{}
    err := transform.Transform(pg)
    
    require.NoError(t, err)
    assert.Contains(t, pg.Content, "{{ callout }}")
}
```

### Inspect Generated Files

Check the actual output in the Hugo site:

```bash
docbuilder build -c config.yaml
cat site/content/repo/path/file.md
```

## Advanced: External Transforms

For transforms without modifying DocBuilder's code, consider:

### Option 1: Pre-Processing Script

Process files before DocBuilder runs:

```bash
#!/bin/bash
# pre-process.sh
find ./docs -name "*.md" -exec sed -i 's/old/new/g' {} \;
docbuilder build -c config.yaml
```

### Option 2: Post-Processing Hook

Modify Hugo content after DocBuilder generates it:

```bash
#!/bin/bash
docbuilder build -c config.yaml
# Modify files in site/content/
find ./site/content -name "*.md" -exec ./my-transform.py {} \;
```

### Option 3: Hugo Modules/Mounts

Use Hugo's own content transformation features in `hugo.yaml`:

```yaml
module:
  mounts:
    - source: content
      target: content
      transforms:
        - filter: \.md$
          command: my-transform-script
```

## Troubleshooting

### Transform Not Running

**Check registration:**
```go
func init() {
    Register(MyTransform{}) // Must be in init()
}
```

**Verify stage and dependencies:**
```go
func (t MyTransform) Stage() TransformStage {
    return StageTransform // Must return valid stage
}

func (t MyTransform) Dependencies() TransformDependencies {
    return TransformDependencies{} // Define dependencies
}
```

### Content Not Changed

**Check you're modifying the right field:**
```go
pg.Content = newContent  // ✅ Correct
pg.Raw = newContent      // ❌ Wrong - overwritten by serializer
```

**Ensure serializer runs after:**
Your priority must be < 90 (serializer priority).

### Front Matter Issues

**Use patches, not direct modification:**
```go
// ✅ Correct
pg.AddPatch(fmcore.FrontMatterPatch{Key: "field", Value: "value"})

// ❌ Wrong - overwritten by merge
pg.OriginalFrontMatter["field"] = "value"
```

## See Also

- [Architecture: Content Transform Pipeline](../explanation/comprehensive-architecture.md#content-transform-pipeline)
- [Package Architecture: Hugo Transforms](../explanation/package-architecture.md#internalhugotransforms)
- [Configuration Reference: Hugo Transforms](../reference/configuration.md#hugo-transforms)
