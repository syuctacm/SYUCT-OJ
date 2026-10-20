#!/bin/sh
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
MONGO_CONTAINER=${MONGO_CONTAINER:-syuct-oj-dev-mongo}
MONGO_USERNAME=${MONGO_ROOT_USERNAME:-syuct}
MONGO_PASSWORD=${MONGO_ROOT_PASSWORD:-syuct_dev_password}
MONGO_DATABASE=${MONGO_DATABASE:-syuct_oj}
JUDGE_FILE_ROOT=${JUDGE_FILE_ROOT:-"$ROOT/data/judge-files"}

data_dir="$JUDGE_FILE_ROOT/problem/system/9001/testdata"
mkdir -p "$data_dir"
printf '7 8\n' > "$data_dir/1.in"
printf '15\n' > "$data_dir/1.out"

docker exec -i "$MONGO_CONTAINER" mongosh --quiet \
  -u "$MONGO_USERNAME" -p "$MONGO_PASSWORD" --authenticationDatabase admin "$MONGO_DATABASE" <<'MONGO'
const now = new Date();
db.document.updateOne(
  { domainId: "system", docType: 10, pid: "PJUDGE" },
  {
    $set: {
      title: "本地判题联调",
      content: "读取两个整数并输出它们的和。",
      hidden: false,
      owner: NumberLong("1"),
      config: "type: default\ntime: 30000ms\nmemory: 256m\nsubtasks:\n  - score: 100\n    type: min\n    cases:\n      - input: 1.in\n        output: 1.out\n",
      data: [
        { _id: "problem/system/9001/1.in", name: "1.in", size: NumberLong("4"), etag: "local-v1", lastModified: now },
        { _id: "problem/system/9001/1.out", name: "1.out", size: NumberLong("3"), etag: "local-v1", lastModified: now }
      ]
    },
    $setOnInsert: {
      _id: ObjectId(),
      domainId: "system",
      docType: 10,
      docId: NumberLong("9001"),
      pid: "PJUDGE",
      tag: ["本地测试"],
      nSubmit: NumberLong("0"),
      nAccept: NumberLong("0")
    }
  },
  { upsert: true }
);
printjson(db.document.findOne({ domainId: "system", docType: 10, pid: "PJUDGE" }, { _id: 1, docId: 1, pid: 1, title: 1, data: 1 }));
MONGO
