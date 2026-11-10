#!/usr/bin/env bash
# Regenerate the vendored Relearn navigation template.
#
# The shared sidebar mode shadows the theme's layouts/partials/menu.html and
# renders the vendored copy instead, so the copy has to match the pinned
# Relearn release. Run this after bumping the pinned version in
# internal/hugo/sharednav.go.
#
# Usage: scripts/vendor-relearn-nav.sh
set -euo pipefail

cd "$(dirname "$0")/.."

TARGET="internal/hugo/assets/relearn/menu.html"
UPSTREAM="https://raw.githubusercontent.com/McShelby/hugo-theme-relearn"

VERSION="$(sed -n 's/^const relearnVersion = "\(.*\)"$/\1/p' internal/hugo/sharednav.go)"
if [ -z "$VERSION" ]; then
    echo "cannot read relearnVersion from internal/hugo/sharednav.go" >&2
    exit 1
fi

echo "vendoring Relearn $VERSION navigation into $TARGET"

{
    cat <<HEADER
<!--
  Vendored from the Relearn theme: layouts/partials/menu.html, release $VERSION
  (github.com/McShelby/hugo-theme-relearn, MIT licensed).

  DocBuilder shadows the theme's own menu.html with a placeholder so the
  navigation can be rendered once and injected in the browser; this copy
  renders the shared fragment instead. The file also carries Relearn's
  template definitions for the navigation walkers, which the fragment needs.

  Keep in sync with the pinned theme version (internal/hugo/relearnVersion).
  Regenerate with scripts/vendor-relearn-nav.sh.
-->
HEADER
    curl -fsSL "$UPSTREAM/$VERSION/layouts/partials/menu.html"
} > "$TARGET"

echo "wrote $(wc -l < "$TARGET") lines to $TARGET"
