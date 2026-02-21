#!/usr/bin/env sh
set -eu

for cmd in getent id groupadd useradd; do
  command -v "$cmd" >/dev/null 2>&1 || { echo "Required command '$cmd' not found" >&2; exit 1; }
done

if ! getent group dzsa-sync >/dev/null 2>&1; then
  groupadd -r dzsa-sync
fi

if ! id -u dzsa-sync >/dev/null 2>&1; then
  useradd -r -g dzsa-sync -s /sbin/nologin -d /nonexistent -M dzsa-sync
fi

if command -v systemctl >/dev/null 2>&1; then
  systemctl daemon-reload
fi
