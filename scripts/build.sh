#!/usr/bin/env bash
# Builds every homectl artifact into dist/: the daemon (cross-compiled for
# linux/amd64, the only supported server target), the frontend, and the
# native client binary embedding that frontend.
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DIST_DIR="$ROOT_DIR/dist"
mkdir -p "$DIST_DIR"

echo "==> Building daemon (linux/amd64)"
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o "$DIST_DIR/homectl-daemon" "$ROOT_DIR/cmd/daemon"

echo "==> Building frontend"
( cd "$ROOT_DIR/web" && npm ci && npm run build )

echo "==> Building client (embeds web/dist, native host platform)"
CLIENT_OUT="$DIST_DIR/homectl"
if [[ "$(go env GOOS)" == "windows" ]]; then
  CLIENT_OUT="$CLIENT_OUT.exe"
fi
go build -o "$CLIENT_OUT" "$ROOT_DIR/cmd/client"

echo "==> Done. Artifacts in $DIST_DIR:"
ls -la "$DIST_DIR"
