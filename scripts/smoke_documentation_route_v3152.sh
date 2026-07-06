#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-https://stage.mustache-security.ir}"

need() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "[FAIL] missing dependency: $1" >&2
    exit 1
  }
}

need curl
need grep
need sed

echo "[INFO] v3.15.2 documentation route smoke"
echo "[INFO] base_url=$BASE_URL"

status="$(
  curl -skL -o /tmp/v3152_documentation_index.html -w '%{http_code}' "$BASE_URL/documentation"
)"

if [ "$status" != "200" ]; then
  echo "[FAIL] /documentation expected HTTP 200, got $status" >&2
  sed -n '1,80p' /tmp/v3152_documentation_index.html >&2 || true
  exit 1
fi

if ! grep -q '<div id="root"' /tmp/v3152_documentation_index.html; then
  echo "[FAIL] /documentation did not return the React app shell" >&2
  sed -n '1,80p' /tmp/v3152_documentation_index.html >&2 || true
  exit 1
fi

asset_path="$(
  grep -oE '/assets/[^"]+\.js' /tmp/v3152_documentation_index.html | head -n 1 || true
)"

if [ -z "$asset_path" ]; then
  echo "[FAIL] could not find frontend JS asset in /documentation HTML" >&2
  sed -n '1,120p' /tmp/v3152_documentation_index.html >&2 || true
  exit 1
fi

curl -skL "$BASE_URL$asset_path" -o /tmp/v3152_documentation_bundle.js

if ! grep -aqE 'Hunt Engine Docs|Official Hunt Engine Documentation|Documentation Portal Foundation|Recon End-to-End|Bug-Class Validation|Operator Skills|مستندات رسمی|شروع سریع' /tmp/v3152_documentation_bundle.js; then
  echo "[FAIL] frontend bundle did not include documentation portal markers" >&2
  echo "[INFO] searched asset: $BASE_URL$asset_path" >&2
  exit 1
fi

echo "[OK] /documentation route returned React shell and documentation bundle markers"
echo "[INFO] index: /tmp/v3152_documentation_index.html"
echo "[INFO] bundle: /tmp/v3152_documentation_bundle.js"
