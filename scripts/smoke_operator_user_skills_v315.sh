#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-https://stage.mustache-security.ir}"
USERNAME="${USERNAME:-admin}"
PASSWORD="${PASSWORD:-admin123}"
TARGET_ID="${TARGET_ID:-5}"
SUFFIX="${SUFFIX:-$(date +%s)}"

need() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "missing required command: $1" >&2
    exit 1
  }
}

need curl
need jq

TMP_DIR="${TMP_DIR:-/tmp}"
RESPONSE_FILE="${RESPONSE_FILE:-$TMP_DIR/v315_user_skills_chat_${TARGET_ID}_${SUFFIX}.json}"

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

echo "[1/8] auth ok"

LEARNING_RESPONSE="$(
  curl -sk -X POST "$BASE_URL/api/operator-learning" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d "{
      \"title\":\"v3.15 smoke SSRF methodology $SUFFIX\",
      \"summary\":\"Smoke methodology for custom SSRF skill selector validation.\",
      \"content\":\"When webhook, callback, import_url, url_parameter, and SSRF signals appear, prefer the matching user-defined SSRF planning skill and keep runtime execution dispatcher-gated until an authorized runtime exists.\",
      \"scope\":\"user\",
      \"source\":\"smoke\",
      \"status\":\"active\",
      \"bug_class\":\"ssrf\",
      \"skill_slug\":\"\",
      \"applies_to\":\"ssrf custom skill selector smoke\",
      \"trigger_signals\":[\"ssrf\",\"webhook\",\"callback\",\"import_url\",\"url_parameter\"],
      \"methodology\":{\"workflow\":[\"identify SSRF candidates\",\"classify callback sinks\",\"plan controlled validation only\"]},
      \"execution_hints\":{\"runtime\":\"dispatcher-gated in v3.15 smoke\"},
      \"confidence\":91
    }"
)"

LEARNING_ID="$(echo "$LEARNING_RESPONSE" | jq -r '.record.id // .learning.id // .data.id // .id // empty')"

if [[ -z "$LEARNING_ID" || "$LEARNING_ID" == "null" ]]; then
  echo "failed to create operator learning record" >&2
  echo "$LEARNING_RESPONSE" | jq '.' >&2
  exit 1
fi

echo "[2/8] created methodology record id=$LEARNING_ID"

SKILL_RESPONSE="$(
  curl -sk -X POST "$BASE_URL/api/operator-skills" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d "{
      \"name\":\"v3.15 Smoke SSRF Planner $SUFFIX\",
      \"slug\":\"v315-smoke-ssrf-planner-$SUFFIX\",
      \"description\":\"Smoke user-defined skill for v3.15 custom skill planning integration.\",
      \"scope\":\"user\",
      \"category\":\"network_file_cloud\",
      \"bug_class\":\"ssrf\",
      \"skill_type\":\"active_validation\",
      \"runtime_backend\":\"payload_generator\",
      \"permission_mode\":\"manual_approval\",
      \"default_risk_level\":\"high\",
      \"default_safety_level\":2,
      \"default_test_level\":2,
      \"default_autonomy_level\":1,
      \"trigger_signals\":[\"ssrf\",\"webhook\",\"callback\",\"url_parameter\",\"import_url\"],
      \"custom_definition\":{\"workflow\":[\"find ssrf candidates\",\"classify callback sinks\",\"plan controlled OOB-safe validation\"]},
      \"budget_defaults\":{\"max_candidates\":5},
      \"stop_conditions\":{\"no_custom_runtime_execution\":true}
    }"
)"

SKILL_ID="$(echo "$SKILL_RESPONSE" | jq -r '.skill.id // .data.id // .id // empty')"
SKILL_SLUG="$(echo "$SKILL_RESPONSE" | jq -r '.skill.slug // .data.slug // .slug // empty')"

if [[ -z "$SKILL_ID" || "$SKILL_ID" == "null" || -z "$SKILL_SLUG" || "$SKILL_SLUG" == "null" ]]; then
  echo "failed to create user-defined operator skill" >&2
  echo "$SKILL_RESPONSE" | jq '.' >&2
  exit 1
fi

echo "[3/8] created user-defined skill id=$SKILL_ID slug=$SKILL_SLUG"

PROFILE_PAYLOAD="$(
  jq -n \
    --argjson learning_id "$LEARNING_ID" \
    '{
      is_enabled: true,
      preferred_learning_record_ids: [$learning_id],
      disabled_skill_slugs: []
    }'
)"

