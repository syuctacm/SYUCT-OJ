#!/usr/bin/env bash
# Local-only migration drill for the single-site flatten migration.
#
# Seeds a throwaway database inside the dev Docker MongoDB, runs the migration
# in dry-run mode, executes it, verifies the result, then reruns it to prove
# idempotency. It never points at the production database.
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$ROOT_DIR"

MONGO_CONTAINER=${MONGO_CONTAINER:-syuct-oj-dev-mongo}
MONGO_ROOT_USERNAME=${MONGO_ROOT_USERNAME:-syuct}
MONGO_ROOT_PASSWORD=${MONGO_ROOT_PASSWORD:-syuct_dev_password}
DRILL_DATABASE=${DRILL_DATABASE:-syuct_oj_migration_drill}
MONGO_URI=${MONGO_URI:-mongodb://${MONGO_ROOT_USERNAME}:${MONGO_ROOT_PASSWORD}@127.0.0.1:27017/?authSource=admin}
KEEP=${KEEP:-0}
TMP_DIR=$(mktemp -d)
trap 'rm -rf "$TMP_DIR"' EXIT

if ! docker inspect -f '{{.State.Running}}' "$MONGO_CONTAINER" >/dev/null 2>&1; then
  echo "error: $MONGO_CONTAINER is not running; start it with: docker compose -f deploy/compose.dev.yaml up -d" >&2
  exit 1
fi

mongo_eval() {
  docker exec "$MONGO_CONTAINER" mongosh --quiet \
    -u "$MONGO_ROOT_USERNAME" -p "$MONGO_ROOT_PASSWORD" --authenticationDatabase admin \
    "$DRILL_DATABASE" --eval "$1"
}

echo "==> Seeding synthetic multi-domain data into $DRILL_DATABASE"
mongo_eval "$(cat <<'JS'
db.dropDatabase();
db.domain.insertMany([
  { _id: "contest2025", name: "contest2025" },
  { _id: "select2025", name: "select2025" },
]);
db.document.insertMany([
  { _id: ObjectId(), domainId: "system", docType: 10, docId: Long(1), pid: "P1", title: "System Problem" },
  { _id: ObjectId(), domainId: "contest2025", docType: 10, docId: Long(2), pid: "C1", title: "Contest Problem" },
  { _id: ObjectId(), domainId: "select2025", docType: 30, docId: ObjectId(), title: "Select Contest", rule: "acm" },
]);
db.getCollection("document.status").insertOne({ domainId: "contest2025", docType: 40, docId: ObjectId(), uid: Long(7), enroll: 1 });
db.record.insertOne({ _id: ObjectId(), domainId: "contest2025", uid: Long(7), pid: Long(2), status: 1, score: 100 });
db.getCollection("record.stat").insertOne({ domainId: "select2025", uid: Long(8), nSubmit: Long(1) });
db.getCollection("record.history").insertOne({ domainId: "contest2025", docId: Long(2), uid: Long(7) });
db.getCollection("task").insertOne({ domainId: "select2025", type: "judge" });
db.getCollection("schedule").insertOne({ domainId: "contest2025", type: "contest" });
db.getCollection("oplog").insertOne({ domainId: "select2025", type: "test" });
db.getCollection("contest.balloon").insertOne({ domainId: "contest2025", uid: Long(7) });
db.getCollection("vjudge").insertOne({ domainId: "select2025", source: "codeforces" });
db.getCollection("vjudge.mount").insertOne({ domainId: "select2025", source: "luogu" });
db.storage.insertMany([
  { _id: ObjectId(), path: "problem/contest2025/2/testdata/1.in" },
  { _id: ObjectId(), path: "problem/system/1/testdata/1.in" },
]);
db.getCollection("domain.user").insertMany([
  { _id: ObjectId(), domainId: "contest2025", uid: Long(7), nSubmit: Long(3), rp: 1500, level: 3 },
  { _id: ObjectId(), domainId: "select2025", uid: Long(7), nSubmit: Long(2), rp: 1600, level: 4 },
  { _id: ObjectId(), domainId: "select2025", uid: Long(8), nSubmit: Long(1) },
]);
JS
)"

run_migrate() {
  local name=$1
  shift
  echo "==> Running migration: $name"
  MONGO_DATABASE="$DRILL_DATABASE" MONGO_URI="$MONGO_URI" \
    go run ./cmd/migrate "$@" --report "$TMP_DIR/$name.json" >/dev/null
}

run_migrate dry-run --dry-run
run_migrate execute --execute
run_migrate repeat --execute

