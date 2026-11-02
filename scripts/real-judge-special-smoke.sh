#!/bin/sh
set -eu

BASE_URL=${BASE_URL:-http://127.0.0.1:18080}
MONGO_CONTAINER=${MONGO_CONTAINER:-syuct-oj-dev-mongo}
MONGO_USERNAME=${MONGO_ROOT_USERNAME:-syuct}
MONGO_PASSWORD=${MONGO_ROOT_PASSWORD:-syuct_dev_password}
MONGO_DATABASE=${MONGO_DATABASE:-syuct_oj_judge_smoke}
JUDGE_FILE_ROOT=${JUDGE_FILE_ROOT:-./data/judge-files}
OJ_USERNAME=${OJ_USERNAME:-admin}
OJ_PASSWORD=${OJ_PASSWORD:-secret}
TIMEOUT=${TIMEOUT:-120}

fail() {
  printf 'ERROR: %s\n' "$*" >&2
  exit 1
}

command -v curl >/dev/null 2>&1 || fail 'curl is required'
command -v docker >/dev/null 2>&1 || fail 'docker is required'
command -v jq >/dev/null 2>&1 || fail 'jq is required'

case "$BASE_URL" in
  http://127.0.0.1:*|http://localhost:*) ;;
  *) fail 'BASE_URL must be a local HTTP endpoint' ;;
esac
case "$MONGO_DATABASE" in
  *test*|*smoke*|*e2e*) ;;
  *) fail 'MONGO_DATABASE must be a dedicated test/smoke database' ;;
esac
if [ "${ALLOW_DESTRUCTIVE_LOCAL_TEST:-0}" != 1 ]; then
  fail 'set ALLOW_DESTRUCTIVE_LOCAL_TEST=1 to reset the dedicated local test data'
fi

SPJ_DIR="$JUDGE_FILE_ROOT/problem/system/9101/testdata"
INTERACTIVE_DIR="$JUDGE_FILE_ROOT/problem/system/9102/testdata"
mkdir -p "$SPJ_DIR" "$INTERACTIVE_DIR"

printf '7 8\n' > "$SPJ_DIR/1.in"
printf '0\n' > "$SPJ_DIR/1.out"
cat > "$SPJ_DIR/chk.cc" <<'CPP'
#include "testlib.h"

int main(int argc, char *argv[]) {
    registerTestlibCmd(argc, argv);
    long long a = inf.readLong();
    long long b = inf.readLong();
    long long got = ouf.readLong();
    if (got != a + b) quitf(_wa, "expected %lld, found %lld", a + b, got);
    quitf(_ok, "special checker accepted %lld", got);
}
CPP

printf '1\n7 8\n' > "$INTERACTIVE_DIR/1.in"
: > "$INTERACTIVE_DIR/1.out"
cat > "$INTERACTIVE_DIR/interactor.cpp" <<'CPP'
#include "testlib.h"
#include <iostream>

using namespace std;

int main(int argc, char *argv[]) {
    setName("Interactive A+B");
    registerInteraction(argc, argv);
    int n = inf.readInt();
    for (int i = 0; i < n; i++) {
        int a = inf.readInt();
        int b = inf.readInt();
        cout << a << " " << b << endl;
        int got = ouf.readInt();
        if (got != a + b) quitf(_wa, "expected %d, found %d", a + b, got);
    }
    quitf(_ok, "%d queries processed", n);
}
CPP

SPJ_IN_SIZE=$(wc -c < "$SPJ_DIR/1.in")
SPJ_OUT_SIZE=$(wc -c < "$SPJ_DIR/1.out")
SPJ_CHECKER_SIZE=$(wc -c < "$SPJ_DIR/chk.cc")
INTERACTIVE_IN_SIZE=$(wc -c < "$INTERACTIVE_DIR/1.in")
INTERACTIVE_OUT_SIZE=$(wc -c < "$INTERACTIVE_DIR/1.out")
INTERACTOR_SIZE=$(wc -c < "$INTERACTIVE_DIR/interactor.cpp")

docker exec -i \
  -e SPJ_IN_SIZE="$SPJ_IN_SIZE" \
  -e SPJ_OUT_SIZE="$SPJ_OUT_SIZE" \
  -e SPJ_CHECKER_SIZE="$SPJ_CHECKER_SIZE" \
  -e INTERACTIVE_IN_SIZE="$INTERACTIVE_IN_SIZE" \
  -e INTERACTIVE_OUT_SIZE="$INTERACTIVE_OUT_SIZE" \
  -e INTERACTOR_SIZE="$INTERACTOR_SIZE" \
  "$MONGO_CONTAINER" mongosh \
  "mongodb://$MONGO_USERNAME:$MONGO_PASSWORD@127.0.0.1:27017/$MONGO_DATABASE?authSource=admin" \
  --quiet --file /dev/stdin <<'MONGO'
