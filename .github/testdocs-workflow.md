# TestDocs Issue Fix Workflow

This document describes the standard workflow for fixing documentation generation issues using the `testdocs` directory.

## Purpose

The `testdocs/` directory contains minimal test cases that demonstrate specific issues with documentation generation. This workflow ensures systematic, verifiable fixes.

## Directory Structure

```
testdocs/
├── README.md                           # Root level readme (should be included)
└── sub1/
    ├── inasub.md                       # Test for heading preservation
    ├── sub1.1/
    │   ├── MixedCaseFile.md           # Test for case handling
    │   └── file name with spaces.md   # Test for space handling
    ├── sub1.2/
    │   └── inasubsub.md               # Nested subdirectory test
    └── sub1.3/
        └── index.md                    # Index file handling
```

## Standard Workflow

### 1. Describe the Issue
Clearly articulate what's wrong with the current output:
- What should happen vs. what actually happens
- Which files in `testdocs/` demonstrate the issue
- Expected HTML output behavior

### 2. Verify the Issue
Run the generate command to see current behavior:
```bash
go run ./cmd/docbuilder generate -d ./testdocs -o /tmp/testout
```

Inspect the generated HTML:
```bash
# List generated files
find /tmp/testout -type f -name "*.html"

# Check specific file content
cat /tmp/testout/docs/path/to/file/index.html
# or use grep/less to inspect specific sections
```

### 3. Implement the Fix
Make code changes in the relevant modules:
- `internal/docs/` - Discovery logic
- `internal/hugo/` - Hugo generation and content transformation
- `internal/pipeline/` - Build pipeline
- Other relevant modules

### 4. Re-test the Fix
Re-run the generate command:
```bash
go run ./cmd/docbuilder generate -d ./testdocs -o /tmp/testout
```

Verify the fix by inspecting the updated HTML output.

### 5. Iterate
Repeat steps 3-4 until the issue is resolved.

### 6. Add Test Coverage (Optional but Recommended)
Once the fix is working:
- Add unit tests in the relevant `*_test.go` files
- Consider adding integration tests in `cmd/docbuilder/cli_integration_test.go`
- Update golden files if needed in `test/testdata/`

## Common Commands

### Generate from testdocs
```bash
go run ./cmd/docbuilder generate -d ./testdocs -o /tmp/testout
```

### Inspect specific HTML file
```bash
cat /tmp/testout/docs/sub1/inasub/index.html
```

### Check content transformation
```bash
# View the intermediate Hugo content files (before rendering)
# This requires keeping the staging directory - modify the command or code temporarily
```

### Clean up output
```bash
rm -rf /tmp/testout
```

## Tips

1. **Focus on One Issue**: Fix one problem at a time using this workflow
2. **Incremental Changes**: Make small, focused code changes
3. **Verify Each Step**: Always re-run generate after each change
4. **Compare HTML**: Use `diff` to compare before/after HTML output
5. **Check Logs**: Run with `-v` for verbose output if needed:
   ```bash
   go run ./cmd/docbuilder generate -d ./testdocs -o /tmp/testout -v
   ```

## Adding New Test Cases

When adding new test cases to `testdocs/`:

1. Create minimal example that demonstrates the issue
2. Use clear, descriptive filenames
3. Add a comment at the top of the file explaining what it tests
4. Update this workflow document with the new test case

## Example Session

```bash
# 1. Verify issue exists
go run ./cmd/docbuilder generate -d ./testdocs -o /tmp/testout
cat /tmp/testout/docs/sub1/inasub/index.html | grep -A5 "HEADING"

# 2. Make code changes
# (edit files in internal/)

# 3. Re-test
go run ./cmd/docbuilder generate -d ./testdocs -o /tmp/testout
cat /tmp/testout/docs/sub1/inasub/index.html | grep -A5 "HEADING"

# 4. Verify fix works
# (check that output is now correct)
```
