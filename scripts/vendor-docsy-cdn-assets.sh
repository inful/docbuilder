#!/usr/bin/env bash
# Regenerate the vendored CDN assets of the Docsy theme.
#
# The theme loads mermaid and KaTeX from a CDN with resources.GetRemote. A
# build without a network - the build of the operator, the build in the
# cluster - fails on it, and the pages lose their diagrams and their
# formulae. DocBuilder serves its own copies, the way it already serves
# the stylesheet, the Bootstrap JavaScript, the fonts and the search
# library.
#
# This script needs a maintainer machine with a network; the build itself
# needs none of it.
#
# Usage: scripts/vendor-docsy-cdn-assets.sh
set -euo pipefail

cd "$(dirname "$0")/.."

# The versions the theme pins. They must match the ones the site
# configuration asks for.
MERMAID_VERSION="11.17.2"
KATEX_VERSION="0.19.0"

ASSETS="$(pwd)/internal/hugo/pipeline/assets"

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

# --- mermaid ---------------------------------------------------------------
mkdir -p "$ASSETS/mermaid"
curl -fsSL "https://cdn.jsdelivr.net/npm/mermaid@${MERMAID_VERSION}/dist/mermaid.esm.min.mjs" \
  -o "$ASSETS/mermaid/mermaid.esm.min.mjs"
echo "mermaid ${MERMAID_VERSION} vendored"

# --- KaTeX -----------------------------------------------------------------
mkdir -p "$ASSETS/katex/fonts"
curl -fsSL "https://unpkg.com/katex@${KATEX_VERSION}/dist/katex.min.css" \
  -o "$WORK/katex.min.css"
curl -fsSL "https://unpkg.com/katex@${KATEX_VERSION}/dist/katex.min.css" \
  -o "$ASSETS/katex/katex.min.css"

# The stylesheet asks for three formats of every font. A browser asks for
# the first one it understands, and every browser understands woff2, so
# only the woff2 files are vendored. The other two are taken out of the
# stylesheet, so a missing file is never asked for.
python3 - "$WORK/katex.min.css" "$ASSETS/katex/katex.min.css" <<'PY'
import re
import sys

source, target = sys.argv[1], sys.argv[2]
with open(source, encoding="utf-8") as handle:
    css = handle.read()

# Drop the woff and truetype sources of every font.
css = re.sub(r",url\(fonts/[^)]+\.woff\) format\(\"woff\"\)", "", css)
css = re.sub(r",url\(fonts/[^)]+\.ttf\) format\(\"truetype\"\)", "", css)

with open(target, "w", encoding="utf-8") as handle:
    handle.write(css)
PY

# The fonts the stylesheet asks for.
grep -o 'url(fonts/[^)]*\.woff2)' "$ASSETS/katex/katex.min.css" \
  | sed 's|url(fonts/||; s|)||' | sort -u | while read -r font; do
  curl -fsSL "https://unpkg.com/katex@${KATEX_VERSION}/dist/fonts/${font}" \
    -o "$ASSETS/katex/fonts/${font}"
done
echo "katex ${KATEX_VERSION} vendored ($(ls "$ASSETS/katex/fonts" | wc -l | tr -d ' ') fonts)"
