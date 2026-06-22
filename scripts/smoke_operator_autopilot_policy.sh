#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-http://185.92.182.85:5081}"
TARGET_ID="${TARGET_ID:-1}"
TOKEN="${TOKEN:-}"

if [ -z "$TOKEN" ]; then
  if [ -f /tmp/hunt35-token.txt ]; then
    TOKEN="$(cat /tmp/hunt35-token.txt)"
  else
    echo "ERROR: TOKEN is not set and /tmp/hunt35-token.txt does not exist" >&2
    exit 1
  fi
fi

TMP_DIR="${TMP_DIR:-/tmp/hunt-autopilot-smoke}"
mkdir -p "$TMP_DIR"

echo "[smoke] base_url=$BASE_URL target_id=$TARGET_ID"

curl_json() {
  curl -sS "$@"
}

save_original_policy() {
  curl_json "$BASE_URL/api/targets/$TARGET_ID/policy" \
    -H "Authorization: Bearer $TOKEN" \
  | jq '.data // {}' > "$TMP_DIR/original-policy.json"
}

restore_original_policy() {
  if [ -s "$TMP_DIR/original-policy.json" ] && [ "$(jq 'length' "$TMP_DIR/original-policy.json")" != "0" ]; then
    echo "[smoke] restoring original target policy"
    curl_json -X PUT "$BASE_URL/api/targets/$TARGET_ID/policy" \
      -H "Authorization: Bearer $TOKEN" \
      -H "Content-Type: application/json" \
      --data @"$TMP_DIR/original-policy.json" >/dev/null || true
  fi
}

put_policy() {
  local mode="$1"
  local auto0="$2"
  local auto1="$3"
  local notes="$4"

  cat > "$TMP_DIR/policy-$mode.json" <<JSON
{
  "platform_name":"Development",
  "program_url":"",
  "in_scope_patterns":["test.com"],
  "out_of_scope_patterns":[],
  "allowed_test_types":["review_endpoint","http_probe","run_safe_bug_tests","run_owasp_checklist"],
  "disallowed_test_types":[],
  "max_test_intensity":"safe",
  "operator_mode":"$mode",
  "auto_execute_level_0":$auto0,
  "auto_execute_level_1":$auto1,
  "require_approval_level_2":true,
  "require_approval_level_3":true,
  "auth_required":false,
  "rate_limit_notes":"",
  "safe_testing_notes":"$notes",
  "reporting_preferences":"",
  "business_context":"",
  "asset_criticality_default":"medium"
}
JSON

  curl_json -X PUT "$BASE_URL/api/targets/$TARGET_ID/policy" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    --data @"$TMP_DIR/policy-$mode.json" \
  | tee "$TMP_DIR/policy-$mode-response.json" \
  | jq -e --arg mode "$mode" '.status == "success" and .data.operator_mode == $mode' >/dev/null
}

create_session() {
  local title="$1"
  curl_json -X POST "$BASE_URL/api/targets/$TARGET_ID/agent-chat/sessions" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    --data "{\"title\":\"$title\"}" \
  | jq -r '.data.id'
}

send_endpoint_message() {
  local session_id="$1"
  local outfile="$2"

  curl_json -X POST "$BASE_URL/api/targets/$TARGET_ID/agent-chat/sessions/$session_id/messages" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    --data '{"content":"فقط این endpoint رو بررسی کن https://www.test.com/"}' \
  | tee "$outfile" >/dev/null
}

assert_assisted_autopilot() {
  local file="$1"

  jq -e '
    .data.assistant_message.output_json.autopilot.enabled == true
    and .data.assistant_message.output_json.autopilot.mode == "assisted_autopilot"
    and .data.assistant_message.output_json.autopilot.policy_source == "target_policy"
    and (.data.assistant_message.output_json.autopilot.executed_actions | length) > 0
    and (.data.assistant_message.output_json.autopilot.controlled_runs | length) > 0
    and (.data.assistant_message.output_json.autopilot.controlled_results | length) > 0
  ' "$file" >/dev/null

  local vulnerable_count
  vulnerable_count="$(jq '[.data.assistant_message.output_json.autopilot.summary[]? | select(.status == "vulnerable")] | length' "$file")"
  if [ "$vulnerable_count" != "0" ]; then
    echo "ERROR: assisted smoke produced vulnerable status for the blocked/inconclusive probe" >&2
    jq '.data.assistant_message.output_json.autopilot' "$file" >&2
    exit 1
  fi
}

assert_strict_approval() {
  local file="$1"

  jq -e '
    .data.assistant_message.output_json.autopilot.enabled == false
    and .data.assistant_message.output_json.autopilot.mode == "strict_approval"
    and .data.assistant_message.output_json.autopilot.policy_source == "target_policy"
    and (.data.assistant_message.output_json.autopilot.executed_actions | length) == 0
    and (.data.assistant_message.output_json.autopilot.controlled_runs | length) == 0
    and (.data.assistant_message.output_json.autopilot.controlled_results | length) == 0
    and (.data.assistant_message.output_json.autopilot.skipped_actions | length) > 0
    and (
      [.data.assistant_message.output_json.autopilot.skipped_actions[]?.reason]
      | join(" ")
      | contains("strict_approval")
    )
  ' "$file" >/dev/null
}

save_original_policy
trap restore_original_policy EXIT

echo "[smoke] assisted_autopilot should execute low-risk controlled endpoint review"
put_policy "assisted_autopilot" "true" "true" "Autopilot policy smoke: assisted mode."
ASSISTED_SESSION_ID="$(create_session "Smoke Assisted Autopilot Policy")"
send_endpoint_message "$ASSISTED_SESSION_ID" "$TMP_DIR/assisted-chat.json"
assert_assisted_autopilot "$TMP_DIR/assisted-chat.json"
echo "[smoke] assisted_autopilot passed"

echo "[smoke] strict_approval should not auto-execute"
put_policy "strict_approval" "false" "false" "Autopilot policy smoke: strict approval mode."
STRICT_SESSION_ID="$(create_session "Smoke Strict Approval Policy")"
send_endpoint_message "$STRICT_SESSION_ID" "$TMP_DIR/strict-chat.json"
assert_strict_approval "$TMP_DIR/strict-chat.json"
echo "[smoke] strict_approval passed"

echo "[smoke] PASS: operator autopilot policy controls are working"
echo "[smoke] assisted_session_id=$ASSISTED_SESSION_ID strict_session_id=$STRICT_SESSION_ID"
