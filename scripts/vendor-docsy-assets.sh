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
    "sass-embedded@${SASS_VERSION}" >/dev/null

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

cat > "$TARGET/VERSION" <<EOF
docsy ${VERSION}
bootstrap ${BOOTSTRAP_VERSION}
fontawesome ${FONTAWESOME_VERSION}
EOF

echo "vendored:"
ls -la "$TARGET"
