#!/usr/bin/env bash
# check-fmt.sh — verify staged Go files are gofmt -s clean.
#
# Usage: check-fmt.sh <file1.go> [<file2.go> ...]
# Exits 0 if every file is formatted, 1 otherwise.
# Lefthook invokes this with the staged Go files as positional args.
#
# gofmt -s -l lists files that need formatting (one per line). An empty
# stdout means everything is clean; we exit non-zero with a helpful message
# when files are listed.

set -euo pipefail

if [ "$#" -eq 0 ]; then
  # No staged files passed; nothing to check. (Shouldn't happen because
  # lefthook's `glob: "*.go"` filter ensures this only runs when at least
  # one staged file matches, but guard anyway.)
  exit 0
fi

unformatted=$(gofmt -s -l "$@")
if [ -n "$unformatted" ]; then
  echo "Files need 'gofmt -s' formatting:"
  echo "$unformatted" | sed 's/^/  /'
  echo
  echo "Run: gofmt -s -w <file>"
  exit 1
fi
exit 0