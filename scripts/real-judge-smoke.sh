#!/bin/sh
set -eu

BASE_URL=${BASE_URL:-http://127.0.0.1:18080}
OJ_USERNAME=${OJ_USERNAME:-admin}
OJ_PASSWORD=${OJ_PASSWORD:-adminpass}
PROBLEM_PID=${PROBLEM_PID:-PJUDGE}
TIMEOUT=${TIMEOUT:-120}

cookie_jar=$(mktemp)
trap 'rm -f "$cookie_jar"' EXIT

curl -fsS -c "$cookie_jar" \
  -H 'Content-Type: application/json' \
  -d "$(jq -nc --arg username "$OJ_USERNAME" --arg password "$OJ_PASSWORD" '{username:$username,password:$password}')" \
  "$BASE_URL/api/v1/auth/login" >/dev/null

code='#include <iostream>
int main() { int a = 0, b = 0; std::cin >> a >> b; std::cout << a + b << "\n"; }
'
created=$(curl -fsS -b "$cookie_jar" \
  -H 'Content-Type: application/json' \
  -d "$(jq -nc --arg pid "$PROBLEM_PID" --arg code "$code" '{pid:$pid,lang:"cpp",code:$code}')" \
  "$BASE_URL/api/v1/submissions")
rid=$(printf '%s' "$created" | jq -er '.data.id')
printf 'submission: %s\n' "$rid"

elapsed=0
while [ "$elapsed" -lt "$TIMEOUT" ]; do
  record=$(curl -fsS -b "$cookie_jar" "$BASE_URL/api/v1/submissions/$rid")
  status=$(printf '%s' "$record" | jq -er '.data.status')
  score=$(printf '%s' "$record" | jq -er '.data.score')
  printf 'status=%s score=%s\n' "$status" "$score"
  if [ "$status" -eq 1 ] && [ "$score" -eq 100 ]; then
    printf '%s\n' "$record" | jq .
    exit 0
  fi
  case "$status" in
    0|20|21) ;;
    *)
      printf '%s\n' "$record" | jq .
      exit 1
      ;;
  esac
  sleep 2
  elapsed=$((elapsed + 2))
done

printf 'judge did not finish within %ss\n' "$TIMEOUT" >&2
exit 1
