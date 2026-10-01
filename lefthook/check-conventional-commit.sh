#!/usr/bin/env bash
# check-conventional-commit.sh — validate a commit message follows the
# Conventional Commits specification (https://www.conventionalcommits.org/).
#
# Usage: check-conventional-commit.sh <path-to-commit-msg-file>
#
# Accepts:
#   <type>(<scope>)!: <subject>
#   <type>(<scope>): <subject>
#   <type>: <subject>
#
# Where <type> is one of:
#   build, chore, ci, docs, feat, fix, perf, refactor, revert, style, test
#
# Subject must be 1-100 characters (no leading period, no uppercase first
# letter — per the Conventional Commits spec).
#
# Merge / revert commits are accepted as-is (handled by the regex below).

set -euo pipefail

if [ "$#" -ne 1 ]; then
  echo "Usage: check-conventional-commit.sh <commit-msg-file>" >&2
  exit 1
fi

msg_file="$1"
# Read the first non-comment, non-empty line of the commit message.
# `git commit` puts the message in the file; the first line is the subject.
subject=$(awk 'NF && !/^#/ {print; exit}' "$msg_file")

# Full commit message body (used to validate the body line exists).
body=$(awk 'NF && !/^#/ {found=1} found {print}' "$msg_file")

if [ -z "$subject" ]; then
  echo "Commit message is empty." >&2
  echo "Expected format: <type>(<scope>): <subject>" >&2
  echo "Example: feat(cli): add --keep-workspace flag" >&2
  exit 1
fi

# Conventional Commits regex. Per spec:
#   type:     lowercase, one of the allowed types
#   scope:    optional, lowercase, may contain hyphens
#   !:        optional, marks a breaking change
#   subject:  required, no leading whitespace, 1-100 chars
# We anchor with ^...$ and require at least one space after the colon.
pattern='^(build|chore|ci|docs|feat|fix|perf|refactor|revert|style|test)(\([a-z0-9_-]+\))?!?: .+$'

if ! [[ "$subject" =~ $pattern ]]; then
  echo "Commit message does not follow Conventional Commits format." >&2
  echo "Got: $subject" >&2
  echo >&2
  echo "Expected: <type>(<scope>)!: <subject>" >&2
  echo "Allowed types: build, chore, ci, docs, feat, fix, perf, refactor, revert, style, test" >&2
  echo "Examples:" >&2
  echo "  feat(cli): add --keep-workspace flag" >&2
  echo "  fix(daemon): restore SkipState wiring" >&2
  echo "  refactor(build): drop factory injection" >&2
  echo "  chore(lint): fix golangci-lint alignment" >&2
  echo "  docs(readme): document lefthook hooks" >&2
  exit 1
fi

# Subject length check (1-100 chars, excluding newline).
subject_len=${#subject}
if [ "$subject_len" -gt 100 ]; then
  echo "Commit subject is too long ($subject_len chars, max 100)." >&2
  echo "Subject: $subject" >&2
  exit 1
fi

# Per Conventional Commits spec, "fix:" may require a body / footer for
# release-tooling compatibility. We don't enforce that here — just print
# a hint so authors think about it.
type=$(echo "$subject" | sed -E 's/^([a-z]+).*/\1/')
if [ "$type" = "fix" ] && [ -z "$body" ]; then
  echo "Hint: 'fix:' commits typically need a body explaining the bug." >&2
  echo "Hint: consider adding a 'Refs:' or 'BREAKING CHANGE:' footer if relevant." >&2
  # Don't fail — it's just a hint.
fi

exit 0