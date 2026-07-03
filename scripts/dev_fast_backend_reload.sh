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

echo "[1/5] gofmt backend"
gofmt -w backend

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
curl -k -i https://stage.mustache-security.ir/api/me | sed -n '1,35p'

echo "[ok] fast backend reload complete"
