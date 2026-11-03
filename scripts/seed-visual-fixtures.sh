#!/usr/bin/env bash
# Seed deterministic, synthetic data into the local visual-regression Mongo.
# Never point this script at a non-local database.
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$ROOT_DIR"

COMPOSE_FILE=${COMPOSE_FILE:-deploy/compose.visual.yaml}
MONGO_DATABASE=${VISUAL_MONGO_DATABASE:-syuct_oj_visual}
MONGO_USERNAME=${VISUAL_MONGO_USERNAME:-syuct}
MONGO_PASSWORD=${VISUAL_MONGO_PASSWORD:-syuct_visual_password}

if [[ "$MONGO_DATABASE" != "syuct_oj_visual" ]]; then
  echo "seed-visual-fixtures: refusing to overwrite database '$MONGO_DATABASE'; the visual harness only supports syuct_oj_visual" >&2
  exit 2
fi

if ! docker compose -f "$COMPOSE_FILE" ps --status running --services 2>/dev/null | grep -qx mongo; then
  echo "seed-visual-fixtures: visual Mongo is not running; start deploy/compose.visual.yaml first" >&2
  exit 2
fi

mongo_script=$(cat <<'MONGO'
if (db.getName() !== "syuct_oj_visual") {
  throw new Error(`refusing to seed unexpected database: ${db.getName()}`);
}

db.dropDatabase();

const salt = "salt";
// Hydro salt/hash pair for the local test password "secret".
const hash = "3fa094211c0cf2ed1d332ab43adc69aab469f0e0f2cae6345c81bb874eef3f9e";
const at = new Date("2026-11-03T08:00:00.000Z");
const earlier = new Date("2026-11-03T07:30:00.000Z");
const contestStart = new Date("2026-10-01T08:00:00.000Z");
const contestEnd = new Date("2026-10-01T11:00:00.000Z");
const homeworkStart = new Date("2026-10-02T08:00:00.000Z");
const homeworkEnd = new Date("2026-10-09T08:00:00.000Z");

const problem1 = ObjectId("6ae994800000000000001001");
const problem2 = ObjectId("6ae994800000000000001002");
const problem3 = ObjectId("6ae994800000000000001003");
const contestID = ObjectId("6ae994800000000000003001");
const homeworkID = ObjectId("6ae994800000000000003002");
const trainingID = ObjectId("6ae994800000000000004001");
const discussionID = ObjectId("6ae994800000000000002101");
const replyID = ObjectId("6ae994800000000000002201");

db.user.insertMany([
  {
    _id: NumberLong("1"), uname: "visual_admin", unameLower: "visual_admin",
    mail: "visual_admin@example.com", mailLower: "visual_admin@example.com",
    salt, hash, hashType: "hydro", priv: NumberLong("1"), regat: at, loginat: earlier,
    avatar: "url:/visual-avatar.svg", unreadMsg: NumberLong("0"), loginip: "127.0.0.1", tfa: "",
  },
  {
    _id: NumberLong("2"), uname: "visual_user", unameLower: "visual_user",
    mail: "visual_user@example.com", mailLower: "visual_user@example.com",
    salt, hash, hashType: "hydro", priv: NumberLong("0"), regat: at, loginat: earlier,
    avatar: "url:/visual-avatar.svg", unreadMsg: NumberLong("0"), bio: "用于截图回归的固定测试用户。",
    school: "SYUCT", _files: [],
  },
]);

db.getCollection("domain.user").insertMany([
  { domainId: "system", uid: NumberLong("1"), level: NumberInt("5"), nSubmit: NumberLong("12"), nAccept: NumberLong("9"), rp: NumberInt("1800") },
  { domainId: "system", uid: NumberLong("2"), level: NumberInt("3"), nSubmit: NumberLong("6"), nAccept: NumberLong("4"), rp: NumberInt("1450") },
]);

db.system.insertMany([
  { _id: "server.name", value: "SYUCT OJ Visual Fixture" },
  { _id: "bulletin", value: "截图回归固定公告" },
  { _id: "ui-default.about", value: "# 关于 SYUCT OJ\n\n这是用于前端截图回归的本地固定页面。" },
  { _id: "ui-default.footer_extra_html", value: "SYUCT ACM Visual Regression" },
  { _id: "ui-default.enableScratchpad", value: false },
  { _id: "hydrooj.homepage", value: "- width: 8\n  contest: 3\n  homework: 3\n  training: 3\n- width: 4\n  discussion_nodes: true\n  problem_search: true\n" },
  { _id: "hydrooj.langs", value: "cpp:\n  display: C++17\n  compile: /usr/bin/g++ -O2 -std=c++17 -o foo foo.cpp\n  code_file: foo.cpp\n  execute: /w/foo\ncc:\n  display: C++\n  compile: /usr/bin/g++ -O2 -std=c++17 -o foo foo.cc\n  code_file: foo.cc\n  execute: /w/foo\n  hidden: true\npython3:\n  display: Python 3\n  compile: /usr/bin/python3 -m py_compile foo.py\n  code_file: foo.py\n  execute: /usr/bin/python3 foo.py\npy:\n  display: Python\n  code_file: foo.py\n  execute: /usr/bin/python3 foo.py\n  hidden: true\n" },
  { _id: "pagination.problem", value: 100 },
  { _id: "pagination.contest", value: 20 },
  { _id: "pagination.discussion", value: 50 },
  { _id: "pagination.record", value: 100 },
  { _id: "pagination.ranking", value: 100 },
  { _id: "pagination.solution", value: 20 },
  { _id: "pagination.training", value: 10 },
  { _id: "limit.user_files", value: 100 },
  { _id: "limit.user_files_size", value: 134217728 },
  { _id: "limit.contest_files", value: 100 },
  { _id: "limit.contest_files_size", value: 134217728 },
  { _id: "limit.problem_files_max", value: 100 },
  { _id: "limit.problem_files_max_size", value: 268435456 },
  { _id: "limit.codelength", value: 131072 },
  { _id: "limit.submission", value: 60 },
  { _id: "limit.submission_user", value: 15 },
  { _id: "limit.pretest", value: 60 },
  { _id: "limit.by_user", value: false },
  { _id: "session.saved_expire_seconds", value: 2592000 },
  { _id: "session.unsaved_expire_seconds", value: 10800 },
]);

db.document.insertMany([
  {
    _id: problem1, domainId: "system", docType: NumberInt("10"), docId: NumberLong("1001"),
    pid: "P1001", title: "视觉回归：区间求和", difficulty: NumberInt("2"),
    nSubmit: NumberLong("9"), nAccept: NumberLong("6"), hidden: false, owner: NumberLong("1"),
    config: "type: default\ntime: 1000ms\nmemory: 256m\n",
    content: "# 区间求和\n\n给定一个整数序列，输出区间 `[l, r]` 的元素和。\n\n## 输入格式\n\n第一行两个整数 `n, q`。\n\n## 样例\n\n```input1\n5 1\n1 2 3 4 5\n1 5\n```\n\n```output1\n15\n```\n",
    tag: ["前缀和", "基础"], data: [], additional_file: [],
  },
  {
    _id: problem2, domainId: "system", docType: NumberInt("10"), docId: NumberLong("1002"),
    pid: "P1002", title: "视觉回归：二分查找", difficulty: NumberInt("3"),
    nSubmit: NumberLong("7"), nAccept: NumberLong("3"), hidden: false, owner: NumberLong("1"),
    config: "type: default\ntime: 1000ms\nmemory: 256m\n",
    content: "# 二分查找\n\n在有序数组中查找目标值。\n",
    tag: ["二分", "基础"], data: [], additional_file: [],
  },
  {
    _id: problem3, domainId: "system", docType: NumberInt("10"), docId: NumberLong("1003"),
    pid: "P1003", title: "视觉回归：最短路径", difficulty: NumberInt("4"),
    nSubmit: NumberLong("4"), nAccept: NumberLong("1"), hidden: false, owner: NumberLong("1"),
    config: "type: default\ntime: 1500ms\nmemory: 512m\n",
    content: "# 最短路径\n\n在带权图中求单源最短路。\n",
    tag: ["图论", "最短路"], data: [], additional_file: [],
  },
  {
    _id: "qa", domainId: "system", docType: NumberInt("20"), docId: "qa", title: "问答",
    content: "visual-test", pic: "qa", owner: NumberLong("1"),
  },
  {
    _id: "share", domainId: "system", docType: NumberInt("20"), docId: "share", title: "分享",
    content: "visual-test", pic: "share", owner: NumberLong("1"),
  },
  {
    _id: contestID, domainId: "system", docType: NumberInt("30"), docId: contestID,
    title: "视觉回归比赛", rule: "acm", owner: NumberLong("1"), beginAt: contestStart, endAt: contestEnd,
    duration: NumberInt("180"), pids: [NumberLong("1001"), NumberLong("1002")], attend: NumberLong("1"),
    rated: false, content: "## 视觉回归比赛\n\n固定比赛数据。",
  },
  {
    _id: homeworkID, domainId: "system", docType: NumberInt("30"), docId: homeworkID,
    title: "视觉回归作业", rule: "homework", owner: NumberLong("1"), beginAt: homeworkStart, endAt: homeworkEnd,
    duration: NumberInt("10080"), pids: [NumberLong("1001"), NumberLong("1002")], attend: NumberLong("1"),
    rated: false, content: "## 视觉回归作业\n\n固定作业数据。",
  },
  {
    _id: trainingID, domainId: "system", docType: NumberInt("40"), docId: trainingID,
    title: "视觉回归训练", owner: NumberLong("1"), attend: NumberLong("1"), pin: false,
    content: "# 视觉回归训练\n\n固定训练数据。", description: "基础算法训练",
    dag: [
      { _id: NumberInt("1"), title: "基础章节", content: "前缀和与二分。", requireNids: [], pids: [NumberLong("1001"), NumberLong("1002")] },
      { _id: NumberInt("2"), title: "进阶章节", content: "图论。", requireNids: [NumberInt("1")], pids: [NumberLong("1003")] },
    ], files: [],
  },
  {
    _id: discussionID, domainId: "system", docType: NumberInt("21"), docId: discussionID,
    title: "视觉回归讨论：如何准备校赛", owner: NumberLong("2"), content: "# 如何准备校赛\n\n先从基础题开始，坚持复盘。",
    parentType: NumberInt("20"), parentId: "qa", nReply: NumberLong("1"), views: NumberLong("12"),
    updateAt: earlier, hidden: false, pin: false, highlight: false, lock: false, edited: false,
  },
  {
    _id: replyID, domainId: "system", docType: NumberInt("22"), docId: replyID,
    parentType: NumberInt("21"), parentId: discussionID, owner: NumberLong("1"),
    content: "先完成基础训练，再模拟比赛。", updateAt: at, edited: false, reply: [],
  },
]);

db.getCollection("document.status").insertMany([
  { domainId: "system", docType: NumberInt("30"), docId: contestID, uid: NumberLong("1"), attend: NumberInt("1"), unrank: false },
  { domainId: "system", docType: NumberInt("30"), docId: homeworkID, uid: NumberLong("1"), attend: NumberInt("1"), unrank: false },
  { domainId: "system", docType: NumberInt("40"), docId: trainingID, uid: NumberLong("1"), enroll: NumberInt("1") },
  { domainId: "system", docType: NumberInt("21"), docId: discussionID, uid: NumberLong("1"), star: false, react: {} },
]);

db.record.insertMany([
  {
    _id: ObjectId("6ae994800000000000005001"), domainId: "system", uid: NumberLong("2"), pid: NumberLong("1001"),
    lang: "cpp", code: "#include <iostream>\nint main(){return 0;}\n", status: NumberInt("1"), score: NumberInt("100"),
    time: 12.5, memory: NumberInt("1024"), judgeTexts: [], compilerTexts: [], testCases: [],
    judger: null, judgeAt: earlier, rejudged: false, type: "judge", source: "system/1001", createdAt: earlier,
  },
  {
    _id: ObjectId("6ae9a2900000000000005002"), domainId: "system", uid: NumberLong("1"), pid: NumberLong("1002"),
    lang: "cpp", code: "#include <iostream>\nint main(){return 1;}\n", status: NumberInt("2"), score: NumberInt("40"),
    time: 20.0, memory: NumberInt("2048"), judgeTexts: [], compilerTexts: [], testCases: [],
    judger: null, judgeAt: at, rejudged: false, type: "judge", source: "system/1002", createdAt: at,
  },
]);

printjson({
  ok: true,
  database: db.getName(),
  users: db.user.countDocuments({}),
  problems: db.document.countDocuments({docType: 10}),
  contests: db.document.countDocuments({docType: 30}),
  trainings: db.document.countDocuments({docType: 40}),
  discussions: db.document.countDocuments({docType: 21}),
  records: db.record.countDocuments({}),
});
MONGO
)
docker compose -f "$COMPOSE_FILE" exec -T mongo mongosh --quiet \
  -u "$MONGO_USERNAME" -p "$MONGO_PASSWORD" --authenticationDatabase admin "$MONGO_DATABASE" \
  --eval "$mongo_script"
