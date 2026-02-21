#!/usr/bin/env sh
set -eu

# Ensure config dir ownership remains correct
mkdir -p /etc/dzsa-sync
chown -R dzsa-sync:dzsa-sync /etc/dzsa-sync || true

# Reload systemd only; do not start/enable service
if command -v systemctl >/dev/null 2>&1; then
  systemctl daemon-reload || true
fi

exit 0
