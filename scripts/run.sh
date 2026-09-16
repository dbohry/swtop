#!/usr/bin/env bash
# Runs the swtop binary, building it first if it doesn't exist yet.
# Any arguments are passed through to swtop (e.g. -config path.yaml).
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"
BIN="$ROOT_DIR/swtop"

if [ ! -x "$BIN" ]; then
  echo "swtop binary not found, building it..." >&2
  (cd "$ROOT_DIR" && go build -o swtop ./cmd/swtop)
fi

cd "$ROOT_DIR"
exec "$BIN" "$@"
