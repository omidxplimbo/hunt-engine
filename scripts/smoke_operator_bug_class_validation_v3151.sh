#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-https://stage.mustache-security.ir}"
TARGET_ID="${TARGET_ID:-5}"
USERNAME="${USERNAME:-admin}"
PASSWORD="${PASSWORD:-admin123}"

need() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "[FAIL] missing dependency: $1" >&2
    exit 1
  }
}

need curl
need jq

echo "[INFO] v3.15.1 bug-class validation smoke"
echo "[INFO] base_url=$BASE_URL target_id=$TARGET_ID"

TOKEN="$(
  curl -sk -X POST "$BASE_URL/api/auth/login" \
    -H 'Content-Type: application/json' \
    -d "{\"username\":\"$USERNAME\",\"password\":\"$PASSWORD\"}" \
    | jq -r '.token // .data.token // .access_token // .data.access_token // empty'
)"

if [ -z "$TOKEN" ]; then
  echo "[FAIL] login did not return token" >&2
  exit 1
fi

SESSION_ID="$(
  curl -sk -X POST "$BASE_URL/api/targets/$TARGET_ID/agent-chat/sessions" \
    -H "Authorization: Bearer $TOKEN" \
    -H 'Content-Type: application/json' \
    -d '{"title":"v3.15.1 bug-class validation smoke"}' \
    | jq -r '.session.id // .data.id // .id // empty'
)"

if [ -z "$SESSION_ID" ]; then
  echo "[FAIL] could not create agent chat session" >&2
  exit 1
fi

OUT_FILE="/tmp/v3151_bug_class_validation_smoke.json"

curl -sk -X POST "$BASE_URL/api/targets/$TARGET_ID/agent-chat/sessions/$SESSION_ID/messages" \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"content":"Run authorized v3.15.1 bug-class validation for XSS reflection context, DOM XSS, CRLF header injection, cache poisoning/deception, open redirect chain, path traversal file read baseline, CORS, clickjacking, and CSRF. Use controlled Level 2 validation only. Do not execute browser payloads, destructive payloads, state-changing CSRF requests, cache poisoning payloads, raw CRLF payloads, sensitive file-read payloads, or external redirect following."}' \
  | tee "$OUT_FILE" >/dev/null

SUMMARY="$(
  jq '.. | objects | select(has("skill_execution")) | {
    executed_count: .skill_execution.executed_count,
    planned_count: .skill_execution.planned_count,
    not_implemented: [.skill_execution.not_implemented[]?.skill_slug],
    has_xss_context: (.skill_execution.xss_reflection_context != null),
    has_dom_xss: (.skill_execution.dom_xss != null),
    has_crlf: (.skill_execution.crlf_header_injection != null),
    has_cache: (.skill_execution.cache_poisoning_deception != null),
    has_open_redirect_chain: (.skill_execution.open_redirect_chain != null),
    has_path: (.skill_execution.path_traversal_file_read_baseline != null),
    has_cors: (.skill_execution.cors_clickjacking_csrf != null),
    xss_scope: .skill_execution.xss_reflection_context.runtime_scope,
    dom_scope: .skill_execution.dom_xss.runtime_scope,
    crlf_scope: .skill_execution.crlf_header_injection.runtime_scope,
    cache_scope: .skill_execution.cache_poisoning_deception.runtime_scope,
    redirect_scope: .skill_execution.open_redirect_chain.runtime_scope,
    path_scope: .skill_execution.path_traversal_file_read_baseline.runtime_scope,
    cors_scope: .skill_execution.cors_clickjacking_csrf.runtime_scope
  }' "$OUT_FILE" | tail -n 1
)"

echo "$SUMMARY" | jq .

fail=0

for field in has_xss_context has_dom_xss has_crlf has_cache has_open_redirect_chain has_path has_cors; do
  value="$(echo "$SUMMARY" | jq -r ".$field")"
  if [ "$value" != "true" ]; then
    echo "[FAIL] $field expected true, got $value" >&2
    fail=1
  fi
done

not_impl_count="$(echo "$SUMMARY" | jq '.not_implemented | length')"
if [ "$not_impl_count" != "0" ]; then
  echo "[FAIL] expected no not_implemented validation skills" >&2
  fail=1
fi

check_scope() {
  local field="$1"
  local expected="$2"
  local got
  got="$(echo "$SUMMARY" | jq -r ".$field // empty")"
  if [ "$got" != "$expected" ]; then
    echo "[FAIL] $field expected $expected, got $got" >&2
    fail=1
  fi
}

check_scope xss_scope "controlled_marker_reflection_probe_no_exploit_payload"
check_scope dom_scope "js_source_sink_evidence_no_browser_execution"
check_scope crlf_scope "controlled_header_marker_probe_no_raw_crlf_payload"
check_scope cache_scope "controlled_cache_behavior_probe_no_poisoning_payload"
check_scope redirect_scope "controlled_redirect_behavior_probe_no_external_follow"
check_scope path_scope "controlled_path_baseline_probe_no_sensitive_file_read"
check_scope cors_scope "controlled_cors_frame_cookie_header_probe_no_state_change"

if [ "$fail" != "0" ]; then
  echo "[FAIL] v3.15.1 bug-class validation smoke failed" >&2
  exit 1
fi

echo "[OK] v3.15.1 bug-class validation smoke passed"
echo "[INFO] raw output: $OUT_FILE"
