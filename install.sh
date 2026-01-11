#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SYSTEMD_USER_DIR="${XDG_CONFIG_HOME:-$HOME/.config}/systemd/user"
SERVICE_SRC="${SCRIPT_DIR}/eumetsat-wallpaper.service"
TIMER_SRC="${SCRIPT_DIR}/eumetsat-wallpaper.timer"
SERVICE_DST="${SYSTEMD_USER_DIR}/eumetsat-wallpaper.service"
TIMER_DST="${SYSTEMD_USER_DIR}/eumetsat-wallpaper.timer"
ESCAPED_DIR="${SCRIPT_DIR//|/\\|}"

mkdir -p "$SYSTEMD_USER_DIR"

sed "s|CHANGE_ME|${ESCAPED_DIR}|g" "$SERVICE_SRC" > "$SERVICE_DST"
sed "s|CHANGE_ME|${ESCAPED_DIR}|g" "$TIMER_SRC" > "$TIMER_DST"

systemctl --user daemon-reload
systemctl --user enable --now eumetsat-wallpaper.timer

echo "Installed systemd user units to: ${SYSTEMD_USER_DIR}"
echo "Timer enabled: eumetsat-wallpaper.timer (hourly)"
