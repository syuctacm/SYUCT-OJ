# 数据迁移方案

## 1. 目标

把现有三个域压平成单站点 `system`，同时保留历史题目、提交、比赛和用户统计。

当前生产域及规模（迁移前基线）：

| 域 | 用户关系 | 提交 |
|---|---:|---:|
| `system` | 531 | 41,635 |
| `contest2025` | 62 | 1,303 |
| `select2025` | 113 | 1,291 |

## 2. 迁移原则

- 只在本地 Docker 或测试环境演练。
- 使用本地恢复的脱敏副本。
- 不直接修改生产数据。
- 保留原始 ID 和来源域标记。
- 先 dry-run，再执行。
- 迁移工具必须幂等，可重复运行。
- 每一步都要有数量校验和可回滚方案。

## 3. 迁移步骤

1. 在本地 Docker 启动 MongoDB。
2. 恢复测试数据。
3. 生成迁移前报告：
   - 每个域的用户关系数
   - 文档/题目/比赛数
   - 提交数
   - 榜单统计
4. 迁移 `contest2025` 和 `select2025` 的文档、比赛、训练和作业。
5. 迁移用户关系，合并重复用户。
6. 更新记录的 `domainId` 为 `system`。
7. 写 `legacyDomain` 字段保留来源。
8. 重算题目统计、用户统计和比赛榜单。
9. 生成迁移后报告并对比。
10. 输出失败项、跳过项和冲突项。

## 4. 题目 ID 冲突

Hydro 的题目 `docId` 是按域自增的（`ProblemModel.add` 取该域最大 docId + 1），三个域压平后可能出现重复。迁移工具的处理：

| 冲突类型 | 行为 |
|---|---|
| 题目 `docId` 重复 | **中止迁移**，不写任何数据；报告 `conflicts[kind=docId]`，CLI 退出码非 0 |
| 题目 `pid` 重复 | 自动把来源域的冲突题目改名为 `P<docId>`，写 `legacyPids`；报告 `conflicts[kind=pid]` 与 `problems_renamed` |

`docId` 被 `record.pid`、`contest.pids`、`training.dag[].pids`、`document.status.docId`、`storage.path` 和物理文件目录引用；重编号必须同时重写这些引用，风险高，所以工具选择拒绝执行而不是猜测。

生产切换前必须先用备份副本跑 dry-run 并检查 `conflicts`：

```bash
MONGO_URI=... MONGO_DATABASE=syuct_oj_restore \
  go run ./cmd/migrate --dry-run --report /tmp/migrate-dry.json
```

如果出现 `docId` 冲突，需要先实现并执行题目重编号（docId + 所有引用 + 存储路径），再跑正式迁移。

## 5. URL 兼容

保留旧 URL 到新 URL 的重定向映射，例如：

```text
/d/system/p/:pid      -> /p/:pid
/d/contest2025/...    -> /...
/d/select2025/...     -> /...
```

重定向规则应集中配置，并有测试覆盖。

## 6. 判题兼容

`hydrojudge` 仍要求任务中包含 `domainId`。迁移后统一使用：

```text
domainId = "system"
```

不得修改判题机来适配新的单站模型。

## 7. 回滚

迁移前保留完整 MongoDB dump。回滚步骤：

1. 停止新后端。
2. 恢复迁移前 MongoDB dump。
3. 恢复旧版本服务。
4. 验证用户、题目、提交、比赛数量。

## 8. 验收

- dry-run 完成且报告可读
- 迁移前后数量差异可解释
- 用户关系无重复
- 比赛榜单可访问
- 旧 URL 可重定向
- 本地判题闭环不受影响

## 9. 迁移工具

```bash
# 默认 dry-run，不修改数据
go run ./cmd/migrate   --sources=contest2025,select2025   --dry-run   --report=/tmp/domain-migration-report.json

# 明确执行迁移
go run ./cmd/migrate   --sources=contest2025,select2025   --execute
```

当前工具会：

- 把 domain 范围集合的 `domainId` 更新为 `system`
- 给迁移后的文档添加 `legacyDomains`
- 合并 `domain.user` 的数值统计
- 把旧 domain 文档归档到 `domain.legacy`
- 重写 `storage.path` 的逻辑前缀
  - `problem/<source>/...` -> `problem/system/...`
  - `contest/<source>/...` -> `contest/system/...`
  - `training/<source>/...` -> `training/system/...`
- 迁移后校验所有源域集合、用户关系和存储路径残留数量
- 在报告中输出每个集合的 `matched`、`modified`、`remaining` 和 `validation.residuals`

`storage` 集合里的 `path` 是逻辑路径，物理对象仍由原 `_id`/`link` 引用；本工具不会移动对象存储中的物理文件。

报告结构示例：

```json
{
  "dry_run": false,
  "validation": {
    "ok": true
  },
  "users_merged": 175,
  "users_remaining": 0,
  "collections": [
    {
      "collection": "storage.path",
      "domain": "contest2025",
      "matched": 24,
      "modified": 24,
      "remaining": 0
    }
  ]
}
```

执行后如果 `validation.ok=false`，CLI 会以非 0 状态退出。重复执行是幂等的，第二次执行不会再次修改已经迁移的数据。
