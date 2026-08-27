#!/usr/bin/env bash
# Installs the homectl daemon as a systemd service on this Linux server.
# Run as root. Usage: install-daemon.sh [path/to/homectl-daemon]
set -euo pipefail

if [[ $EUID -ne 0 ]]; then
  echo "must run as root" >&2
  exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BINARY="${1:-$SCRIPT_DIR/../dist/homectl-daemon}"

if [[ ! -f "$BINARY" ]]; then
  echo "daemon binary not found at $BINARY (build it first with scripts/build.sh, or pass a path)" >&2
  exit 1
fi

install -m 0755 "$BINARY" /usr/local/bin/homectl-daemon
install -m 0644 "$SCRIPT_DIR/../deploy/systemd/homectl-daemon.service" /etc/systemd/system/homectl-daemon.service

systemctl daemon-reload
systemctl enable --now homectl-daemon

sleep 1
echo "homectl-daemon installed and started."
echo
echo "Identity fingerprint (from the service log):"
journalctl -u homectl-daemon -n 20 --no-pager | grep "identity fingerprint" \
  || echo "  (not printed yet — check: journalctl -u homectl-daemon)"

echo
echo "To pair a client, open this server's pairing window (valid for a short window only):"
echo "  sudo homectl-daemon pair"
