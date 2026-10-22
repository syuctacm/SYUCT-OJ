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

mongo_eval_db() {
  local database=$1
  docker exec "$MONGO_CONTAINER" mongosh --quiet \
    -u "$MONGO_ROOT_USERNAME" -p "$MONGO_ROOT_PASSWORD" --authenticationDatabase admin \
    "$database" --eval "$2"
}

mongo_eval() {
  mongo_eval_db "$DRILL_DATABASE" "$1"
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
  { _id: ObjectId(), domainId: "contest2025", docType: 10, docId: Long(2), pid: "P1", title: "Contest Problem" },
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
  contestProblem: (() => {
    const p = db.document.findOne({domainId: "system", docType: 10, docId: Long(2)});
    return p ? { pid: p.pid, legacyPids: p.legacyPids } : null;
  })(),
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
contest = verify["contestProblem"] or {}
check(contest.get("pid") == "P2", f"colliding pid should be renamed to P2, got {contest.get('pid')}")
check(contest.get("legacyPids") == ["P1"], f"legacyPids should keep P1, got {contest.get('legacyPids')}")
check(len(execute.get("problems_renamed") or []) == 1, f"problems_renamed should have 1 entry, got {execute.get('problems_renamed')}")
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

echo "==> Checking the docId collision guard"
COLLISION_DB="${DRILL_DATABASE}_collision"
mongo_eval_db "$COLLISION_DB" 'db.dropDatabase()' >/dev/null
mongo_eval_db "$COLLISION_DB" "$(cat <<'JS'
db.document.insertMany([
  { _id: ObjectId(), domainId: "system", docType: 10, docId: Long(1), pid: "P1", title: "System" },
  { _id: ObjectId(), domainId: "contest2025", docType: 10, docId: Long(1), pid: "C1", title: "Contest" },
]);
db.domain.insertOne({ _id: "contest2025", name: "contest2025" });
JS
)" >/dev/null
if MONGO_DATABASE="$COLLISION_DB" MONGO_URI="$MONGO_URI" \
  go run ./cmd/migrate --execute --report "$TMP_DIR/collision.json" >/dev/null 2>&1; then
  echo "ERROR: migration accepted a docId collision" >&2
  exit 1
fi
collision_remaining=$(mongo_eval_db "$COLLISION_DB" 'print(db.document.countDocuments({domainId: "contest2025"}))' | tail -1)
if [ "$collision_remaining" != "1" ]; then
  echo "ERROR: docId collision modified source data (remaining=$collision_remaining)" >&2
  exit 1
fi
collision_domain=$(mongo_eval_db "$COLLISION_DB" 'print(db.domain.countDocuments({_id: "contest2025"}))' | tail -1)
if [ "$collision_domain" != "1" ]; then
  echo "ERROR: docId collision archived the source domain" >&2
  exit 1
fi
mongo_eval_db "$COLLISION_DB" 'db.dropDatabase()' >/dev/null
echo "    docId collision correctly refused"

echo "==> Checking docId renumbering"
RENUMBER_DB="${DRILL_DATABASE}_renumber"
mongo_eval_db "$RENUMBER_DB" 'db.dropDatabase()' >/dev/null
mongo_eval_db "$RENUMBER_DB" "$(cat <<'JS'
db.domain.insertOne({ _id: "contest2025", name: "contest2025" });
db.document.insertMany([
  { _id: ObjectId(), domainId: "system", docType: 10, docId: Long(1), pid: "P1", title: "System" },
  { _id: ObjectId(), domainId: "contest2025", docType: 10, docId: Long(1), pid: "C1", title: "Contest" },
  { _id: ObjectId(), domainId: "contest2025", docType: 11, parentType: 10, parentId: Long(1), content: "sol" },
  { _id: ObjectId(), domainId: "contest2025", docType: 30, docId: ObjectId(), pids: [Long(1)] },
  { _id: ObjectId(), domainId: "contest2025", docType: 40, docId: ObjectId(), dag: [{ pids: [Long(1)] }] },
]);
db.record.insertOne({ domainId: "contest2025", pid: Long(1), uid: Long(7) });
db.getCollection("document.status").insertOne({ domainId: "contest2025", docType: 10, docId: Long(1), uid: Long(7) });
db.storage.insertOne({ _id: ObjectId(), path: "problem/contest2025/1/testdata/1.in" });
JS
)" >/dev/null
MONGO_DATABASE="$RENUMBER_DB" MONGO_URI="$MONGO_URI" \
  go run ./cmd/migrate --execute --renumber-doc-ids --report "$TMP_DIR/renumber.json" >/dev/null
RENUMBER=$(mongo_eval_db "$RENUMBER_DB" 'print(JSON.stringify({
  problem: (() => { const p = db.document.findOne({docType: 10, pid: "C1"}); return p ? { docId: Number(p.docId), legacyDocId: Number(p.legacyDocId), domainId: p.domainId } : null; })(),
  recordPid: Number(db.record.findOne({}).pid),
  statusDocId: Number(db.getCollection("document.status").findOne({}).docId),
  solutionParent: Number(db.document.findOne({docType: 11}).parentId),
  contestPids: db.document.findOne({docType: 30}).pids.map(Number),
  trainingPids: db.document.findOne({docType: 40}).dag[0].pids.map(Number),
  storagePath: db.storage.findOne({}).path,
}))' | tail -1)
echo "$RENUMBER"
python3 - "$RENUMBER" "$TMP_DIR/renumber.json" <<'PY'
import json
import sys

data = json.loads(sys.argv[1])
report = json.load(open(sys.argv[2]))
errors = []
problem = data.get("problem") or {}
if problem.get("docId") != 2 or problem.get("legacyDocId") != 1 or problem.get("domainId") != "system":
    errors.append(f"problem not renumbered: {problem}")
if data["recordPid"] != 2:
    errors.append(f"record pid = {data['recordPid']}")
if data["statusDocId"] != 2:
    errors.append(f"status docId = {data['statusDocId']}")
if data["solutionParent"] != 2:
    errors.append(f"solution parent = {data['solutionParent']}")
if data["contestPids"] != [2]:
    errors.append(f"contest pids = {data['contestPids']}")
if data["trainingPids"] != [2]:
    errors.append(f"training pids = {data['trainingPids']}")
if data["storagePath"] != "problem/system/2/testdata/1.in":
    errors.append(f"storage path = {data['storagePath']}")
if len(report.get("problems_renumbered") or []) != 1:
    errors.append(f"report problems_renumbered = {report.get('problems_renumbered')}")
if errors:
    print("Renumber drill FAILED:", file=sys.stderr)
    for error in errors:
        print("  -", error, file=sys.stderr)
    sys.exit(1)
print("    docId renumbering correct")
PY
mongo_eval_db "$RENUMBER_DB" 'db.dropDatabase()' >/dev/null

if [[ "$KEEP" != "1" ]]; then
  echo "==> Dropping drill database"
  mongo_eval 'db.dropDatabase()' >/dev/null
else
  echo "==> Keeping drill database (KEEP=1)"
fi
