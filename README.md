# SYUCT OJ Platform

SYUCT ACM 的新一代 OJ 平台，目前处于 **开发阶段**。

## 状态与安全边界

- **禁止连接生产环境**：开发期间不得访问生产 OJ、生产 MongoDB 或生产服务器。
- **只使用本地 Docker**：数据库、缓存和其他依赖优先通过本地 Docker Compose 启动。
- **只使用开发数据**：使用脱敏数据、种子数据或本地测试数据，不复制生产用户数据。
- **不修改生产 OJ**：现有的 `https://oj.syuctacm.cn` 保持不动。
- **判题机不改**：现有 `hydrojudge` 作为兼容边界，Go 后端需要实现其所需协议。

## 项目目标

- 单站点：移除多域/多租户，不暴露域创建、加入、切换和域管理。
- 保留核心功能：用户、题目、提交、评测、记录、比赛、榜单、训练、作业、讨论、文件等。
- 使用 Go 重写 Web/API 层。
- 使用 React 重写前端，视觉和交互向现有站点对齐。
- 保持判题机、判题协议和现有数据迁移路径的兼容性。

## 推荐目录结构

```text
syuct-oj/
├── cmd/
│   └── api/                 # Go 服务入口
├── internal/
│   ├── auth/                # 用户、登录、权限
│   ├── problem/             # 题目、测试数据
│   ├── submission/          # 提交与记录
│   ├── contest/             # 比赛与榜单
│   ├── judge/               # hydrojudge 兼容层
│   └── storage/             # 文件存储
├── web/                     # React + Vite 前端
├── migrations/              # 数据迁移脚本
├── deploy/                  # 本地 Docker / 生产部署配置
├── docs/                    # 架构、迁移、验收和决策记录
├── go.mod
└── README.md
```

当前阶段只建立开发目录和约束，尚未初始化具体技术栈或启动容器。

## 参考资产

- `../SYUCT-OJ/`：Hydro 官方代码基线，只读参考。
- `../SYUCTACM/`：社团官网项目，与本 OJ 平台分开维护。
- 生产全量备份：`/home/syuct/backup/oj-full-20261019T112022Z`（仅用于灾备，不用于日常开发）。

## 开发工作流

遵循 Issue 驱动的隔离开发：

1. 创建/确认 GitHub Issue。
2. 在 `.temp/` 下创建 Git Worktree。
3. 在独立分支开发和测试。
4. 通过后端和前端构建、测试。
5. 提交 Conventional Commits。
6. 创建 Pull Request 并合并。
7. 删除 Worktree 和临时分支。

后续会为 OJ 单独补充 `AGENTS.md`，明确判题兼容、数据迁移和视觉回归要求。

## 开发文档

- [AGENTS-OJ.md](./AGENTS-OJ.md)：重构开发规范与安全边界
- [docs/architecture.md](./docs/architecture.md)：目标架构
- [docs/migrations.md](./docs/migrations.md)：多域数据迁移方案
- [docs/parity-checklist.md](./docs/parity-checklist.md)：前端视觉与交互对照清单

> 当前为开发阶段，不连接生产环境；判题机保持不动。

## 本地开发

### 1. 启动 MongoDB

```bash
docker compose -f deploy/compose.dev.yaml up -d
```

默认只绑定 `127.0.0.1:27017`，不会暴露到公网。

### 2. 启动 Go API

```bash
cp .env.example .env
go run ./cmd/api
```

默认地址：`http://127.0.0.1:8080`。如果端口被占用，可以使用 `SERVER_PORT=18080 go run ./cmd/api`。

### 3. 启动 React

```bash
cd web
pnpm install
pnpm dev
```

默认地址：`http://127.0.0.1:5173`，`/api` 会代理到本地 Go 服务。

### 4. 验证

```bash
go test ./...
go build ./...
cd web && pnpm build
```

当前只包含开发脚手架和健康检查，不包含业务功能，也不连接生产环境。

## 判题机开发接口

本地开发使用以下环境变量（不要用于生产）：

```bash
JUDGE_USERNAME=judge
JUDGE_PASSWORD=judge_dev_password
JUDGE_FILE_ROOT=./data/judge-files
```

目前实现了 `hydrojudge` 兼容的登录、判题文件下载、测试数据下载、上传和 WebSocket 握手接口。详细契约见 [docs/judge-compat.md](./docs/judge-compat.md)。

> Session 当前是内存实现，仅用于本地开发；真实 `hydrojudge` 进程联调尚未完成。

## 域数据迁移工具

本地 dry-run：

```bash
go run ./cmd/migrate --sources=contest2025,select2025 --dry-run --report=/tmp/domain-migration.json
```

默认不会修改数据；只有显式传入 `--execute` 才会执行。存储文件路径迁移尚未包含。

## 用户认证接口

- `POST /api/v1/auth/login`
- `GET /api/v1/auth/me`
- `POST /api/v1/auth/logout`

密码校验兼容 Hydro 的 PBKDF2 格式，详见 [docs/auth.md](./docs/auth.md)。Session 当前为内存实现，仅用于本地开发。

## 题目接口

- `GET /api/v1/problems`
- `GET /api/v1/problems/:pid`

接口直接读取本地 MongoDB 的 `document` 集合，详见 [docs/problems.md](./docs/problems.md)。

## 提交接口

- `POST /api/v1/submissions`
- `GET /api/v1/submissions`
- `GET /api/v1/submissions/:rid`

提交会写入 `record` 和 `task`，TaskPump 再将任务下发给兼容的判题机。详见 [docs/submissions.md](./docs/submissions.md)。

## 前端核心页面

已实现 `/login`、`/p`、`/p/:pid`、`/p/:pid/submit`、`/record/:rid`。本地联调时可以把 Vite 的 API 代理指向 `SERVER_PORT` 对应的后端端口。

## 比赛接口

- `GET /api/v1/contests`
- `GET /api/v1/contests/:tid`
- `GET /api/v1/contests/:tid/problems`
- `GET /api/v1/contests/:tid/scoreboard`

比赛列表和详情读取 Hydro `document` 集合，详见 [docs/contests.md](./docs/contests.md)。

## 训练接口

- `GET /api/v1/trainings`
- `GET /api/v1/trainings/:tid`

训练列表和详情读取 Hydro `document` 集合，详见 [docs/training.md](./docs/training.md)。

## 作业接口

- `GET /api/v1/homeworks`
- `GET /api/v1/homeworks/:tid`

作业使用 `docType=30`、`rule=homework` 的文档，详见 [docs/homework.md](./docs/homework.md)。

## 讨论接口

- `GET /api/v1/discussions`
- `GET /api/v1/discussions/:did`

讨论读取 Hydro `document` 的 `docType=21` 文档，详见 [docs/discussion.md](./docs/discussion.md)。

## 用户主页

- `GET /api/v1/users/:uid`
- `/user/:uid`

公开资料不包含邮箱和密码字段，详见 [docs/user-profile.md](./docs/user-profile.md)。

## 后台概览

- `GET /api/v1/admin/stats`
- `GET /api/v1/admin/users`
- `/admin`

需要 `PRIV_EDIT_SYSTEM` 权限，详见 [docs/admin.md](./docs/admin.md)。
