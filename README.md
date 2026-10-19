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
