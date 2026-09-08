#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
export HRS_URL="${HRS_URL:-http://127.0.0.1:9746}"
if ! curl -sf "$HRS_URL/health" >/dev/null; then
  echo "hrs serve is not reachable at $HRS_URL" >&2
  echo "start it with: hrs serve" >&2
  exit 1
fi
cd "$ROOT/gui-macos"
exec swift run HrsGUI
