#!/usr/bin/env bash
# Smoke test for the production-shaped stack (deploy/compose.prod.yaml).
# Usage: BASE_URL=http://localhost:8080 scripts/prod-stack-smoke.sh
set -euo pipefail

BASE_URL=${BASE_URL:-http://localhost:8080}
JUDGE_USERNAME=${JUDGE_USERNAME:-judge}
JUDGE_PASSWORD=${JUDGE_PASSWORD:-judge_dev_password}
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT
fail=0

ok() { printf '  ok   %s\n' "$1"; }
bad() { printf '  FAIL %s\n' "$1"; fail=1; }

expect() {
  local label=$1 want=$2 got=$3
  if [ "$got" = "$want" ]; then ok "$label"; else bad "$label (got $got, want $want)"; fi
}

echo "smoke: $BASE_URL"

status=$(curl -s -o "$TMP/index.html" -w '%{http_code}' "$BASE_URL/")
expect "GET /" 200 "$status"
grep -q 'id="root"' "$TMP/index.html" && ok "SPA shell served" || bad "SPA shell missing"

headers=$(curl -s -D - -o /dev/null "$BASE_URL/")
echo "$headers" | grep -qi 'X-Content-Type-Options: nosniff' && ok "SPA security headers" || bad "SPA security headers"

status=$(curl -s -o "$TMP/health.json" -w '%{http_code}' "$BASE_URL/api/v1/health")
expect "GET /api/v1/health" 200 "$status"
grep -q '"status":"ok"' "$TMP/health.json" && ok "database healthy" || bad "health payload"

curl -s -D - -o /dev/null "$BASE_URL/api/v1/health" | grep -qi 'X-Content-Type-Options: nosniff' \
  && ok "API security headers" || bad "API security headers"

status=$(curl -s -o "$TMP/spa.html" -w '%{http_code}' "$BASE_URL/p/P201")
expect "SPA fallback /p/P201" 200 "$status"
grep -q 'id="root"' "$TMP/spa.html" && ok "SPA route served" || bad "SPA route not served"

status=$(curl -s -o /dev/null -w '%{http_code}' -X POST \
  -d "uname=$JUDGE_USERNAME&password=$JUDGE_PASSWORD" "$BASE_URL/login")
expect "judge POST /login" 200 "$status"
curl -s -c "$TMP/judge.jar" -o /dev/null -X POST \
  -d "uname=$JUDGE_USERNAME&password=$JUDGE_PASSWORD" "$BASE_URL/login"
status=$(curl -s -b "$TMP/judge.jar" -o "$TMP/files.json" -w '%{http_code}' "$BASE_URL/judge/files")
expect "judge GET /judge/files" 200 "$status"
grep -q '"ok":true' "$TMP/files.json" && ok "judge file check" || bad "judge file payload"

ws=$(curl -s -i -N --max-time 3 \
  -H 'Connection: Upgrade' -H 'Upgrade: websocket' -H 'Sec-WebSocket-Version: 13' \
  -H 'Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==' \
  -b "$TMP/judge.jar" "$BASE_URL/judge/conn" | head -1 || true)
echo "$ws" | grep -q '101 Switching Protocols' && ok "judge websocket upgrade" || bad "judge websocket upgrade ($ws)"

if [ "$fail" -ne 0 ]; then
  echo "smoke: FAILED"
  exit 1
fi
echo "smoke: passed"
