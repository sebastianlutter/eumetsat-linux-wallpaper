#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TMP_DIR="$(mktemp -d)"
BIN_PATH="${TMP_DIR}/eumetsat-wallpaper"

cleanup() {
    rm -rf "${TMP_DIR}"
}

trap cleanup EXIT

cd "${SCRIPT_DIR}"
go build -o "${BIN_PATH}" ./cmd/eumetsat-wallpaper
exec "${BIN_PATH}" uninstall "$@"
