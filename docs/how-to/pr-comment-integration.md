# PR Comment Integration Examples

This document provides examples for integrating DocBuilder lint results into pull request comments across different platforms.

## Table of Contents

- [GitHub Actions](#github-actions)
- [GitLab CI](#gitlab-ci)
- [BitBucket Pipelines](#bitbucket-pipelines)
- [Azure DevOps](#azure-devops)
- [Generic Webhook](#generic-webhook)

## GitHub Actions

### Basic Comment

```yaml
- name: Comment PR with Lint Results
  if: github.event_name == 'pull_request'
  uses: actions/github-script@v7
  with:
    script: |
      const fs = require('fs');
      const report = JSON.parse(fs.readFileSync('lint-report.json', 'utf8'));
      
      let body = '## 📝 Documentation Lint Report\n\n';
      body += `**Files scanned:** ${report.summary.total_files}\n`;
      body += `**Errors:** ${report.summary.errors}\n`;
      body += `**Warnings:** ${report.summary.warnings}\n`;
      
      await github.rest.issues.createComment({
        owner: context.repo.owner,
        repo: context.repo.repo,
        issue_number: context.issue.number,
        body: body
      });
```

### Advanced Comment with Issue Details

```yaml
- name: Advanced PR Comment
  if: github.event_name == 'pull_request'
  uses: actions/github-script@v7
  with:
    script: |
      const fs = require('fs');
      const report = JSON.parse(fs.readFileSync('lint-report.json', 'utf8'));
      
      // Build detailed comment
      let comment = '## 📝 Documentation Lint Report\n\n';
      
      // Summary with status emoji
      if (report.summary.errors === 0 && report.summary.warnings === 0) {
        comment += '✅ **All documentation passes linting!**\n\n';
      } else {
        if (report.summary.errors > 0) {
          comment += `❌ **${report.summary.errors} error(s) found** - merge blocked\n`;
        }
        if (report.summary.warnings > 0) {
          comment += `⚠️ **${report.summary.warnings} warning(s) found** - should fix\n`;
        }
        comment += '\n';
      }
      
      comment += `📊 **Summary:** ${report.summary.total_files} files scanned\n\n`;
      
      // Group issues by file
      if (report.issues && report.issues.length > 0) {
        comment += '### Issues Found\n\n';
        
        const byFile = {};
        for (const issue of report.issues) {
          if (!byFile[issue.file]) byFile[issue.file] = [];
          byFile[issue.file].push(issue);
        }
        
        // Show up to 10 files with most severe issues first
        const files = Object.keys(byFile)
          .sort((a, b) => {
            const aErrors = byFile[a].filter(i => i.severity === 'error').length;
            const bErrors = byFile[b].filter(i => i.severity === 'error').length;
            return bErrors - aErrors;
          })
          .slice(0, 10);
        
        for (const file of files) {
          const issues = byFile[file];
          const errorCount = issues.filter(i => i.severity === 'error').length;
          const warnCount = issues.filter(i => i.severity === 'warning').length;
          
          comment += `<details>\n`;
          comment += `<summary><code>${file}</code> - `;
          if (errorCount > 0) comment += `${errorCount} error(s) `;
          if (warnCount > 0) comment += `${warnCount} warning(s)`;
          comment += `</summary>\n\n`;
          
          for (const issue of issues.slice(0, 5)) {
            const emoji = issue.severity === 'error' ? '❌' : '⚠️';
            const lineLink = issue.line > 0 
              ? `[L${issue.line}](https://github.com/${context.repo.owner}/${context.repo.repo}/blob/${context.payload.pull_request.head.sha}/${issue.file}#L${issue.line})`
              : 'File-level';
            
            comment += `${emoji} **${issue.rule}** (${lineLink})\n`;
            comment += `> ${issue.message}\n`;
            
            if (issue.suggestion) {
              comment += `> 💡 Suggestion: \`${issue.suggestion}\`\n`;
            }
            comment += '\n';
          }
          
          if (issues.length > 5) {
            comment += `... and ${issues.length - 5} more issue(s)\n`;
          }
          
          comment += `</details>\n\n`;
        }
        
        if (Object.keys(byFile).length > 10) {
          comment += `*... and ${Object.keys(byFile).length - 10} more file(s) with issues*\n\n`;
        }
      }
      
      // Broken links section
      if (report.broken_links && report.broken_links.length > 0) {
        comment += `### 🔗 Broken Links (${report.broken_links.length})\n\n`;
        
        for (const link of report.broken_links.slice(0, 10)) {
          const lineLink = `[${link.source_file}:${link.line}](https://github.com/${context.repo.owner}/${context.repo.repo}/blob/${context.payload.pull_request.head.sha}/${link.source_file}#L${link.line})`;
          comment += `- ${lineLink}: \`${link.target}\`\n`;
          comment += `  *${link.error}*\n`;
        }
        
        if (report.broken_links.length > 10) {
          comment += `\n*... and ${report.broken_links.length - 10} more broken link(s)*\n`;
        }
        comment += '\n';
      }
      
      // Instructions
      comment += '---\n\n';
      comment += '### How to Fix\n\n';
      comment += '```bash\n';
      comment += '# Review all issues\n';
      comment += 'docbuilder lint\n\n';
      comment += '# Auto-fix where possible\n';
      comment += 'docbuilder lint --fix\n\n';
      comment += '# Preview changes without applying\n';
      comment += 'docbuilder lint --fix --dry-run\n';
      comment += '```\n\n';
      comment += '*💡 Tip: The pre-commit hook will prevent future issues*\n';
      comment += '```bash\n';
      comment += 'docbuilder lint install-hook\n';
      comment += '```\n';
      
      // Post or update comment
      const { data: comments } = await github.rest.issues.listComments({
        owner: context.repo.owner,
        repo: context.repo.repo,
        issue_number: context.issue.number
      });
      
      const botComment = comments.find(c => 
        c.user.type === 'Bot' && 
        c.body.includes('Documentation Lint Report')
      );
      
      if (botComment) {
        // Update existing comment
        await github.rest.issues.updateComment({
          owner: context.repo.owner,
          repo: context.repo.repo,
          comment_id: botComment.id,
          body: comment
        });
      } else {
        // Create new comment
        await github.rest.issues.createComment({
          owner: context.repo.owner,
          repo: context.repo.repo,
          issue_number: context.issue.number,
          body: comment
        });
      }
```

### Minimal Comment (Errors Only)

```yaml
- name: Minimal Error Comment
  if: github.event_name == 'pull_request' && steps.lint.outputs.exit_code == '2'
  uses: actions/github-script@v7
  with:
    script: |
      const fs = require('fs');
      const report = JSON.parse(fs.readFileSync('lint-report.json', 'utf8'));
      
      const errors = report.issues.filter(i => i.severity === 'error');
      
      let comment = `❌ **Documentation linting failed** - ${errors.length} error(s) found\n\n`;
      
      for (const error of errors.slice(0, 5)) {
        comment += `- \`${error.file}\`: ${error.message}\n`;
      }
      
      if (errors.length > 5) {
        comment += `\n*... and ${errors.length - 5} more error(s)*\n`;
      }
      
      comment += `\nRun \`docbuilder lint --fix\` to resolve automatically.\n`;
      
      await github.rest.issues.createComment({
        owner: context.repo.owner,
        repo: context.repo.repo,
        issue_number: context.issue.number,
        body: comment
      });
```

## GitLab CI

### Basic MR Comment

```yaml
lint:docs:comment:
  stage: lint
  image: alpine:latest
  needs:
    - job: lint:docs
      artifacts: true
  
  script:
    - apk add --no-cache curl jq
    - |
      ERRORS=$(jq -r '.summary.errors' lint-report.json)
      WARNINGS=$(jq -r '.summary.warnings' lint-report.json)
      TOTAL=$(jq -r '.summary.total_files' lint-report.json)
      
      COMMENT="## 📝 Documentation Lint Report\n\n"
      COMMENT="${COMMENT}**Files scanned:** ${TOTAL}\n"
      COMMENT="${COMMENT}**Errors:** ${ERRORS}\n"
      COMMENT="${COMMENT}**Warnings:** ${WARNINGS}\n"
      
      if [ "${ERRORS}" -gt 0 ]; then
        COMMENT="${COMMENT}\n❌ Linting failed - see details in pipeline artifacts"
      else
        COMMENT="${COMMENT}\n✅ All documentation passes linting!"
      fi
      
      # Post to MR
      curl --request POST \
        --header "PRIVATE-TOKEN: ${GITLAB_API_TOKEN}" \
        --header "Content-Type: application/json" \
        --data "{\"body\": \"${COMMENT}\"}" \
        "${CI_API_V4_URL}/projects/${CI_PROJECT_ID}/merge_requests/${CI_MERGE_REQUEST_IID}/notes"
  
  rules:
    - if: $CI_PIPELINE_SOURCE == "merge_request_event"
  
  allow_failure: true
```

### Detailed GitLab Comment

```yaml
lint:docs:comment:
  stage: lint
  image: alpine:latest
  needs:
    - job: lint:docs
      artifacts: true
  
  script:
    - apk add --no-cache curl jq
    - |
      # Build detailed comment with issue grouping
      COMMENT=$(cat <<'EOF'
      ## 📝 Documentation Lint Report
      
      EOF
      )
      
      TOTAL=$(jq -r '.summary.total_files' lint-report.json)
      ERRORS=$(jq -r '.summary.errors' lint-report.json)
      WARNINGS=$(jq -r '.summary.warnings' lint-report.json)
      
      COMMENT="${COMMENT}**Summary:** ${TOTAL} files scanned\n\n"
      
      if [ "${ERRORS}" -gt 0 ]; then
        COMMENT="${COMMENT}❌ **${ERRORS} error(s)** - merge blocked\n"
      fi
      
      if [ "${WARNINGS}" -gt 0 ]; then
        COMMENT="${COMMENT}⚠️ **${WARNINGS} warning(s)** - should fix\n"
      fi
      
      # Add top 5 issues
      ISSUES=$(jq -r '.issues[:5] | .[] | 
        "- **\(.rule)** in `\(.file)` (L\(.line)): \(.message)"' 
        lint-report.json)
      
      if [ -n "${ISSUES}" ]; then
        COMMENT="${COMMENT}\n### Issues\n\n${ISSUES}\n"
      fi
      
      COMMENT="${COMMENT}\n---\n**Fix:** \`docbuilder lint --fix\`\n"
      
      # Escape for JSON
      COMMENT_ESCAPED=$(echo "${COMMENT}" | jq -Rs .)
      
      # Post to MR
      curl --request POST \
        --header "PRIVATE-TOKEN: ${GITLAB_API_TOKEN}" \
        --header "Content-Type: application/json" \
        --data "{\"body\": ${COMMENT_ESCAPED}}" \
        "${CI_API_V4_URL}/projects/${CI_PROJECT_ID}/merge_requests/${CI_MERGE_REQUEST_IID}/notes"
  
  rules:
    - if: $CI_PIPELINE_SOURCE == "merge_request_event"
  
  allow_failure: true
```

## BitBucket Pipelines

```yaml
pipelines:
  pull-requests:
    '**':
      - step:
          name: Lint Documentation
          image: golang:1.21
          script:
            - go install git.home.luguber.info/inful/docbuilder/cmd/docbuilder@latest
            - docbuilder lint --format=json > lint-report.json || true
            
            # Parse results
            - ERRORS=$(jq -r '.summary.errors' lint-report.json)
            - WARNINGS=$(jq -r '.summary.warnings' lint-report.json)
            
            # Build comment
            - |
              COMMENT="## Documentation Lint Report\n\n"
              COMMENT="${COMMENT}Errors: ${ERRORS}, Warnings: ${WARNINGS}\n\n"
              if [ "${ERRORS}" -gt 0 ]; then
                COMMENT="${COMMENT}❌ Linting failed\n"
              fi
              
              # Post comment via API
              curl -X POST \
                -u "${BB_AUTH_STRING}" \
                -H "Content-Type: application/json" \
                -d "{\"content\": {\"raw\": \"${COMMENT}\"}}" \
                "https://api.bitbucket.org/2.0/repositories/${BITBUCKET_REPO_FULL_NAME}/pullrequests/${BITBUCKET_PR_ID}/comments"
            
            # Fail if errors found
            - test "${ERRORS}" -eq 0
          
          artifacts:
            - lint-report.json
```

## Azure DevOps

```yaml
- task: Bash@3
  displayName: 'Lint Documentation'
  inputs:
    targetType: 'inline'
    script: |
      go install git.home.luguber.info/inful/docbuilder/cmd/docbuilder@latest
      docbuilder lint --format=json > lint-report.json || true

- task: Bash@3
  displayName: 'Comment PR with Results'
  condition: eq(variables['Build.Reason'], 'PullRequest')
  inputs:
    targetType: 'inline'
    script: |
      ERRORS=$(jq -r '.summary.errors' lint-report.json)
      WARNINGS=$(jq -r '.summary.warnings' lint-report.json)
      
      COMMENT="## Documentation Lint Report\n\n"
      COMMENT="${COMMENT}Errors: ${ERRORS}, Warnings: ${WARNINGS}\n"
      
      # Post to PR using Azure DevOps API
      az repos pr comment create \
        --org "$(System.TeamFoundationCollectionUri)" \
        --project "$(System.TeamProject)" \
        --pull-request-id "$(System.PullRequest.PullRequestId)" \
        --repository-id "$(Build.Repository.ID)" \
        --content "${COMMENT}"
  env:
    AZURE_DEVOPS_EXT_PAT: $(System.AccessToken)
```

## Generic Webhook

For platforms without native integrations, post results to a webhook:

```bash
#!/bin/bash

# Run linter
docbuilder lint --format=json > lint-report.json

# Post to webhook
curl -X POST https://your-webhook.example.com/lint-results \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${WEBHOOK_TOKEN}" \
  --data @lint-report.json
```

## Best Practices

### 1. Update Instead of Duplicate

Always update existing comments instead of creating new ones:

```javascript
const existingComment = comments.find(c => 
  c.user.type === 'Bot' && 
  c.body.includes('Documentation Lint Report')
);

if (existingComment) {
  await github.rest.issues.updateComment({...});
} else {
  await github.rest.issues.createComment({...});
}
```

### 2. Collapsible Sections

Use `<details>` tags for long issue lists:

```markdown
<details>
<summary>📄 docs/api-guide.md - 5 issues</summary>

- Error 1
- Error 2
...

</details>
```

### 3. Link to Source

Link directly to the problematic lines:

```javascript
const lineLink = `[L${issue.line}](https://github.com/${owner}/${repo}/blob/${sha}/${file}#L${issue.line})`;
```

### 4. Rate Limiting

Avoid posting comments on every push to a PR:

```yaml
# Only comment once per PR
- name: Check for existing comment
  id: check
  run: |
    EXISTING=$(gh pr view ${{ github.event.pull_request.number }} \
      --json comments --jq '.comments[] | select(.body | contains("Documentation Lint Report"))')
    echo "has_comment=$([[ -n "$EXISTING" ]] && echo true || echo false)" >> $GITHUB_OUTPUT

- name: Comment PR
  if: steps.check.outputs.has_comment == 'false'
  ...
```

### 5. Conditional Posting

Only post when there are issues:

```yaml
- name: Comment PR
  if: steps.parse.outputs.errors != '0' || steps.parse.outputs.warnings != '0'
  ...
```

### 6. Clear Remediation Steps

Always include actionable instructions:

```markdown
### How to Fix

1. Run locally: `docbuilder lint`
2. Auto-fix: `docbuilder lint --fix`
3. Review changes and commit

Or install the pre-commit hook:
```bash
docbuilder lint install-hook
```
```

### 7. Status Emojis

Use consistent emojis for quick visual scanning:
- ✅ Success
- ❌ Errors (blocking)
- ⚠️ Warnings (non-blocking)
- 🔗 Broken links
- 💡 Suggestions
- 📊 Summary stats

## Testing Comments

Test your comment formatting locally before deploying:

```bash
# Generate test report
docbuilder lint --format=json > test-report.json

# Test comment generation
node test-comment-script.js

# Validate markdown
npx markdownlint comment.md
```

## Security Considerations

1. **Token Permissions**: Use minimal required permissions
   - GitHub: `pull-requests: write` only
   - GitLab: `api` scope with project access

2. **Sensitive Data**: Never include secrets in comments
   - Sanitize file paths if they contain usernames
   - Don't expose internal URLs

3. **Rate Limits**: Respect platform API rate limits
   - Cache comment existence checks
   - Batch operations when possible

4. **Spam Prevention**: Limit comment size and frequency
   - Cap issue display (e.g., max 10 files, 5 issues per file)
   - Update existing comments instead of creating new ones