PROFILE_RESPONSE="$(
  curl -sk -X PUT "$BASE_URL/api/targets/$TARGET_ID/operator-skill-profile" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d "$PROFILE_PAYLOAD"
)"

PROFILE_LEARNING_COUNT="$(echo "$PROFILE_RESPONSE" | jq -r '(.profile.preferred_learning_record_ids // .data.preferred_learning_record_ids // .preferred_learning_record_ids // []) | length')"

if [[ "$PROFILE_LEARNING_COUNT" == "0" ]]; then
  echo "failed to select methodology on target operator profile" >&2
  echo "$PROFILE_RESPONSE" | jq '.' >&2
  exit 1
fi

echo "[4/8] target operator profile selected methodology"

SESSION_ID="$(
  curl -sk -X POST "$BASE_URL/api/targets/$TARGET_ID/agent-chat/sessions" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d "{\"title\":\"v3.15 user-defined skill smoke $SUFFIX\"}" \
    | jq -r '.session.id // .data.id // .id // empty'
)"

if [[ -z "$SESSION_ID" || "$SESSION_ID" == "null" ]]; then
  echo "failed to create agent chat session" >&2
  exit 1
fi

echo "[5/8] created chat session id=$SESSION_ID"

curl -sk -X POST "$BASE_URL/api/targets/$TARGET_ID/agent-chat/sessions/$SESSION_ID/messages" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"content\":\"Hunt for real high-impact and reportable SSRF bugs. Look for webhook, callback, import_url and url_parameter sinks. Use custom operator skills if their trigger signals match, but do not execute custom runtimes yet.\"}" \
  > "$RESPONSE_FILE"

SUMMARY="$(
  jq -c --arg slug "$SKILL_SLUG" '
    .. | objects | select(has("selected_skills")) |
    {
      selected_custom_slugs: [.selected_skills[]? | select(.origin == "user") | .slug],
      selected_custom_reasons: [.selected_skills[]? | select(.slug == $slug) | .reason],
      selected_methodology_count: (.selected_methodologies // [] | length),
      custom_skill_selected: any(.selected_skills[]?; .slug == $slug and .origin == "user"),
      custom_not_implemented: any(.skill_execution.not_implemented[]?; .skill_slug == $slug),
      custom_run_ids: ([.skill_execution.not_implemented[]? | select(.skill_slug == $slug) | .run_ids[]?]),
      skill_execution_status: (.skill_execution.status // "")
    }
  ' "$RESPONSE_FILE" | tail -n 1
)"

if [[ -z "$SUMMARY" ]]; then
  echo "missing selected_skills summary in response" >&2
  cat "$RESPONSE_FILE" >&2
  exit 1
fi

echo "$SUMMARY" | jq '.'

custom_skill_selected="$(echo "$SUMMARY" | jq -r '.custom_skill_selected')"
custom_not_implemented="$(echo "$SUMMARY" | jq -r '.custom_not_implemented')"
custom_run_count="$(echo "$SUMMARY" | jq -r '.custom_run_ids | length')"
selected_methodology_count="$(echo "$SUMMARY" | jq -r '.selected_methodology_count')"

if [[ "$custom_skill_selected" != "true" ]]; then
  echo "expected custom skill to be selected in selected_skills" >&2
  exit 1
fi

if [[ "$custom_not_implemented" != "true" ]]; then
  echo "expected custom skill to remain dispatcher-gated/not_implemented" >&2
  exit 1
fi

if (( custom_run_count < 1 )); then
  echo "expected at least one planned OperatorSkillRun id for custom skill" >&2
  exit 1
fi

if (( selected_methodology_count < 1 )); then
  echo "expected selected methodology records in chat output" >&2
  exit 1
fi

echo "[6/8] chat selected custom skill and queued dispatcher-gated planned run"

LIST_CHECK="$(
  curl -sk "$BASE_URL/api/operator-skills?include_disabled=true" \
    -H "Authorization: Bearer $TOKEN" \
    | jq -r --arg slug "$SKILL_SLUG" 'any(.skills[]?; .slug == $slug and .origin == "user")'
)"

if [[ "$LIST_CHECK" != "true" ]]; then
  echo "expected created user-defined skill to be visible in operator skill list" >&2
  exit 1
fi

echo "[7/8] user-defined skill visible in executable skills API"
echo "[8/8] v3.15 user-defined operator skills smoke passed"
echo "response_file=$RESPONSE_FILE"
