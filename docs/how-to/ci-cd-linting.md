---
aliases:
  - /_uid/a89ff86e-31ab-43b5-b751-05c37768b0ba/
categories:
  - how-to
date: 2025-12-29T00:00:00Z
fingerprint: 353995dbe6d099953fd10c0cc256ebea079f85f0014d193dc476df4599846209
lastmod: "2026-01-22"
tags:
  - ci-cd
  - linting
  - automation
  - github-actions
  - gitlab-ci
title: 'How To: CI/CD Linting Integration'
uid: a89ff86e-31ab-43b5-b751-05c37768b0ba
---

# CI/CD Linting Integration

This guide shows how to integrate documentation linting into your CI/CD pipeline for automated validation.

## Overview

CI/CD linting provides:
- **Automated validation**: Catch issues before merge
- **Consistent enforcement**: All PRs validated equally
- **Visible feedback**: Clear error messages in PR comments
- **Quality gates**: Block merges if docs fail validation

## Supported Platforms

- [GitHub Actions](#github-actions)
- [GitLab CI](#gitlab-ci)
- [Jenkins](#jenkins)
- [CircleCI](#circleci)
- [Generic CI](#generic-ci-systems)

---

## GitHub Actions

### Basic Workflow

Create `.github/workflows/lint-docs.yml`:

```yaml
name: Lint Documentation

on:
  pull_request:
    paths:
      - 'docs/**'
      - '**.md'
  push:
    branches:
      - main

jobs:
  lint:
    runs-on: ubuntu-latest
    
    steps:
      - name: Checkout code
        uses: actions/checkout@v4
      
      - name: Setup Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.21'
      
      - name: Install DocBuilder
        run: |
          go install github.com/your-org/docbuilder/cmd/docbuilder@latest
          echo "$(go env GOPATH)/bin" >> $GITHUB_PATH
      
      - name: Lint Documentation
        run: |
          docbuilder lint --format=json > lint-report.json
          docbuilder lint  # Human-readable output
      
      - name: Upload Lint Report
        if: always()
        uses: actions/upload-artifact@v3
        with:
          name: lint-report
          path: lint-report.json
          retention-days: 30
```

**Features**:
- ✅ Runs on PRs affecting docs
- ✅ Installs DocBuilder from source
- ✅ Generates both JSON and text reports
- ✅ Uploads artifacts for later review

### Advanced: PR Comments

Post lint results directly on PR:

```yaml
name: Lint Documentation with PR Comments

on:
  pull_request:
    paths:
      - 'docs/**'
      - '**.md'

jobs:
  lint:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      pull-requests: write  # Required for PR comments
    
    steps:
      - uses: actions/checkout@v4
      
      - name: Setup Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.21'
      
      - name: Install DocBuilder
        run: |
          go install github.com/your-org/docbuilder/cmd/docbuilder@latest
          echo "$(go env GOPATH)/bin" >> $GITHUB_PATH
      
      - name: Lint Documentation
        id: lint
        run: |
          set +e  # Don't fail on linting errors
          docbuilder lint --format=json > lint-report.json
          LINT_EXIT=$?
          echo "exit_code=$LINT_EXIT" >> $GITHUB_OUTPUT
          
          # Generate summary
          ERROR_COUNT=$(jq '[.issues[] | select(.severity=="error")] | length' lint-report.json)
          WARNING_COUNT=$(jq '[.issues[] | select(.severity=="warning")] | length' lint-report.json)
          
          echo "errors=$ERROR_COUNT" >> $GITHUB_OUTPUT
          echo "warnings=$WARNING_COUNT" >> $GITHUB_OUTPUT
          
          exit $LINT_EXIT
      
      - name: Comment PR - Success
        if: steps.lint.outputs.exit_code == '0'
        uses: actions/github-script@v6
        with:
          script: |
            github.rest.issues.createComment({
              issue_number: context.issue.number,
              owner: context.repo.owner,
              repo: context.repo.repo,
              body: '✅ **Documentation linting passed!**\n\nAll files meet linting standards.'
            })
      
      - name: Comment PR - Warnings
        if: steps.lint.outputs.exit_code == '1'
        uses: actions/github-script@v6
        with:
          script: |
            const warnings = ${{ steps.lint.outputs.warnings }};
            github.rest.issues.createComment({
              issue_number: context.issue.number,
              owner: context.repo.owner,
              repo: context.repo.repo,
              body: `⚠️ **Documentation has ${warnings} warning(s)**\n\nConsider fixing before merge. Run \`docbuilder lint --fix\` locally.`
            })
      
      - name: Comment PR - Errors
        if: steps.lint.outputs.exit_code == '2'
        uses: actions/github-script@v6
        with:
          script: |
            const fs = require('fs');
            const report = JSON.parse(fs.readFileSync('lint-report.json', 'utf8'));
            const errors = report.issues.filter(i => i.severity === 'error');
            
            let comment = `❌ **Documentation linting failed with ${errors.length} error(s)**\n\n`;
            comment += '### Errors\n\n';
            
            errors.slice(0, 10).forEach(issue => {
              comment += `- **${issue.file}**\n`;
              comment += `  \`${issue.message}\`\n\n`;
            });
            
            if (errors.length > 10) {
              comment += `\n_...and ${errors.length - 10} more errors. Download full report from artifacts._\n`;
            }
            
            comment += '\n**How to fix**:\n';
            comment += '```bash\n';
            comment += 'docbuilder lint --fix\n';
            comment += 'git add -A\n';
            comment += 'git commit -m "docs: fix linting issues"\n';
            comment += '```';
            
            github.rest.issues.createComment({
              issue_number: context.issue.number,
              owner: context.repo.owner,
              repo: context.repo.repo,
              body: comment
            })
      
      - name: Upload Report
        if: always()
        uses: actions/upload-artifact@v3
        with:
          name: lint-report
          path: lint-report.json
      
      - name: Fail if errors
        if: steps.lint.outputs.exit_code == '2'
        run: exit 1
```

**Features**:
- ✅ Posts summary comment on PR
- ✅ Shows first 10 errors inline
- ✅ Provides fix instructions
- ✅ Fails workflow if errors found
- ✅ Allows warnings without blocking

### Auto-Fix on Push

Automatically fix issues and commit:

```yaml
name: Auto-Fix Documentation

on:
  push:
    branches:
      - main
    paths:
      - 'docs/**'
      - '**.md'

jobs:
  auto-fix:
    runs-on: ubuntu-latest
    permissions:
      contents: write  # Required to push commits
    
    steps:
      - uses: actions/checkout@v4
        with:
          token: ${{ secrets.GITHUB_TOKEN }}
      
      - name: Setup Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.21'
      
      - name: Install DocBuilder
        run: |
          go install github.com/your-org/docbuilder/cmd/docbuilder@latest
          echo "$(go env GOPATH)/bin" >> $GITHUB_PATH
      
      - name: Auto-Fix Issues
        run: |
          docbuilder lint --fix --yes
      
      - name: Commit Fixes
        run: |
          git config user.name "docbuilder-bot"
          git config user.email "bot@example.com"
          
          if [[ -n $(git status -s) ]]; then
            git add -A
            git commit -m "docs: auto-fix linting issues [skip ci]"
            git push
          else
            echo "No fixes needed"
          fi
```

⚠️ **Warning**: Auto-fix on push can create unexpected commits. Consider using only for specific branches or requiring review.

### Lint Only Changed Files

Optimize CI by linting only changed files:

```yaml
- name: Get Changed Files
  id: changed-files
  run: |
    git fetch origin ${{ github.base_ref }}
    CHANGED=$(git diff --name-only origin/${{ github.base_ref }}...HEAD | grep -E '\.(md|markdown)$' || echo "")
    echo "files=$CHANGED" >> $GITHUB_OUTPUT
    echo "$CHANGED" > changed_files.txt

- name: Lint Changed Files
  if: steps.changed-files.outputs.files != ''
  run: |
    cat changed_files.txt | xargs docbuilder lint
```

---

## GitLab CI

### Basic Pipeline

Add to `.gitlab-ci.yml`:

```yaml
stages:
  - test

lint-docs:
  stage: test
  image: golang:1.21
  
  before_script:
    - go install github.com/your-org/docbuilder/cmd/docbuilder@latest
    - export PATH=$PATH:$(go env GOPATH)/bin
  
  script:
    - docbuilder lint --format=json | tee lint-report.json
    - docbuilder lint  # Human-readable output
  
  artifacts:
    when: always
    paths:
      - lint-report.json
    reports:
      junit: lint-report.json
    expire_in: 30 days
  
  only:
    changes:
      - docs/**
      - '**/*.md'
```

### MR Comments with API

Post lint results to merge request:

```yaml
lint-docs-with-comments:
  stage: test
  image: golang:1.21
  
  before_script:
    - go install github.com/your-org/docbuilder/cmd/docbuilder@latest
    - export PATH=$PATH:$(go env GOPATH)/bin
    - apt-get update && apt-get install -y jq curl
  
  script:
    - |
      set +e
      docbuilder lint --format=json > lint-report.json
      LINT_EXIT=$?
      
      ERROR_COUNT=$(jq '[.issues[] | select(.severity=="error")] | length' lint-report.json)
      WARNING_COUNT=$(jq '[.issues[] | select(.severity=="warning")] | length' lint-report.json)
      
      if [ "$LINT_EXIT" -eq 2 ]; then
        STATUS="❌ **Linting failed** with $ERROR_COUNT error(s)"
      elif [ "$LINT_EXIT" -eq 1 ]; then
        STATUS="⚠️ **Linting passed** with $WARNING_COUNT warning(s)"
      else
        STATUS="✅ **Linting passed**"
      fi
      
      COMMENT="$STATUS\n\nRun \`docbuilder lint --fix\` to auto-fix issues."
      
      # Post comment using GitLab API
      curl --request POST \
        --header "PRIVATE-TOKEN: $CI_JOB_TOKEN" \
        --header "Content-Type: application/json" \
        --data "{\"body\": \"$COMMENT\"}" \
        "$CI_API_V4_URL/projects/$CI_PROJECT_ID/merge_requests/$CI_MERGE_REQUEST_IID/notes"
      
      exit $LINT_EXIT
  
  artifacts:
    when: always
    paths:
      - lint-report.json
  
  only:
    - merge_requests
```

### Auto-Fix on Main

```yaml
auto-fix-docs:
  stage: fix
  image: golang:1.21
  
  before_script:
    - go install github.com/your-org/docbuilder/cmd/docbuilder@latest
    - export PATH=$PATH:$(go env GOPATH)/bin
    - git config user.name "DocBuilder Bot"
    - git config user.email "bot@example.com"
  
  script:
    - docbuilder lint --fix --yes
    - |
      if [[ -n $(git status -s) ]]; then
        git add -A
        git commit -m "docs: auto-fix linting issues [skip ci]"
        git push "https://oauth2:${CI_JOB_TOKEN}@${CI_SERVER_HOST}/${CI_PROJECT_PATH}.git" HEAD:main
      fi
  
  only:
    - main
  
  when: on_success
```

---

## Jenkins

### Pipeline Configuration

Create `Jenkinsfile`:

```groovy
pipeline {
    agent any
    
    environment {
        GOPATH = "${WORKSPACE}/go"
        PATH = "${PATH}:${GOPATH}/bin"
    }
    
    stages {
        stage('Setup') {
            steps {
                sh 'go install github.com/your-org/docbuilder/cmd/docbuilder@latest'
            }
        }
        
        stage('Lint Documentation') {
            steps {
                script {
                    def lintStatus = sh(
                        script: 'docbuilder lint --format=json > lint-report.json && docbuilder lint',
                        returnStatus: true
                    )
                    
                    archiveArtifacts artifacts: 'lint-report.json', allowEmptyArchive: false
                    
                    if (lintStatus == 2) {
                        error("Documentation linting failed with errors")
                    } else if (lintStatus == 1) {
                        unstable("Documentation has warnings")
                    }
                }
            }
        }
    }
    
    post {
        always {
            publishHTML([
                allowMissing: false,
                alwaysLinkToLastBuild: true,
                keepAll: true,
                reportDir: '.',
                reportFiles: 'lint-report.json',
                reportName: 'Lint Report'
            ])
        }
    }
}
```

---

## CircleCI

### Configuration

Create `.circleci/config.yml`:

```yaml
version: 2.1

jobs:
  lint-docs:
    docker:
      - image: cimg/go:1.21
    
    steps:
      - checkout
      
      - restore_cache:
          keys:
            - go-mod-v1-{{ checksum "go.sum" }}
      
      - run:
          name: Install DocBuilder
          command: |
            go install github.com/your-org/docbuilder/cmd/docbuilder@latest
      
      - save_cache:
          key: go-mod-v1-{{ checksum "go.sum" }}
          paths:
            - "/home/circleci/go/pkg/mod"
      
      - run:
          name: Lint Documentation
          command: |
            docbuilder lint --format=json > lint-report.json
            docbuilder lint
      
      - store_artifacts:
          path: lint-report.json
          destination: lint-report
      
      - store_test_results:
          path: lint-report.json

workflows:
  version: 2
  lint:
    jobs:
      - lint-docs:
          filters:
            branches:
              ignore:
                - gh-pages
```

---

## Generic CI Systems

### Docker-Based Approach

For any CI that supports Docker:

```dockerfile
# Dockerfile.lint
FROM golang:1.21-alpine

RUN apk add --no-cache git

RUN go install github.com/your-org/docbuilder/cmd/docbuilder@latest

WORKDIR /workspace

ENTRYPOINT ["docbuilder", "lint"]
```

**Build image**:
```bash
docker build -f Dockerfile.lint -t docbuilder-lint:latest .
```

**Use in CI**:
```bash
# Generic CI script
docker run --rm -v $(pwd):/workspace docbuilder-lint:latest --format=json > lint-report.json
```

### Shell Script Approach

For CI without Docker support:

```bash
#!/bin/bash
# lint-docs.sh

set -e

# Install Go if needed
if ! command -v go &> /dev/null; then
    echo "Installing Go..."
    wget https://go.dev/dl/go1.21.0.linux-amd64.tar.gz
    tar -C /usr/local -xzf go1.21.0.linux-amd64.tar.gz
    export PATH=$PATH:/usr/local/go/bin
fi

# Install DocBuilder
echo "Installing DocBuilder..."
go install github.com/your-org/docbuilder/cmd/docbuilder@latest
export PATH=$PATH:$(go env GOPATH)/bin

# Run linting
echo "Linting documentation..."
docbuilder lint --format=json > lint-report.json
docbuilder lint

# Check exit code
LINT_EXIT=$?
if [ $LINT_EXIT -eq 2 ]; then
    echo "❌ Linting failed with errors"
    exit 1
elif [ $LINT_EXIT -eq 1 ]; then
    echo "⚠️ Linting passed with warnings"
    exit 0
else
    echo "✅ Linting passed"
    exit 0
fi
```

---

## Advanced Patterns

### Parallel Linting

Lint multiple directories in parallel:

```yaml
# GitHub Actions
jobs:
  lint-api-docs:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - # ... setup steps
      - run: docbuilder lint docs/api/
  
  lint-guides:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - # ... setup steps
      - run: docbuilder lint docs/guides/
  
  lint-reference:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - # ... setup steps
      - run: docbuilder lint docs/reference/
```

### Conditional Enforcement

Different rules for different branches:

```yaml
- name: Lint (Strict on Main)
  if: github.ref == 'refs/heads/main'
  run: docbuilder lint  # Fail on any error

- name: Lint (Relaxed on Feature Branches)
  if: github.ref != 'refs/heads/main'
  run: docbuilder lint || true  # Don't block
```

### Scheduled Deep Scans

Run comprehensive linting weekly:

```yaml
name: Weekly Documentation Audit

on:
  schedule:
    - cron: '0 2 * * 0'  # Sunday 2am

jobs:
  audit:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - # ... setup steps
      
      - name: Full Lint
        run: |
          docbuilder lint --format=json > weekly-audit.json
      
      - name: Generate Report
        run: |
          jq -r '.issues[] | "\(.severity): \(.file) - \(.message)"' weekly-audit.json > weekly-report.txt
      
      - name: Email Report
        uses: dawidd6/action-send-mail@v3
        with:
          server_address: smtp.example.com
          server_port: 465
          username: ${{ secrets.MAIL_USERNAME }}
          password: ${{ secrets.MAIL_PASSWORD }}
          subject: Weekly Documentation Audit
          to: docs-team@example.com
          from: CI Bot
          attachments: weekly-report.txt
```

---

## Performance Optimization

### Cache Dependencies

```yaml
# GitHub Actions
- name: Cache Go modules
  uses: actions/cache@v3
  with:
    path: |
      ~/.cache/go-build
      ~/go/pkg/mod
    key: ${{ runner.os }}-go-${{ hashFiles('**/go.sum') }}
    restore-keys: |
      ${{ runner.os }}-go-
```

### Incremental Linting

Only lint changed files:

```yaml
- name: Get changed markdown files
  id: changed-files
  uses: tj-actions/changed-files@v39
  with:
    files: |
      **/*.md
      **/*.markdown

- name: Lint changed files
  if: steps.changed-files.outputs.any_changed == 'true'
  run: |
    echo "${{ steps.changed-files.outputs.all_changed_files }}" | xargs docbuilder lint
```

### Matrix Builds

Test against multiple DocBuilder versions:

```yaml
jobs:
  lint:
    strategy:
      matrix:
        docbuilder-version: ['latest', 'v1.0.0', 'v1.1.0']
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Install DocBuilder ${{ matrix.docbuilder-version }}
        run: |
          go install github.com/your-org/docbuilder/cmd/docbuilder@${{ matrix.docbuilder-version }}
      - name: Lint
        run: docbuilder lint
```

---

## Monitoring and Metrics

### Track Lint Success Rate

```yaml
- name: Record Metrics
  if: always()
  run: |
    LINT_EXIT=$?
    curl -X POST https://metrics.example.com/api/lint \
      -H "Content-Type: application/json" \
      -d "{
        \"repo\": \"${{ github.repository }}\",
        \"pr\": \"${{ github.event.pull_request.number }}\",
        \"exit_code\": $LINT_EXIT,
        \"timestamp\": \"$(date -u +%Y-%m-%dT%H:%M:%SZ)\"
      }"
```

### Dashboard Integration

Export metrics to dashboard tools:

```yaml
- name: Export to DataDog
  if: always()
  run: |
    ERROR_COUNT=$(jq '[.issues[] | select(.severity=="error")] | length' lint-report.json)
    WARNING_COUNT=$(jq '[.issues[] | select(.severity=="warning")] | length' lint-report.json)
    
    echo "lint.errors:$ERROR_COUNT|g|#repo:${{ github.repository }}" | nc -u -w1 datadog-agent 8125
    echo "lint.warnings:$WARNING_COUNT|g|#repo:${{ github.repository }}" | nc -u -w1 datadog-agent 8125
```

---

## Troubleshooting CI

### Issue: CI Timeout

**Solution**: Lint only changed files or increase timeout

```yaml
- name: Lint with timeout
  timeout-minutes: 10
  run: docbuilder lint
```

### Issue: False Positives in CI

**Solution**: Ensure consistent environment

```yaml
- name: Normalize line endings
  run: git config core.autocrlf false

- name: Lint
  run: docbuilder lint
```

### Issue: Secrets in Error Messages

**Solution**: Sanitize output

```yaml
- name: Lint
  run: |
    docbuilder lint 2>&1 | sed 's/${{ secrets.TOKEN }}/***REDACTED***/g'
```

---

## Best Practices

1. **Start permissive**: Allow warnings initially, tighten later
2. **Fast feedback**: Lint only changed files in PRs
3. **Clear messages**: Use PR comments for actionable feedback
4. **Auto-fix carefully**: Only on trusted branches
5. **Monitor trends**: Track lint success rates over time
6. **Document exceptions**: Explain any `--no-verify` usage
7. **Version pin**: Use specific DocBuilder version in CI

---

## Next Steps

- Review [Lint Rules Reference](../reference/lint-rules.md) for complete rule list
- See [Setup Linting](./setup-linting.md) for local development
- See [Migration Guide](./migrate-to-linting.md) for adopting in existing repos
- Read [ADR-005](../adr/adr-005-documentation-linting.md) for design decisions

---

**CI Integration Checklist**:
- [ ] Basic workflow runs on PRs
- [ ] Artifacts uploaded for review
- [ ] PR comments provide feedback
- [ ] Failures block merge
- [ ] Warnings don't block (optional)
- [ ] Team notified of new checks
- [ ] Documentation updated
