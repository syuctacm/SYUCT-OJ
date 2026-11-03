#!/bin/sh
set -eu

BASE_URL=${BASE_URL:-http://127.0.0.1:18080}
MONGO_CONTAINER=${MONGO_CONTAINER:-syuct-oj-dev-mongo}
MONGO_USERNAME=${MONGO_ROOT_USERNAME:-syuct}
MONGO_PASSWORD=${MONGO_ROOT_PASSWORD:-syuct_dev_password}
MONGO_DATABASE=${MONGO_DATABASE:-syuct_oj_judge_hack_smoke}
JUDGE_FILE_ROOT=${JUDGE_FILE_ROOT:-./data/judge-files}
OJ_PASSWORD=${OJ_PASSWORD:-secret}
TIMEOUT=${TIMEOUT:-180}

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

PROBLEM_DIR="$JUDGE_FILE_ROOT/problem/system/9201/testdata"
mkdir -p "$PROBLEM_DIR"
printf '7 8\n' > "$PROBLEM_DIR/1.in"
printf '15\n' > "$PROBLEM_DIR/1.out"
cat > "$PROBLEM_DIR/validator.cc" <<'CPP'
#include "testlib.h"

int main(int argc, char *argv[]) {
    registerValidation(argc, argv);
    inf.strict = false;
    long long a = inf.readLong();
    long long b = inf.readLong();
    ensuref(a >= -2000000000LL && a <= 2000000000LL, "a is out of range");
    ensuref(b >= -2000000000LL && b <= 2000000000LL, "b is out of range");
    inf.skipBlanks();
    inf.readEof();
}
CPP
cat > "$PROBLEM_DIR/checker.cc" <<'CPP'
#include "testlib.h"

int main(int argc, char *argv[]) {
    registerTestlibCmd(argc, argv);
    long long a = inf.readLong();
    long long b = inf.readLong();
    long long expected = a + b;
    long long got = ouf.readLong();
    if (got != expected) quitf(_wa, "expected %lld, found %lld", expected, got);
    quitf(_ok, "sum is %lld", got);
}
CPP

IN_SIZE=$(wc -c < "$PROBLEM_DIR/1.in")
OUT_SIZE=$(wc -c < "$PROBLEM_DIR/1.out")
VALIDATOR_SIZE=$(wc -c < "$PROBLEM_DIR/validator.cc")
CHECKER_SIZE=$(wc -c < "$PROBLEM_DIR/checker.cc")
ETAG=$(date +%s)

docker exec -i \
  -e IN_SIZE="$IN_SIZE" \
  -e OUT_SIZE="$OUT_SIZE" \
  -e VALIDATOR_SIZE="$VALIDATOR_SIZE" \
  -e CHECKER_SIZE="$CHECKER_SIZE" \
  -e ETAG="$ETAG" \
  "$MONGO_CONTAINER" mongosh \
  "mongodb://$MONGO_USERNAME:$MONGO_PASSWORD@127.0.0.1:27017/$MONGO_DATABASE?authSource=admin" \
  --quiet --file /dev/stdin <<'MONGO'
const now = new Date();
const hash = '3fa094211c0cf2ed1d332ab43adc69aab469f0e0f2cae6345c81bb874eef3f9e';
db.record.deleteMany({ pid: NumberLong("9201") });
db.task.deleteMany({ pid: NumberLong("9201") });
db.document.deleteMany({ docType: NumberLong("10"), pid: 'PHACK' });
for (const user of [
  { id: 1, name: 'admin' },
  { id: 2, name: 'alice' },
  { id: 3, name: 'bob' },
]) {
  db.user.updateOne(
    { _id: NumberLong(String(user.id)) },
    {
      $set: {
        uname: user.name, unameLower: user.name,
        mail: `${user.name}@example.com`, mailLower: `${user.name}@example.com`,
        salt: 'salt', hash, hashType: 'hydro', priv: NumberLong("1"),
        regat: now, loginat: now, viewLang: 'zh',
      },
    },
    { upsert: true },
  );
}
db.document.insertOne({
  _id: ObjectId(), domainId: 'system', docType: NumberLong("10"), docId: NumberLong("9201"), pid: 'PHACK',
  title: 'Hackable A+B', content: 'Hack smoke test', hidden: false, owner: NumberLong("1"), tag: ['judge'],
  nSubmit: NumberLong("0"), nAccept: NumberLong("0"),
  config: 'type: default\ntime: 3000ms\nmemory: 256m\nchecker_type: testlib\nchecker: checker.cc\nvalidator: validator.cc\nsubtasks:\n  - score: 100\n    type: min\n    cases:\n      - input: 1.in\n        output: 1.out\n',
  data: [
    { _id: 'problem/system/9201/testdata/1.in', name: '1.in', size: NumberLong(process.env.IN_SIZE), etag: process.env.ETAG, lastModified: now },
    { _id: 'problem/system/9201/testdata/1.out', name: '1.out', size: NumberLong(process.env.OUT_SIZE), etag: process.env.ETAG, lastModified: now },
    { _id: 'problem/system/9201/testdata/validator.cc', name: 'validator.cc', size: NumberLong(process.env.VALIDATOR_SIZE), etag: process.env.ETAG, lastModified: now },
    { _id: 'problem/system/9201/testdata/checker.cc', name: 'checker.cc', size: NumberLong(process.env.CHECKER_SIZE), etag: process.env.ETAG, lastModified: now },
  ],
});
MONGO

login() {
  user=$1
  cookie=$2
  curl -fsS -c "$cookie" \
    -H 'Content-Type: application/json' \
    -d "$(jq -nc --arg username "$user" --arg password "$OJ_PASSWORD" '{username:$username,password:$password}')" \
    "$BASE_URL/api/v1/auth/login" >/dev/null
}

ADMIN_COOKIE="$JUDGE_FILE_ROOT/real-judge-hack-admin-cookie.txt"
ALICE_COOKIE="$JUDGE_FILE_ROOT/real-judge-hack-alice-cookie.txt"
BOB_COOKIE="$JUDGE_FILE_ROOT/real-judge-hack-bob-cookie.txt"
login admin "$ADMIN_COOKIE"
login alice "$ALICE_COOKIE"
login bob "$BOB_COOKIE"

submit() {
  cookie=$1
  code=$2
  curl -fsS -b "$cookie" \
    -H 'Content-Type: application/json' \
    -d "$(jq -nc --arg code "$code" '{pid:"PHACK",lang:"cpp",code:$code}')" \
    "$BASE_URL/api/v1/submissions" | jq -er '.data.id'
}

wait_record() {
  rid=$1
  cookie=$2
  elapsed=0
  while [ "$elapsed" -lt "$TIMEOUT" ]; do
    record=$(curl -fsS -b "$cookie" "$BASE_URL/api/v1/submissions/$rid")
    st=$(printf '%s' "$record" | jq -er '.data.status')
    if [ "$st" -ne 0 ] && [ "$st" -ne 20 ] && [ "$st" -ne 21 ] && [ "$st" -ne 22 ]; then
      printf '%s\n' "$record"
      return 0
    fi
    sleep 2
    elapsed=$((elapsed + 2))
  done
  fail "record $rid did not finish within ${TIMEOUT}s"
}

expect_record() {
  label=$1
  expected_status=$2
  record=$3
  st=$(printf '%s' "$record" | jq -er '.data.status')
  if [ "$st" -ne "$expected_status" ]; then
    printf '%s\n' "$record" | jq . >&2
    fail "$label status=$st, want $expected_status"
  fi
  printf '%s: status=%s\n' "$label" "$st"
}

BUGGY_CODE='#include <iostream>
int main(){int a,b;std::cin>>a>>b;std::cout<<a+b<<"\n";}'
CORRECT_CODE='#include <iostream>
int main(){long long a,b;std::cin>>a>>b;std::cout<<a+b<<"\n";}'

TARGET_RID=$(submit "$ALICE_COOKIE" "$BUGGY_CODE")
expect_record 'initial buggy target' 1 "$(wait_record "$TARGET_RID" "$ALICE_COOKIE")"
OWN_RID=$(submit "$ADMIN_COOKIE" "$CORRECT_CODE")
expect_record 'initial actor solution' 1 "$(wait_record "$OWN_RID" "$ADMIN_COOKIE")"

self_status=$(curl -sS -o /tmp/hack-self.json -w '%{http_code}' -b "$ADMIN_COOKIE" \
  -H 'Content-Type: application/json' \
  -d '{"input":"1 2\n"}' \
  "$BASE_URL/api/v1/problems/PHACK/hack/$OWN_RID")
[ "$self_status" = 400 ] || fail "self hack status=$self_status: $(cat /tmp/hack-self.json)"

no_accept_status=$(curl -sS -o /tmp/hack-no-accept.json -w '%{http_code}' -b "$BOB_COOKIE" \
  -H 'Content-Type: application/json' \
  -d '{"input":"1 2\n"}' \
  "$BASE_URL/api/v1/problems/PHACK/hack/$TARGET_RID")
[ "$no_accept_status" = 400 ] || fail "no-accept hack status=$no_accept_status: $(cat /tmp/hack-no-accept.json)"

HACK_RID=$(curl -fsS -b "$ADMIN_COOKIE" \
  -H 'Content-Type: application/json' \
  -d "$(jq -nc --arg input '2000000000 2000000000
' '{input:$input}')" \
  "$BASE_URL/api/v1/problems/PHACK/hack/$TARGET_RID" | jq -er '.data.id')
SUCCESS_RECORD=$(wait_record "$HACK_RID" "$ADMIN_COOKIE")
expect_record 'successful hack record' 32 "$SUCCESS_RECORD"
printf '%s\n' "$SUCCESS_RECORD" | jq -e --arg target "$TARGET_RID" '.data.hackTarget == $target and (.data.files.hack | startswith("hack-"))' >/dev/null \
  || fail "successful hack metadata: $SUCCESS_RECORD"

hack_input=$(curl -fsS -b "$ADMIN_COOKIE" "$BASE_URL/api/v1/submissions/$HACK_RID/hack")
[ "$hack_input" = '2000000000 2000000000' ] || fail "downloaded hack input = $hack_input"

TARGET_AFTER=$(wait_record "$TARGET_RID" "$ALICE_COOKIE")
expect_record 'target after successful hack' 11 "$TARGET_AFTER"
OWN_AFTER=$(wait_record "$OWN_RID" "$ADMIN_COOKIE")
expect_record 'actor solution after hack rejudge' 1 "$OWN_AFTER"

stats=$(docker exec -i "$MONGO_CONTAINER" mongosh \
  "mongodb://$MONGO_USERNAME:$MONGO_PASSWORD@127.0.0.1:27017/$MONGO_DATABASE?authSource=admin" \
  --quiet --eval '
const p = db.document.findOne({ docType: NumberLong("10"), pid: "PHACK" });
print(JSON.stringify({
  nSubmit: Number(p.nSubmit), nAccept: Number(p.nAccept),
  data: (p.data || []).map((file) => file.name),
  config: String(p.config),
}));
')
printf '%s\n' "$stats" | jq -e '.nSubmit == 2 and .nAccept == 1 and any(.data[]; startswith("hack-")) and (.config | contains("output: /dev/null"))' >/dev/null \
  || fail "post-hack problem state: $stats"

SECOND_TARGET=$(submit "$ALICE_COOKIE" "$CORRECT_CODE")
expect_record 'second target after hack testcase' 1 "$(wait_record "$SECOND_TARGET" "$ALICE_COOKIE")"

HACK_INPUT_FILE="$JUDGE_FILE_ROOT/real-judge-hack-input.txt"
printf '  1   2 \n' > "$HACK_INPUT_FILE"
UNSUCCESSFUL_RID=$(curl -fsS -b "$ADMIN_COOKIE" \
  -F "file=@$HACK_INPUT_FILE" \
  -F 'autoOrganizeInput=on' \
  "$BASE_URL/api/v1/problems/PHACK/hack/$SECOND_TARGET" | jq -er '.data.id')
UNSUCCESSFUL_RECORD=$(wait_record "$UNSUCCESSFUL_RID" "$ADMIN_COOKIE")
expect_record 'unsuccessful hack record' 33 "$UNSUCCESSFUL_RECORD"
normalized=$(curl -fsS -b "$ADMIN_COOKIE" "$BASE_URL/api/v1/submissions/$UNSUCCESSFUL_RID/hack")
[ "$normalized" = '1 2' ] || fail "normalized hack input = $normalized"
SECOND_AFTER=$(wait_record "$SECOND_TARGET" "$ALICE_COOKIE")
expect_record 'target after unsuccessful hack' 1 "$SECOND_AFTER"

printf 'hack workflow smoke passed\n'
printf '%s\n' "$SUCCESS_RECORD" | jq '{hack:.data.id,target:.data.hackTarget,status:.data.status}'
printf '%s\n' "$UNSUCCESSFUL_RECORD" | jq '{hack:.data.id,target:.data.hackTarget,status:.data.status}'