const now = new Date();
db.record.deleteMany({ pid: { $in: [NumberLong("9101"), NumberLong("9102")] } });
db.task.deleteMany({ pid: { $in: [NumberLong("9101"), NumberLong("9102")] } });
db.document.deleteMany({ docType: NumberLong("10"), pid: { $in: ['PJUDGE_SPJ', 'PJUDGE_INT'] } });
db.user.updateOne(
  { _id: NumberLong("1") },
  {
    $set: {
      uname: 'admin', unameLower: 'admin',
      mail: 'admin@example.com', mailLower: 'admin@example.com',
      salt: 'salt', hash: '3fa094211c0cf2ed1d332ab43adc69aab469f0e0f2cae6345c81bb874eef3f9e', hashType: 'hydro',
      priv: NumberLong("1"), regat: now, loginat: now, viewLang: 'zh',
    },
  },
  { upsert: true },
);
db.document.insertMany([
  {
    _id: ObjectId(), domainId: 'system', docType: NumberLong("10"), docId: NumberLong("9101"), pid: 'PJUDGE_SPJ',
    title: 'Special checker A+B', content: 'Special checker smoke test', hidden: false, owner: NumberLong("1"), tag: ['judge'],
    nSubmit: NumberLong("0"), nAccept: NumberLong("0"),
    config: 'type: default\ntime: 3000ms\nmemory: 256m\nchecker_type: testlib\nchecker: chk.cc\nsubtasks:\n  - score: 100\n    type: min\n    cases:\n      - input: 1.in\n        output: 1.out\n',
    data: [
      { _id: 'problem/system/9101/testdata/1.in', name: '1.in', size: NumberLong(process.env.SPJ_IN_SIZE), etag: 'special-smoke-v1', lastModified: now },
      { _id: 'problem/system/9101/testdata/1.out', name: '1.out', size: NumberLong(process.env.SPJ_OUT_SIZE), etag: 'special-smoke-v1', lastModified: now },
      { _id: 'problem/system/9101/testdata/chk.cc', name: 'chk.cc', size: NumberLong(process.env.SPJ_CHECKER_SIZE), etag: 'special-smoke-v1', lastModified: now },
    ],
  },
  {
    _id: ObjectId(), domainId: 'system', docType: NumberLong("10"), docId: NumberLong("9102"), pid: 'PJUDGE_INT',
    title: 'Interactive A+B', content: 'Interactive smoke test', hidden: false, owner: NumberLong("1"), tag: ['judge'],
    nSubmit: NumberLong("0"), nAccept: NumberLong("0"),
    config: 'type: interactive\ntime: 3000ms\nmemory: 256m\ninteractor: interactor.cpp\nsubtasks:\n  - score: 100\n    type: min\n    cases:\n      - input: 1.in\n        output: 1.out\n',
    data: [
      { _id: 'problem/system/9102/testdata/1.in', name: '1.in', size: NumberLong(process.env.INTERACTIVE_IN_SIZE), etag: 'special-smoke-v1', lastModified: now },
      { _id: 'problem/system/9102/testdata/1.out', name: '1.out', size: NumberLong(process.env.INTERACTIVE_OUT_SIZE), etag: 'special-smoke-v1', lastModified: now },
      { _id: 'problem/system/9102/testdata/interactor.cpp', name: 'interactor.cpp', size: NumberLong(process.env.INTERACTOR_SIZE), etag: 'special-smoke-v1', lastModified: now },
    ],
  },
]);
MONGO

COOKIE_FILE="$JUDGE_FILE_ROOT/real-judge-special-cookie.txt"
curl -fsS -c "$COOKIE_FILE" \
  -H 'Content-Type: application/json' \
  -d "$(jq -nc --arg username "$OJ_USERNAME" --arg password "$OJ_PASSWORD" '{username:$username,password:$password}')" \
  "$BASE_URL/api/v1/auth/login" >/dev/null

