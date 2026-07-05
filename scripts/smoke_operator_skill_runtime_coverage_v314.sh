#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-https://stage.mustache-security.ir}"
USERNAME="${USERNAME:-admin}"
PASSWORD="${PASSWORD:-admin123}"
TARGET_ID="${TARGET_ID:-6}"

need() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "missing required command: $1" >&2
    exit 1
  }
}

need curl
need jq

TOKEN="$(
  curl -sk -X POST "$BASE_URL/api/auth/login" \
    -H "Content-Type: application/json" \
    -d "{\"username\":\"$USERNAME\",\"password\":\"$PASSWORD\"}" \
    | jq -r '.token // .data.token // .access_token // .data.access_token // empty'
)"

if [[ -z "$TOKEN" ]]; then
  echo "failed to obtain auth token" >&2
  exit 1
fi

SESSION_ID="$(
  curl -sk -X POST "$BASE_URL/api/targets/$TARGET_ID/agent-chat/sessions" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"title":"v3.14 skill runtime coverage smoke"}' \
    | jq -r '.session.id // .data.id // .id // empty'
)"

if [[ -z "$SESSION_ID" ]]; then
  echo "failed to create agent chat session" >&2
  exit 1
fi

RESPONSE_FILE="${RESPONSE_FILE:-/tmp/v314_skill_runtime_coverage.json}"

curl -sk -X POST "$BASE_URL/api/targets/$TARGET_ID/agent-chat/sessions/$SESSION_ID/messages" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"content":"Run authorized pentest reasoning for JS/API discovery, IDOR/auth context, XSS reflection, open redirect, path traversal and file-read candidates."}' \
  > "$RESPONSE_FILE"

SUMMARY="$(
  jq -c '.. | objects | select(has("skill_execution")) | .skill_execution | {
    executed_count,
    planned_count,
    not_implemented,
    has_parameter_inventory: has("parameter_inventory"),
    has_http_evidence_analysis: has("http_evidence_analysis"),
    has_auth_context_needed: has("auth_context_needed"),
    has_js_audit: has("js_audit"),
    has_xss_reflection: has("xss_reflection"),
    has_open_redirect: has("open_redirect"),
    has_path_traversal_baseline: has("path_traversal_baseline")
  }' "$RESPONSE_FILE" | tail -n 1
)"

if [[ -z "$SUMMARY" ]]; then
  echo "missing skill_execution in response" >&2
  cat "$RESPONSE_FILE" >&2
  exit 1
fi

echo "$SUMMARY" | jq '.'

executed_count="$(echo "$SUMMARY" | jq -r '.executed_count')"
planned_count="$(echo "$SUMMARY" | jq -r '.planned_count')"
not_implemented_count="$(echo "$SUMMARY" | jq -r '.not_implemented | length')"

if (( executed_count < 7 )); then
  echo "expected executed_count >= 7, got $executed_count" >&2
  exit 1
fi

if [[ "$planned_count" != "0" ]]; then
  echo "expected planned_count=0, got $planned_count" >&2
  exit 1
fi

if [[ "$not_implemented_count" != "0" ]]; then
  echo "expected not_implemented=[], got count=$not_implemented_count" >&2
  exit 1
fi

for key in \
  has_parameter_inventory \
  has_http_evidence_analysis \
  has_auth_context_needed \
  has_js_audit \
  has_xss_reflection \
  has_open_redirect \
  has_path_traversal_baseline
do
  value="$(echo "$SUMMARY" | jq -r ".$key")"
  if [[ "$value" != "true" ]]; then
    echo "expected $key=true, got $value" >&2
    exit 1
  fi
done

echo "v3.14 skill runtime coverage smoke passed"
