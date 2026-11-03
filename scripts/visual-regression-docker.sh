#!/usr/bin/env bash
# Run screenshot regression against an isolated local Docker stack.
# The stack and its disposable Mongo database are removed after the run.
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$ROOT_DIR"

COMPOSE_FILE=${COMPOSE_FILE:-deploy/compose.visual.yaml}
VISUAL_WEB_PORT=${VISUAL_WEB_PORT:-13009}
KEEP=${KEEP:-0}
BASE_URL=${BASE_URL:-http://localhost:$VISUAL_WEB_PORT}
STARTED=0

cleanup() {
  local rc=$?
  if [[ "$STARTED" == "1" && "$KEEP" != "1" ]]; then
    docker compose -f "$COMPOSE_FILE" down -v --remove-orphans >/dev/null 2>&1 || true
  elif [[ "$KEEP" == "1" ]]; then
    echo "visual-regression-docker: stack kept running at $BASE_URL (KEEP=1)" >&2
  fi
  exit "$rc"
}
trap cleanup EXIT

if [[ ! "$BASE_URL" =~ ^http://localhost(:[0-9]+)?/?$ ]]; then
  echo "visual-regression-docker: BASE_URL must be http://localhost[:port], got: $BASE_URL" >&2
  exit 2
fi
if [[ "$BASE_URL" != "http://localhost:$VISUAL_WEB_PORT" ]]; then
  echo "visual-regression-docker: BASE_URL must match VISUAL_WEB_PORT ($VISUAL_WEB_PORT)" >&2
  exit 2
fi

echo "==> Starting isolated visual stack"
docker compose -f "$COMPOSE_FILE" down -v --remove-orphans >/dev/null 2>&1 || true
STARTED=1
VISUAL_WEB_PORT="$VISUAL_WEB_PORT" docker compose -f "$COMPOSE_FILE" up -d --build

echo "==> Waiting for local API and web"
ready=0
for _ in $(seq 1 120); do
  if curl -fsS "http://localhost:$VISUAL_WEB_PORT/api/v1/health" >/dev/null 2>&1; then
    ready=1
    break
  fi
  sleep 1
done
if [[ "$ready" != "1" ]]; then
  docker compose -f "$COMPOSE_FILE" ps >&2 || true
  docker compose -f "$COMPOSE_FILE" logs api web >&2 || true
  echo "visual-regression-docker: local stack did not become healthy" >&2
  exit 1
fi

echo "==> Seeding deterministic visual fixtures"
COMPOSE_FILE="$COMPOSE_FILE" ./scripts/seed-visual-fixtures.sh

echo "==> Running screenshot regression"
./scripts/visual-regression.sh --self-test
BASE_URL="$BASE_URL" CURRENT_DIR="${CURRENT_DIR:-.temp/visual-current-docker}" \
  BASELINE_DIR="${BASELINE_DIR:-web/visual-baselines}" \
  UPDATE_BASELINES="${UPDATE_BASELINES:-0}" \
  MAX_DIFF_PIXELS="${MAX_DIFF_PIXELS:-5000}" \
  VISUAL_AUTH_USERNAME="${VISUAL_AUTH_USERNAME:-visual_admin}" \
  VISUAL_AUTH_PASSWORD="${VISUAL_AUTH_PASSWORD:-secret}" \
  ./scripts/visual-regression.sh