echo "==> Verifying migrated data"
VERIFY=$(mongo_eval 'print(JSON.stringify({
  liveDomains: db.domain.countDocuments({}),
  archivedDomains: db.getCollection("domain.legacy").countDocuments({}),
  residualDocuments: db.document.countDocuments({domainId: {$in: ["contest2025", "select2025"]}}),
  legacyDocuments: db.document.countDocuments({legacyDomains: {$in: ["contest2025", "select2025"]}}),
  systemDocuments: db.document.countDocuments({domainId: "system"}),
  residualRecords: db.record.countDocuments({domainId: {$in: ["contest2025", "select2025"]}}),
  systemRecords: db.record.countDocuments({domainId: "system"}),
  residualStorage: db.storage.countDocuments({path: /^(problem|contest|training)\/(contest2025|select2025)\//}),
  systemStorage: db.storage.countDocuments({path: /^(problem|contest|training)\/system\//}),
  mergedUsers: db.getCollection("domain.user").countDocuments({domainId: "system"}),
  mergedUid7: (() => {
    const u = db.getCollection("domain.user").findOne({domainId: "system", uid: Long(7)});
    if (!u) return null;
    return { nSubmit: Number(u.nSubmit), rp: Number(u.rp), level: Number(u.level), legacyDomains: u.legacyDomains };
  })(),
}))' | tail -1)
echo "$VERIFY"

python3 - "$TMP_DIR" "$VERIFY" <<'PY'
import json
import sys

tmp_dir, verify_raw = sys.argv[1], sys.argv[2]
verify = json.loads(verify_raw)
dry = json.load(open(f"{tmp_dir}/dry-run.json"))
execute = json.load(open(f"{tmp_dir}/execute.json"))
repeat = json.load(open(f"{tmp_dir}/repeat.json"))

errors = []

def check(condition, message):
    if not condition:
        errors.append(message)

check(verify["liveDomains"] == 0, f"live domains should be archived, got {verify['liveDomains']}")
check(verify["archivedDomains"] == 2, f"expected 2 archived domains, got {verify['archivedDomains']}")
check(verify["residualDocuments"] == 0, f"source documents remain: {verify['residualDocuments']}")
check(verify["legacyDocuments"] == 2, f"expected 2 documents with legacyDomains, got {verify['legacyDocuments']}")
check(verify["systemDocuments"] == 3, f"expected 3 system documents, got {verify['systemDocuments']}")
check(verify["residualRecords"] == 0, f"source records remain: {verify['residualRecords']}")
check(verify["systemRecords"] == 1, f"expected 1 system record, got {verify['systemRecords']}")
check(verify["residualStorage"] == 0, f"source storage paths remain: {verify['residualStorage']}")
check(verify["systemStorage"] == 2, f"expected 2 system storage paths, got {verify['systemStorage']}")
check(verify["mergedUsers"] == 2, f"expected 2 merged domain users, got {verify['mergedUsers']}")
uid7 = verify["mergedUid7"] or {}
check(uid7.get("nSubmit") == 5, f"uid 7 nSubmit should be merged to 5, got {uid7.get('nSubmit')}")
check(uid7.get("rp") == 1600, f"uid 7 rp should use max 1600, got {uid7.get('rp')}")
check(uid7.get("level") == 4, f"uid 7 level should use max 4, got {uid7.get('level')}")
check(sorted(uid7.get("legacyDomains") or []) == ["contest2025", "select2025"], f"uid 7 legacyDomains: {uid7.get('legacyDomains')}")

check(dry["dry_run"] is True, "dry-run report should set dry_run")
check(execute["dry_run"] is False, "execute report should not be a dry run")
check(execute["users_merged"] == 3, f"execute should merge 3 domain users, got {execute['users_merged']}")
check(bool(execute["validation"]) and execute["validation"]["ok"], "execute validation should pass")
check(repeat["users_merged"] == 0, f"repeat run should merge 0 users, got {repeat['users_merged']}")
check(bool(repeat["validation"]) and repeat["validation"]["ok"], "repeat validation should pass")
modified = sum(item.get("modified", 0) for item in repeat["collections"])
check(modified == 0, f"repeat run should modify 0 documents, got {modified}")

if errors:
    print("\nMigration drill FAILED:", file=sys.stderr)
    for error in errors:
        print(f"  - {error}", file=sys.stderr)
    sys.exit(1)

print("==> Migration drill passed")
PY

if [[ "$KEEP" != "1" ]]; then
  echo "==> Dropping drill database"
  mongo_eval 'db.dropDatabase()' >/dev/null
else
  echo "==> Keeping drill database (KEEP=1)"
fi
