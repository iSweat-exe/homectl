#!/usr/bin/env bash
# Generates <dist-dir>/manifest.json describing every release artifact
# (component, os/arch, filename, size, sha256, download URL) for a given
# tag. Run by .github/workflows/release.yml after cross-compiling, and
# published as a release asset alongside the binaries.
#
# This is also the manifest a future `homectl update` / `homectl-daemon
# update` command would fetch to discover and verify the latest binary for
# its own component/os/arch — not built yet, but the schema is shaped for it.
set -euo pipefail

VERSION="${1:?usage: generate-manifest.sh <version> <owner/repo> <dist-dir>}"
REPO="${2:?usage: generate-manifest.sh <version> <owner/repo> <dist-dir>}"
DIST_DIR="${3:?usage: generate-manifest.sh <version> <owner/repo> <dist-dir>}"

RELEASED_AT="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

artifacts="[]"
for entry in \
  "daemon:linux:amd64:homectl-daemon-linux-amd64" \
  "daemon:linux:arm64:homectl-daemon-linux-arm64" \
  "client:linux:amd64:homectl-linux-amd64" \
  "client:linux:arm64:homectl-linux-arm64"
do
  IFS=: read -r component os arch filename <<<"$entry"
  path="$DIST_DIR/$filename"
  if [[ ! -f "$path" ]]; then
    echo "generate-manifest: missing artifact $path" >&2
    exit 1
  fi

  sha256="$(sha256sum "$path" | cut -d' ' -f1)"
  size="$(stat -c%s "$path")"
  url="https://github.com/$REPO/releases/download/$VERSION/$filename"

  artifacts="$(jq --argjson artifacts "$artifacts" \
    --arg component "$component" --arg os "$os" --arg arch "$arch" \
    --arg filename "$filename" --arg sha256 "$sha256" --argjson size "$size" \
    --arg url "$url" \
    '$artifacts + [{component: $component, os: $os, arch: $arch, filename: $filename, size: $size, sha256: $sha256, url: $url}]')"
done

jq -n --arg version "$VERSION" --arg released_at "$RELEASED_AT" --argjson artifacts "$artifacts" \
  '{version: $version, released_at: $released_at, artifacts: $artifacts}' \
  > "$DIST_DIR/manifest.json"

echo "==> Wrote $DIST_DIR/manifest.json"
cat "$DIST_DIR/manifest.json"