submit_and_wait() {
  pid=$1
  code=$2
  payload=$(jq -nc --arg pid "$pid" --arg code "$code" '{pid:$pid,lang:"cpp",code:$code}')
  rid=$(curl -fsS -b "$COOKIE_FILE" \
    -H 'Content-Type: application/json' \
    -d "$payload" \
    "$BASE_URL/api/v1/submissions" | jq -er '.data.id')
  printf 'submission %s: %s\n' "$pid" "$rid" >&2
  elapsed=0
  while [ "$elapsed" -lt "$TIMEOUT" ]; do
    record=$(curl -fsS -b "$COOKIE_FILE" "$BASE_URL/api/v1/submissions/$rid")
    st=$(printf '%s' "$record" | jq -er '.data.status')
    if [ "$st" -ne 0 ] && [ "$st" -ne 20 ] && [ "$st" -ne 21 ]; then
      printf '%s\n' "$record"
      return 0
    fi
    sleep 2
    elapsed=$((elapsed + 2))
  done
  fail "submission $rid did not finish within ${TIMEOUT}s"
}

assert_submission() {
  label=$1
  expected_status=$2
  expected_score=$3
  expected_text=$4
  record=$5
  st=$(printf '%s' "$record" | jq -er '.data.status')
  score=$(printf '%s' "$record" | jq -er '.data.score')
  if [ "$st" -ne "$expected_status" ] || [ "$score" -ne "$expected_score" ]; then
    fail "$label status=$st score=$score, want status=$expected_status score=$expected_score"
  fi
  if ! printf '%s' "$record" | jq -e --arg text "$expected_text" '.. | strings | select(contains($text))' >/dev/null; then
    fail "$label output does not contain: $expected_text"
  fi
  printf '%s: status=%s score=%s\n' "$label" "$st" "$score"
}

AC_CODE='#include <iostream>
int main(){long long a,b;std::cin>>a>>b;std::cout<<a+b<<"\n";}'
WA_CODE='#include <iostream>
int main(){long long a,b;std::cin>>a>>b;std::cout<<a+b+1<<"\n";}'

record=$(submit_and_wait PJUDGE_SPJ "$AC_CODE")
assert_submission 'special checker AC' 1 100 'special checker accepted 15' "$record"

record=$(submit_and_wait PJUDGE_SPJ "$WA_CODE")
assert_submission 'special checker WA' 2 0 'expected 15, found 16' "$record"
SPJ_WA_RID=$(printf '%s' "$record" | jq -er '.data.id')

record=$(submit_and_wait PJUDGE_INT "$AC_CODE")
assert_submission 'interactive AC' 1 100 'queries processed' "$record"

record=$(submit_and_wait PJUDGE_INT "$WA_CODE")
assert_submission 'interactive WA' 2 0 'expected 15, found 16' "$record"

curl -fsS -b "$COOKIE_FILE" -X POST "$BASE_URL/api/v1/submissions/$SPJ_WA_RID/rejudge" >/dev/null
elapsed=0
rejudged_record=''
while [ "$elapsed" -lt "$TIMEOUT" ]; do
  record=$(curl -fsS -b "$COOKIE_FILE" "$BASE_URL/api/v1/submissions/$SPJ_WA_RID")
  st=$(printf '%s' "$record" | jq -er '.data.status')
  rejudged=$(printf '%s' "$record" | jq -er '.data.rejudged')
  if [ "$rejudged" = true ] && [ "$st" -ne 0 ] && [ "$st" -ne 20 ] && [ "$st" -ne 21 ]; then
    rejudged_record=$record
    break
  fi
  sleep 2
  elapsed=$((elapsed + 2))
done
[ -n "$rejudged_record" ] || fail "rejudge $SPJ_WA_RID did not finish within ${TIMEOUT}s"
assert_submission 'special checker rejudge' 2 0 'expected 15, found 16' "$rejudged_record"

stats=$(docker exec -i "$MONGO_CONTAINER" mongosh \
  "mongodb://$MONGO_USERNAME:$MONGO_PASSWORD@127.0.0.1:27017/$MONGO_DATABASE?authSource=admin" \
  --quiet --eval '
const result = {};
for (const p of db.document.find({ docType: NumberLong("10"), pid: { $in: ["PJUDGE_SPJ", "PJUDGE_INT"] } })) {
  result[p.pid] = { nSubmit: Number(p.nSubmit), nAccept: Number(p.nAccept), records: db.record.countDocuments({ pid: p.docId }) };
}
print(JSON.stringify(result));
')
printf '%s\n' "$stats" | jq -e '.PJUDGE_SPJ.nSubmit == 2 and .PJUDGE_SPJ.nAccept == 1 and .PJUDGE_SPJ.records == 2' >/dev/null \
  || fail "unexpected special checker statistics: $stats"
printf '%s\n' "$stats" | jq -e '.PJUDGE_INT.nSubmit == 2 and .PJUDGE_INT.nAccept == 1 and .PJUDGE_INT.records == 2' >/dev/null \
  || fail "unexpected interactive statistics: $stats"

printf 'special checker and interactive smoke passed\n'
printf '%s\n' "$stats" | jq .
