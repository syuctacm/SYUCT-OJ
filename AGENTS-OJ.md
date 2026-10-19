# AGENTS-OJ.md — SYUCT OJ 重构规范

本文件约束 `syuct-oj` 的重构工作，补充而不是替代环境级 AGENTS 规范。

## 1. 安全边界

- 开发期间禁止连接生产 OJ、生产 MongoDB 或生产服务器。
- 禁止把生产凭据、用户数据或备份数据提交到 Git。
- 本地依赖通过 Docker 启动。
- 联调使用脱敏数据、种子数据或本地恢复的测试副本。
- 生产切换必须单独评审，不在日常开发任务中执行。

## 2. 产品范围

### 保留

- 用户、登录、会话、权限
- 题目、测试数据、提交、记录
- 比赛、榜单、训练、作业
- 讨论、题解、文件
- 系统管理
- 自定义签到、倒计时、活跃日历、标签榜单

### 移除

- 多域 / 多租户
- VJudge 远程评测
- HydroAC 远程评测 / LSP 客户端
- Cordis / addon 插件加载机制

### 不修改

- `hydrojudge`
- `go-judge`
- sandbox

## 3. 技术边界

- 后端：Go
- 前端：React + Vite + React Router
- 数据：第一阶段 MongoDB；PostgreSQL 迁移必须单独立项
- 认证：HttpOnly Session Cookie 为主
- 前端渲染：SPA；SSR 不在一期范围
- 内部站点标识：固定为 `system`，仅用于数据和判题兼容

## 4. 标准工作流

```text
Issue
→ .temp/ worktree
→ 开发与测试
→ Conventional Commit
→ push
→ Pull Request
→ 合并
→ 清理 worktree
```

分支命名：

```text
feat/issue-<编号>-<名称>
fix/issue-<编号>-<名称>
docs/issue-<编号>-<名称>
refactor/issue-<编号>-<名称>
chore/issue-<编号>-<名称>
```

提交格式：

```text
<type>(<scope>): <subject>
```

## 5. 质量门禁

后端：

```bash
gofmt -w .
go vet ./...
go test ./...
go build ./...
```

前端：

```bash
pnpm --dir web lint
pnpm --dir web build
```

判题相关改动额外要求：

- `hydrojudge` 协议契约测试
- 一次本地完整提交闭环
- 不修改判题机代码

前端页面迁移额外要求：

- 关键页面截图对比
- 关键操作 E2E 测试
- 已知差异写入 `docs/parity-checklist.md`

数据迁移额外要求：

- `--dry-run`
- 迁移前后计数报告
- 回滚说明
- 仅在本地或测试环境演练

## 6. Definition of Done

- 功能实现与 Issue 验收标准一致
- 构建和测试通过
- 不触碰生产环境
- 文档和已知差异同步更新
- worktree 和临时分支已清理
