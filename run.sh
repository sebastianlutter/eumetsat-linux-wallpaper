#!/usr/bin/env bash
set -euo pipefail

venv_dir="venv"

if [[ -z "${VIRTUAL_ENV:-}" ]]; then
  if [[ ! -d "${venv_dir}" ]]; then
    python3 -m venv "${venv_dir}"
  fi
  # shellcheck disable=SC1090
  source "${venv_dir}/bin/activate"
  pip install -r requirements.txt
fi

python3 download_earth_from_eumetsat.py "$@"
