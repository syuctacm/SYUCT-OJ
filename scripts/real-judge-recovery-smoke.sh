#!/bin/sh
set -eu

BASE_URL=${BASE_URL:-http://127.0.0.1:18080}
JUDGE_CONTAINER=${JUDGE_CONTAINER:-syuct-oj-dev-judge}
OJ_USERNAME=${OJ_USERNAME:-admin}
OJ_PASSWORD=${OJ_PASSWORD:-adminpass}
PROBLEM_PID=${PROBLEM_PID:-PJUDGE}
LEASE_WAIT=${LEASE_WAIT:-5}
TIMEOUT=${TIMEOUT:-90}

cookie_jar=$(mktemp)
stopped=0
cleanup() {
  rm -f "$cookie_jar"
  if [ "$stopped" -eq 1 ]; then
    docker start "$JUDGE_CONTAINER" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

wait_for_judge() {
  since=$(date -u +%Y-%m-%dT%H:%M:%SZ)
  i=0
  while [ "$i" -lt 20 ]; do
    if docker logs --since "$since" "$JUDGE_CONTAINER" 2>&1 | grep -q '已连接'; then
      return 0
    fi
    sleep 1
    i=$((i + 1))
  done
  echo 'judge did not reconnect' >&2
  return 1
}

docker inspect "$JUDGE_CONTAINER" >/dev/null
docker restart "$JUDGE_CONTAINER" >/dev/null
wait_for_judge

curl -fsS -c "$cookie_jar" \
  -H 'Content-Type: application/json' \
  -d "$(jq -nc --arg username "$OJ_USERNAME" --arg password "$OJ_PASSWORD" '{username:$username,password:$password}')" \
  "$BASE_URL/api/v1/auth/login" >/dev/null

code='#include <iostream>
#include <chrono>
#include <thread>
int main(){std::this_thread::sleep_for(std::chrono::seconds(8));int a,b;std::cin>>a>>b;std::cout<<a+b<<std::endl;}'
created=$(curl -fsS -b "$cookie_jar" \
  -H 'Content-Type: application/json' \
  -d "$(jq -nc --arg pid "$PROBLEM_PID" --arg code "$code" '{pid:$pid,lang:"cpp",code:$code}')" \
  "$BASE_URL/api/v1/submissions")
rid=$(printf '%s' "$created" | jq -er '.data.id')
printf 'submission: %s\n' "$rid"

saw_running=0
elapsed=0
while [ "$elapsed" -lt "$TIMEOUT" ]; do
  record=$(curl -fsS -b "$cookie_jar" "$BASE_URL/api/v1/submissions/$rid")
  st=$(printf '%s' "$record" | jq -er '.data.status')
  if [ "$st" -eq 20 ]; then
    saw_running=1
    break
  fi
  case "$st" in
    0|21) ;;
    *) printf '%s\n' "$record" | jq .; exit 1 ;;
  esac
  sleep 0.2
  elapsed=$((elapsed + 1))
done
if [ "$saw_running" -ne 1 ]; then
  echo 'submission never reached judging state' >&2
  exit 1
fi

printf 'stopping judge container while submission is running\n'
docker stop "$JUDGE_CONTAINER" >/dev/null
sleep "$LEASE_WAIT"
printf 'restarting judge container\n'
docker start "$JUDGE_CONTAINER" >/dev/null
sleep 4

elapsed=0
while [ "$elapsed" -lt "$TIMEOUT" ]; do
  record=$(curl -fsS -b "$cookie_jar" "$BASE_URL/api/v1/submissions/$rid")
  st=$(printf '%s' "$record" | jq -er '.data.status')
  score=$(printf '%s' "$record" | jq -er '.data.score')
  if [ "$st" -eq 1 ] && [ "$score" -eq 100 ]; then
    attempts=$(docker logs "$JUDGE_CONTAINER" 2>&1 | grep -c "Submission: .*$rid" || true)
    printf 'judge executions observed: %s\n' "$attempts"
    if [ "$attempts" -lt 2 ]; then
      echo 'task was not retried after judge disconnect' >&2
      exit 1
    fi
    printf '%s\n' "$record" | jq .
    exit 0
  fi
  case "$st" in
    0|20|21) ;;
    *) printf '%s\n' "$record" | jq .; exit 1 ;;
  esac
  sleep 1
  elapsed=$((elapsed + 1))
done

printf 'recovered submission did not finish within %ss\n' "$TIMEOUT" >&2
exit 1
