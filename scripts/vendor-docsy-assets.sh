#!/usr/bin/env bash
# Regenerate the vendored Docsy assets.
#
# Docsy's stylesheet is compiled from SCSS that imports Bootstrap and
# Font Awesome from npm. Hugo 0.170 has no Sass support of its own
# (the extended edition is gone), so DocBuilder ships the compiled
# stylesheet, the Bootstrap JavaScript, and the two web fonts the
# stylesheet references. Nothing else in the Docsy theme needs npm.
#
# This script needs a maintainer machine with hugo, node and npm; the
# build itself needs none of them.
#
# Usage: scripts/vendor-docsy-assets.sh
set -euo pipefail

cd "$(dirname "$0")/.."

HUGO_BIN="${HUGO_BIN:-hugo}"

VERSION="$(sed -n 's/^const DocsyVersion = "\(.*\)"$/\1/p' internal/hugo/pipeline/theme.go)"
if [ -z "$VERSION" ]; then
    echo "cannot read DocsyVersion from internal/hugo/pipeline/theme.go" >&2
    exit 1
fi

# The npm packages the theme's SCSS imports. The theme pins them in
# theme/package.json; these must match.
BOOTSTRAP_VERSION="5.3.8"
FONTAWESOME_VERSION="7.3.1"
SASS_VERSION="1.106.1"

# The Docsy theme loads the offline-search library (lunr) from the
# unpkg CDN. The site serves its own copy, so the version has to match
# the one the theme pins.
LUNR_VERSION="2.3.9"

# Absolute: the script builds the theme in a temporary directory.
TARGET="$(pwd)/internal/hugo/pipeline/assets/docsy"

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

mkdir -p "$WORK/content/docs"
cd "$WORK"

cat > hugo.yaml <<EOF
title: Docsy Asset Vendor
baseURL: http://localhost/
module:
  proxy: direct
  imports:
    - path: github.com/docsy/docsy/theme
      version: ${VERSION}
  mounts:
    - source: content
      target: content
    - source: assets
      target: assets
    - source: layouts
      target: layouts
    - source: static
      target: static
    - source: node_modules/bootstrap
      target: assets/vendor/bootstrap
    - source: node_modules/@fortawesome/fontawesome-free
      target: assets/vendor/Font-Awesome
    - source: node_modules/@fortawesome/fontawesome-free/webfonts
      target: static/webfonts
EOF

printf -- '---\ntitle: Docs\ntype: docs\n---\n' > content/docs/_index.md

echo "vendoring Docsy ${VERSION} assets into ${TARGET}"

"$HUGO_BIN" mod init docsy-asset-vendor >/dev/null
"$HUGO_BIN" mod get "github.com/docsy/docsy/theme@${VERSION}" >/dev/null

npm init -y >/dev/null 2>&1
npm install --no-audit --no-fund --save-exact \
    "bootstrap@${BOOTSTRAP_VERSION}" \
    "@fortawesome/fontawesome-free@${FONTAWESOME_VERSION}" \
    "sass-embedded@${SASS_VERSION}" \
    "lunr@${LUNR_VERSION}" >/dev/null

# The theme compiles its stylesheet with the dartsass transpiler, which
# Hugo looks up as "sass" on PATH. The npm package puts it there.
PATH="$WORK/node_modules/.bin:$PATH" "$HUGO_BIN" -e production >/dev/null

mkdir -p "$TARGET"

# The stylesheet references the web fonts through a relative URL. The
# fonts are served from a versioned directory, because the docs server
# gives fonts a one-year immutable cache header: a stable URL would
# keep serving the old font after a theme bump.
CSS="$(ls public/scss/main.min.*.css)"
sed "s|\.\./webfonts/|../webfonts/${VERSION}/|g" "$CSS" > "$TARGET/docsy.css"

cp node_modules/bootstrap/dist/js/bootstrap.bundle.js "$TARGET/bootstrap.bundle.js"
cp node_modules/@fortawesome/fontawesome-free/webfonts/fa-brands-400.woff2 "$TARGET/"
cp node_modules/@fortawesome/fontawesome-free/webfonts/fa-solid-900.woff2 "$TARGET/"
cp node_modules/lunr/lunr.min.js "$TARGET/lunr.min.js"

# The theme's head partial, with the offline-search script pointed at
# the copy above instead of the unpkg CDN. The theme's own partial is
# the single source for everything else in the head.
"$HUGO_BIN" mod vendor >/dev/null
THEME_HEAD="$(find _vendor -path '*docsy*/theme@*/layouts/_partials/head.html' | head -1)"
if [ -z "$THEME_HEAD" ]; then
    echo "cannot find the theme's head.html in _vendor" >&2
    exit 1
fi
{
  echo "{{- /* Vendored from the Docsy theme ${VERSION}: layouts/_partials/head.html."
  echo "  Regenerate with scripts/vendor-docsy-assets.sh; do not edit by hand."
  echo ""
  echo "  The only change is the offline-search script: the theme loads lunr"
  echo "  from the unpkg CDN, this site serves its own copy. */ -}}"
} > "$TARGET/head.html"

awk '
  /^\{\{ if \.Site\.Params\.offlineSearch -\}\}$/ {
    print "{{ if .Site.Params.offlineSearch -}}"
    print "{{- $lunr := resources.Get \"vendor/lunr/lunr.min.js\" | fingerprint -}}"
    print "<script defer"
    print "  src=\"{{ $lunr.RelPermalink }}\""
    print "  integrity=\"{{ $lunr.Data.Integrity }}\""
    print "  crossorigin=\"anonymous\"></script>"
    skipping = 1
    next
  }
  skipping && /^\{\{ end -\}\}$/ { print; skipping = 0; next }
  skipping { next }
  { print }
' "$THEME_HEAD" >> "$TARGET/head.html"

if grep -q unpkg "$TARGET/head.html"; then
    echo "the theme's head.html still points at the unpkg CDN" >&2
    exit 1
fi

# The theme loads the styles of the code blocks through its own
# stylesheet with Hugo's meta.load-css, and the stylesheet we ship is
# built without them: the code blocks of every site had no colours and
# their line numbers stood in a column of their own. The theme keeps the
# styles in two files, one for the light and one for the dark mode.
CHROMA_LIGHT="$(find _vendor -path '*docsy*/theme@*/assets/scss/td/chroma/_light.scss' | head -1)"
CHROMA_DARK="$(find _vendor -path '*docsy*/theme@*/assets/scss/td/chroma/_dark.scss' | head -1)"
if [ -z "$CHROMA_LIGHT" ] || [ -z "$CHROMA_DARK" ]; then
  echo "cannot find the styles of the code blocks in the theme" >&2
  exit 1
fi
{
  echo "/* The styles of the code blocks, from the Docsy theme ${VERSION}"
  echo "   (assets/scss/td/chroma). The theme loads them through its own"
  echo "   stylesheet with Hugo's meta.load-css, and the stylesheet we ship"
  echo "   is built without them."
  echo ""
  echo "   Regenerate with scripts/vendor-docsy-assets.sh; do not edit by hand. */"
  echo "[data-bs-theme='light'] {"
  cat "$CHROMA_LIGHT"
  echo "}"
  echo ""
  echo "[data-bs-theme='dark'] {"
  cat "$CHROMA_DARK"
  echo "}"
} > "$TARGET/chroma.css"

cat > "$TARGET/VERSION" <<EOF
docsy ${VERSION}
bootstrap ${BOOTSTRAP_VERSION}
fontawesome ${FONTAWESOME_VERSION}
lunr ${LUNR_VERSION}
EOF

echo "vendored:"
ls -la "$TARGET"
