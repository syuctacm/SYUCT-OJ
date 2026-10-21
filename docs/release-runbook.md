# 发布与回滚 Runbook（单站点切换）

> 状态：准备稿。**在对生产环境执行任何步骤之前，必须得到用户明确确认。**
> 所有演练只在本地 Docker / 预发布副本上进行；生产操作由用户执行，或用户明确授权后执行。

## 0. 前置条件

- 生产全量备份已完成并校验（MongoDB dump + 文件存储）
- 新版本镜像已构建，且本地回归通过（见 [前端视觉与交互对照清单](./parity-checklist.md)、[首页](./homepage.md) 等文档）
- 迁移演练通过：`./scripts/migration-drill.sh`
- 维护窗口、回滚负责人、判题机切换方式已确认
- `hydrojudge` 的 `server_url` 已准备指向新 API（生产建议 `http://api:8080/`）
- 判题机版本已按 [判题沙箱安全评审](./judge-security.md) 确认（`sandbox --version` 不低于 `v1.11.0`，建议 `v1.13.3`）

## 1. 生产备份

```bash
# 在数据库主机上执行（示例，按实际路径调整）
mongodump --uri "$MONGO_URI" --archive=/home/syuct/backup/pre-cutover-$(date +%Y%m%dT%H%M%SZ).archive --gzip
```

校验：

- dump 文件存在且大小合理
- 在本地恢复一份副本，核对 `document` / `record` / `user` / `domain.user` 数量
- 记录迁移前基线（用于切换后对比）

## 2. 预发布演练（本地 Docker）

1. 把生产备份恢复到本地临时数据库：

```bash
docker exec -i syuct-oj-dev-mongo mongorestore --archive --gzip \
  -u syuct -p syuct_dev_password --authenticationDatabase admin \
  --nsFrom='syuct_oj.*' --nsTo='syuct_oj_rehearsal.*' < pre-cutover.archive
```

2. 对副本跑 dry-run，检查报告：

```bash
MONGO_DATABASE=syuct_oj_rehearsal MONGO_URI=... \
  go run ./cmd/migrate --dry-run --report /tmp/migrate-dry-run.json
```

关注：每个集合/域的 `matched`、`users_merged`、`warnings`、残留项（dry-run 阶段应只有预期内的匹配数据）。

3. 对副本执行迁移并验证：

```bash
MONGO_DATABASE=syuct_oj_rehearsal MONGO_URI=... \
  go run ./cmd/migrate --execute --report /tmp/migrate-execute.json
```

验收：

- `validation.ok == true`，无 residual
- 用户合并数量符合基线
- 抽样页面正常（登录、题库、题目、记录、比赛、讨论、用户主页）
- 旧 URL 重定向正常
- 本地 `hydrojudge` 提交闭环正常

4. 幂等复跑：再次执行 `--execute`，确认 `modified == 0`、`users_merged == 0`、validation 仍为 OK。

## 3. 切换步骤（生产，需确认）

1. 公告维护窗口，停止旧服务写入。
2. 执行第 1 节的全量备份并校验。
3. 对生产库执行迁移：

```bash
MONGO_URI=... MONGO_DATABASE=syuct_oj \
  go run ./cmd/migrate --execute --report /tmp/prod-migrate.json
```

4. 检查报告：`validation.ok == true`、`users_merged` 符合预期、无 residual。
5. 启动新栈：

```bash
cp .env.prod.example .env.prod   # 填写真实值（不要提交）
docker compose -f deploy/compose.prod.yaml up -d --build
```

6. 前端流量切到 `web` 服务；`hydrojudge` 的 `server_url` 切到新 API。
7. 执行第 4 节验证清单。

## 4. 验证清单

- [ ] `GET /api/v1/health` 返回 healthy
- [ ] 首页、题库、题目详情
- [ ] 登录/登出、Session 持久化
- [ ] 提交 → 判题 → 记录闭环（hydrojudge）
- [ ] 判题机 `sandbox --version` 不低于 `v1.11.0`，`/version` 返回 `fixSymlinkEscape=true`
- [ ] 生产启用的每种语言各跑一次测试提交
- [ ] 比赛/作业/训练列表与详情、榜单
- [ ] 讨论列表/详情/回复
- [ ] 用户主页、活跃日历
- [ ] 后台概览/用户管理
- [ ] 旧 URL 重定向
- [ ] 数据抽查：用户数、题目数、提交数、比赛数与基线一致

## 5. 回滚

触发条件：迁移校验失败、判题闭环异常、关键页面不可用、数据丢失或数量异常。

1. 停止新栈：`docker compose -f deploy/compose.prod.yaml down`
2. 恢复迁移前备份（先 drop 目标库再 restore）。
3. 如文件存储被移动过，恢复文件存储副本（本次迁移只重写逻辑 `storage.path`，不移动物理文件）。
4. 启动旧 Hydro 服务。
5. 核对数量与核心闭环，恢复流量并公告。

## 6. 已知限制

- 迁移只重写逻辑存储路径（`storage.path`），不移动物理文件；存储后端必须保持相同布局。
- 迁移后统一使用 `domainId=system`，旧多域 URL 依赖前端重定向。
- 计划移除的功能（vjudge / hydroac 等）不会迁移。
- 迁移工具幂等，但回滚仍依赖备份。
