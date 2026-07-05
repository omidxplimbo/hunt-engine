#!/usr/bin/env bash
set -euo pipefail

APP_DIR="/opt/hunt-engine/dev/app"
BACKEND_CONTAINER="hunt-dev-backend"
COMPOSE="docker compose -p hunt-dev -f docker-compose.yml -f docker-compose.dev.yml"

cd "$APP_DIR"

if [ -f .env ]; then
  set -a
  # shellcheck disable=SC1091
  source .env
  set +a
fi

echo "[1/5] gofmt changed backend Go files"
mapfile -t GO_FILES < <(git ls-files -m -o --exclude-standard -- 'backend/*.go' 'backend/**/*.go')
if [ "${#GO_FILES[@]}" -gt 0 ]; then
  gofmt -w "${GO_FILES[@]}"
else
  echo "no changed backend Go files to format"
fi

echo "[2/5] compile checks"
cd backend
go test ./internal/api/handlers -run TestNonExistingSmoke -count=0
go test ./internal/ai/operator

echo "[3/5] build backend binary"
CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /tmp/hunt-engine-api ./cmd/server

cd "$APP_DIR"

echo "[4/5] copy binary into dev backend container"
docker cp /tmp/hunt-engine-api "$BACKEND_CONTAINER:/root/hunt-engine-api"
docker exec "$BACKEND_CONTAINER" chmod +x /root/hunt-engine-api

echo "[5/5] restart backend"
$COMPOSE restart backend

sleep 5

echo "[check] backend logs"
$COMPOSE logs --tail=80 backend | grep -Ei 'panic|fatal|undefined' && {
  echo "[fail] suspicious backend log found"
  exit 1
} || true

echo "[check] stage api"
API_OK=0
for attempt in 1 2 3 4 5; do
  HTTP_CODE="$(curl -k -s -o /tmp/hunt_fast_backend_api_me.out -w "%{http_code}" https://stage.mustache-security.ir/api/me || true)"
  cat /tmp/hunt_fast_backend_api_me.out || true
  echo
  echo "HTTP_CODE=$HTTP_CODE attempt=$attempt"

  if [ "$HTTP_CODE" = "401" ] || [ "$HTTP_CODE" = "200" ]; then
    API_OK=1
    break
  fi

  sleep 5
done

if [ "$API_OK" != "1" ]; then
  echo "[fail] stage API did not become healthy after backend restart"
  exit 1
fi

echo "[ok] fast backend reload complete"
