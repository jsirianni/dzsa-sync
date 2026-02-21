#!/usr/bin/env sh
set -eu

# Stop the dzsa-sync service before removal
if command -v systemctl >/dev/null 2>&1; then
  systemctl stop dzsa-sync.service || true
fi

exit 0
