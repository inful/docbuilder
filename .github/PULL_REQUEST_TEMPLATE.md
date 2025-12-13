## Description

<!-- Provide a brief description of the changes in this PR -->

## Type of Change

- [ ] Bug fix (non-breaking change which fixes an issue)
- [ ] New feature (non-breaking change which adds functionality)
- [ ] Breaking change (fix or feature that would cause existing functionality to not work as expected)
- [ ] Documentation update
- [ ] Refactoring (no functional changes)
- [ ] Performance improvement
- [ ] Test coverage improvement

## Changes Made

<!-- List the main changes in bullet points -->

-
-
-

## Testing

### Test Coverage

- [ ] Unit tests added/updated
- [ ] Integration tests added/updated
- [ ] Golden tests added/updated (if applicable)
- [ ] Manual testing performed

### Golden Test Checklist

<!-- If changes affect generated Hugo sites, front matter, or content processing -->

- [ ] Golden files reviewed (not blindly regenerated)
- [ ] New test case added for new feature (if applicable)
- [ ] Golden files updated with `-update-golden` flag
- [ ] Changes to golden files committed in same PR
- [ ] HTML rendering verified (if applicable)

### Test Commands Run

<!-- Example commands used to verify changes -->

```bash
# List commands used for testing
go test ./... -v
# go test ./test/integration -run=TestGolden_NewFeature -update-golden
```

## Documentation

- [ ] README updated (if user-facing changes)
- [ ] Code comments added/updated
- [ ] Architecture documentation updated (if structural changes)
- [ ] CHANGELOG.md updated

## Performance Impact

<!-- Does this change affect build performance? -->

- [ ] No performance impact
- [ ] Performance improved
- [ ] Performance regression (justified below)

<!-- If performance changed, provide details -->

## Breaking Changes

<!-- If breaking changes were introduced, describe the migration path -->

**Migration Guide:**

<!-- How should users update their code/config to work with these changes? -->

## Checklist

- [ ] Code follows the project's style guide
- [ ] Self-review of code performed
- [ ] Comments added in hard-to-understand areas
- [ ] No new warnings generated
- [ ] Tests pass locally (`go test ./...`)
- [ ] Golden tests pass (`go test ./test/integration -run=TestGolden`)
- [ ] Linter passes (`golangci-lint run`)
- [ ] Commit messages follow conventional commits format

## Related Issues

<!-- Link related issues using keywords: Fixes #123, Relates to #456 -->

Fixes #
Relates to #

## Screenshots/Examples

<!-- If applicable, add screenshots or example output -->

## Additional Notes

<!-- Any additional information that reviewers should know -->
