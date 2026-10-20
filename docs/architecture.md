# 目标架构

## 1. 总体结构

```text
syuct-oj/
├── cmd/api/                  # Go 服务入口
├── internal/
│   ├── config/
│   ├── database/
│   ├── auth/
│   ├── user/
│   ├── problem/
│   ├── submission/
│   ├── contest/
│   ├── training/
│   ├── homework/
│   ├── discussion/
│   ├── storage/
│   ├── admin/
│   ├── judge/                # hydrojudge 兼容层
│   └── site/                 # 单站点上下文
├── web/                      # React + Vite
├── migrations/
├── deploy/
├── docs/
├── go.mod
└── README.md
```

## 2. 后端

- Go 服务负责浏览器 API、业务逻辑、权限、存储和判题调度。
- 业务代码放 `internal/`；`cmd/` 只放入口和装配。
- 数据访问以 repository/DAO 隔离，便于未来数据库迁移。
- 第一阶段使用 MongoDB，保留现有数据兼容性。
- API 统一前缀 `/api/v1`。
- 错误响应使用明确 HTTP 状态码和稳定错误码。

## 3. 认证

- 浏览器使用 HttpOnly Session Cookie。
- 判题机继续使用兼容的登录接口和 Cookie。
- 不在第一阶段引入 JWT/localStorage 认证。
- 后续如需 API Token，再单独设计。

## 4. 单站点模型

- 对外不暴露 domain 概念。
- UI/API 不提供创建、加入、切换、管理域。
- 内部固定使用 `system` 作为兼容站点标识。
- 现有文档/记录中的 `domainId` 在迁移期固定为 `system`。
- 比赛、训练、作业仍按自身 ID 管理，不再依赖 domain 隔离。

## 5. 判题兼容层

现有 `hydrojudge` 不改动。Go 后端至少兼容：

- `POST /login`
- `GET /judge/files`
- `POST /judge/files`
- `POST /d/{domainId}/judge/files`
- `POST /judge/upload`
- `WebSocket /judge/conn`

WebSocket 消息：

```text
language / task / next / end / status / config / start / ping / pong
```

发给判题机的 `domainId` 固定为 `system`。

## 6. 前端

- React 19 + TypeScript + Vite + React Router。
- 第一阶段为 SPA。
- 复用旧站 CSS、字体、图标和视觉结构。
- 已迁移页面通过 `HydroTemplate` 直接渲染固定版本 `@hydrooj/ui-default` 的上游 Nunjucks 模板；React 负责数据、路由和交互挂载。
- 禁止复制上游页面 DOM 到手写 TSX；剩余页面按功能域逐步迁移。

## 7. 数据迁移

- 迁移工具作为 Go 命令行程序放 `cmd/migrate` 或独立包。
- 所有迁移必须支持 dry-run、幂等和报告输出。
- 迁移前只读取本地测试副本，不访问生产。
- 详细方案见 `docs/migrations.md`。

## 8. 明确不在范围内

- 多域 / 多租户
- VJudge
- HydroAC client / remote judge / LSP
- 插件加载器
- SSR / Next.js
- PostgreSQL 一期迁移
- Redis / Kafka / 微服务
