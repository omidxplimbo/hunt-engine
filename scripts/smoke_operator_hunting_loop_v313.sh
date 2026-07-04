#!/usr/bin/env bash
set -euo pipefail

APP_DIR="${APP_DIR:-/opt/hunt-engine/dev/app}"
COMPOSE_PROJECT="${COMPOSE_PROJECT:-hunt-dev}"
TARGET_ID="${TARGET_ID:-5}"
BASE_URL="${BASE_URL:-https://stage.mustache-security.ir}"
DB_USER="${DB_USER:-hunter}"
DB_NAME="${DB_NAME:-huntdb}"

cd "$APP_DIR"

COMPOSE=(docker compose -p "$COMPOSE_PROJECT" -f docker-compose.yml -f docker-compose.dev.yml)

echo "[1/8] repository"
git branch --show-current
git status --short

echo "[2/8] backend compile checks"
if [ -f .env ]; then
  set -a
  # shellcheck disable=SC1091
  source .env
  set +a
fi

cd backend
go test ./internal/api/handlers -run TestNonExistingSmoke -count=0
go test ./internal/ai/operator
cd "$APP_DIR"

echo "[3/8] dev API health"
HTTP_CODE="$(curl -k -s -o /tmp/hunt_v313_api_me.out -w "%{http_code}" "$BASE_URL/api/me" || true)"
cat /tmp/hunt_v313_api_me.out || true
echo
echo "HTTP_CODE=$HTTP_CODE"

if [ "$HTTP_CODE" != "401" ] && [ "$HTTP_CODE" != "200" ]; then
  echo "[fail] expected API /api/me to return 401 or 200, got $HTTP_CODE"
  exit 1
fi

echo "[4/8] hunting loop actions exist"
ACTION_COUNT="$("${COMPOSE[@]}" exec -T postgres psql -U "$DB_USER" -d "$DB_NAME" -Atc "
select count(*)
from agent_actions
where target_id = ${TARGET_ID}
  and input_json->>'source' = 'high_value_hunting_loop_v1';
")"

echo "ACTION_COUNT=$ACTION_COUNT"

if [ "${ACTION_COUNT:-0}" -lt 1 ]; then
  echo "[fail] expected at least one high_value_hunting_loop_v1 action"
  exit 1
fi

echo "[5/8] latest hunting session has structured output"
SESSION_ROW="$("${COMPOSE[@]}" exec -T postgres psql -U "$DB_USER" -d "$DB_NAME" -Atc "
select concat_ws('|',
       coalesce(output_json->'hunting_session'->>'version', ''),
       coalesce(output_json->'hunting_session'->>'candidate_count', ''),
       coalesce(output_json->'hunting_session'->>'analyzed_result_count', ''),
       coalesce(output_json->'hunting_session'->>'analysis_memory_ingested', '')
)
from agent_chat_messages
where target_id = ${TARGET_ID}
  and role = 'assistant'
  and output_json ? 'hunting_session'
  and coalesce((output_json->'hunting_session'->>'candidate_count')::int, 0) >= 1
  and coalesce((output_json->'hunting_session'->>'analyzed_result_count')::int, 0) >= 1
order by id desc
limit 1;
")"

echo "SESSION_ROW=$SESSION_ROW"

SESSION_VERSION="$(echo "$SESSION_ROW" | cut -d'|' -f1)"
SESSION_CANDIDATES="$(echo "$SESSION_ROW" | cut -d'|' -f2)"
SESSION_ANALYZED="$(echo "$SESSION_ROW" | cut -d'|' -f3)"
SESSION_MEMORY="$(echo "$SESSION_ROW" | cut -d'|' -f4)"

if [ -z "$SESSION_ROW" ]; then
  echo "[fail] no analyzed hunting_session found; run the UI prompt once when fresh candidates are available, then rerun smoke"
  exit 1
fi

if [ "$SESSION_VERSION" != "small_controlled_baseline_hunting_loop_v1" ]; then
  echo "[fail] unexpected hunting_session version: $SESSION_VERSION"
  exit 1
fi

if [ -z "$SESSION_CANDIDATES" ] || [ "$SESSION_CANDIDATES" -lt 1 ]; then
  echo "[fail] expected hunting_session candidate_count >= 1"
  exit 1
fi

if [ -z "$SESSION_ANALYZED" ] || [ "$SESSION_ANALYZED" -lt 1 ]; then
  echo "[fail] expected hunting_session analyzed_result_count >= 1"
  exit 1
fi

if [ "$SESSION_MEMORY" != "true" ]; then
  echo "[fail] expected hunting_session analysis_memory_ingested=true"
  exit 1
fi

echo "[6/8] analyzer learning memory exists"
MEMORY_COUNT="$("${COMPOSE[@]}" exec -T postgres psql -U "$DB_USER" -d "$DB_NAME" -Atc "
select count(*)
from target_memory_items
where target_id = ${TARGET_ID}
  and metadata->>'ingest_version' = 'hunting-probe-analysis-memory-v1';
")"

echo "MEMORY_COUNT=$MEMORY_COUNT"

if [ "${MEMORY_COUNT:-0}" -lt 1 ]; then
  echo "[fail] expected analyzer learning memory items"
  exit 1
fi

echo "[7/8] avoid-retesting memory exists for blocked/inconclusive candidates"
AVOID_COUNT="$("${COMPOSE[@]}" exec -T postgres psql -U "$DB_USER" -d "$DB_NAME" -Atc "
select count(*)
from target_memory_items
where target_id = ${TARGET_ID}
  and metadata->>'ingest_version' = 'hunting-probe-analysis-memory-v1'
  and metadata->>'avoid_retesting' = 'true';
")"

echo "AVOID_COUNT=$AVOID_COUNT"

if [ "${AVOID_COUNT:-0}" -lt 1 ]; then
  echo "[fail] expected at least one avoid_retesting=true memory item"
  exit 1
fi

echo "[8/8] blocked/challenged evidence is not promoted as vulnerability finding"
BAD_FINDING_COUNT="$("${COMPOSE[@]}" exec -T postgres psql -U "$DB_USER" -d "$DB_NAME" -Atc "
select count(*)
from target_memory_items
where target_id = ${TARGET_ID}
  and metadata->>'ingest_version' = 'hunting-probe-analysis-memory-v1'
  and metadata->>'should_not_promote_as_finding' = 'true'
  and memory_type = 'finding_evidence';
")"

echo "BAD_FINDING_COUNT=$BAD_FINDING_COUNT"

if [ "${BAD_FINDING_COUNT:-0}" -ne 0 ]; then
  echo "[fail] blocked/inconclusive analyzer memory must not be finding_evidence"
  exit 1
fi

echo "[ok] v3.13 operator hunting loop smoke passed"
