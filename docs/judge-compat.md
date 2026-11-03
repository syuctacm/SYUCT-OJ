# hydrojudge 兼容层

## 目标

在不修改 `hydrojudge` 和 `go-judge` 的前提下，让现有判题机能够连接新的 Go 后端。

## 已实现的开发接口

| 方法 | 路径 | 用途 |
|---|---|---|
| POST | `/login` | 判题机账号登录，设置 `sid` Cookie |
| GET | `/judge/files` | 检查登录状态 |
| POST | `/judge/files` | 获取提交代码下载地址 |
| POST | `/d/:domainId/judge/files` | 获取题目测试数据下载地址 |
| POST | `/judge/upload` | 接收判题机生成/上传的文件，兼容无 filename 的 multipart part |
| GET | `/judge/conn` | 判题机 WebSocket 连接 |
| GET | `/judge/files/submission/:id/code` | 下载提交代码 |
| GET | `/judge/files/problem/:domainId/:pid/:filename` | 下载题目测试文件 |

内部站点标识固定为：

```text
domainId = "system"
```

## 认证

- 浏览器/判题机登录使用 `sid` HttpOnly Cookie。
- WebSocket 同时支持 `Authorization: Bearer <session>`。
- 当前 Session 使用 MongoDB 持久化，API 重启后登录状态仍有效。

## WebSocket 消息

服务端发送：

```json
{"language": {"cpp": {"display": "C++17", "compile": "...", "execute": "...", "code_file": "main.cpp"}}}
{"task": {"rid": "...", "lang": "cpp", "code": "...", "domainId": "system"}}
```

判题机发送：

```json
{"key": "start"}
{"key": "next", "rid": "...", "status": 1}
{"key": "end", "rid": "...", "status": 0}
{"key": "status", "info": {}}
```

## 真实判题机联调

本地容器固定使用：

- `@hydrooj/hydrojudge@4.0.6`
- `go-judge v1.13.3`
- 上游 `install/docker/judge` 的 mount 配置

启动：

```bash
docker compose -f deploy/compose.dev.yaml --profile judge up -d --build judge
./scripts/seed-local-judge.sh
./scripts/real-judge-smoke.sh
```

判题机通过 `host.docker.internal:18080` 连接宿主机上的 Go API。容器启用 `privileged` 和 host cgroup namespace，否则 `go-judge` 无法启用 memory cgroup，内存统计和限制会不准确。该配置只用于本地开发。

## 任务 lease 与断线恢复

- TaskPump 使用 MongoDB `leaseUntil` 原子领取任务，不再下发后立即删除
- 判题机连接存活期间，Hub 记录正在执行的 rid，防止 lease 到期造成并发重复下发
- `next` 会续租，`end` 持久化成功后才删除 task
- 判题机断线后，lease 到期即可被新连接重新领取
- 默认 lease 为 `2m`，本地恢复测试可通过 `JUDGE_TASK_LEASE=3s` 缩短

## 已验证

- 登录 Cookie、文件检查、提交代码和题目测试数据下载
- WebSocket Bearer 认证和纯文本 `ping/pong` 心跳
- 真实 `hydrojudge` 建立 `/judge/conn`
- C++ 编译、运行并通过
- Python 编译、运行并通过
- `type=generate` 自动生成测试数据：生成器/标准程序编译运行、逐文件上传、题目 testdata 元数据落库和最终 Accepted
- `type=pretest` Scratchpad 预测试：自定义输入编译运行、状态/时间/内存回写，且不影响正式题目统计
- 状态链路：`waiting(0) → compiling(21) → judging(20) → accepted(1)`
- 编译错误写入 `compilerTexts`
- WA 的测试点结果写入 `testCases`
- 系统级判题错误写入 `judgeTexts`
- 判题过程中停止 Docker 容器，重启后任务重新领取并 Accepted
- 短 lease 下的 8 秒长任务只执行一次，不会因 lease 到期重复下发
- testlib 特殊 checker 的 AC / WA 结果、checker 输出的错误信息
- interactive 题的 AC / WA、interactor 消息和测试点耗时/内存
- 特殊 checker 单条记录重判
- 普通题目 Hack：successful / unsuccessful、输入文件下载、追加测试点和 AC 记录 `hackRejudge`

服务端广播语言时保留 hydrojudge 的文件扩展名别名：`cc` 和 `py` 会作为隐藏编译语言下发，不出现在提交语言选择列表中，因此 `chk.cc`、`interactor.cpp`、`interactor.py` 等文件可以按扩展名自动选择编译器。

断线恢复验证：

```bash
JUDGE_TASK_LEASE=3s go run ./cmd/api
./scripts/real-judge-recovery-smoke.sh
```

## 特殊 checker / 交互题回归

使用隔离数据库和独立 API 进程：

```bash
MONGO_DATABASE=syuct_oj_judge_smoke JUDGE_FILE_ROOT=./data/judge-files \
  SERVER_PORT=18080 go run ./cmd/api

docker compose -f deploy/compose.dev.yaml --profile judge up -d judge

ALLOW_DESTRUCTIVE_LOCAL_TEST=1 \
MONGO_DATABASE=syuct_oj_judge_smoke \
JUDGE_FILE_ROOT=./data/judge-files \
./scripts/real-judge-special-smoke.sh
```

脚本会重建两个本地测试题，验证：

- `checker_type: testlib` + `checker: chk.cc` 的 AC / WA
- `type: interactive` + `interactor.cpp` 的 AC / WA
- 特殊 checker 记录重判
- 两个题目的 `nSubmit`、`nAccept` 和记录数

脚本只允许 localhost API，并且只接受名称含 `test`、`smoke` 或 `e2e` 的数据库。所有写入都限制在本地 Docker MongoDB 和 `JUDGE_FILE_ROOT`。

## 普通题目 Hack 回归

```bash
ALLOW_DESTRUCTIVE_LOCAL_TEST=1 \
MONGO_DATABASE=syuct_oj_judge_hack_smoke \
JUDGE_FILE_ROOT=./data/judge-files \
./scripts/real-judge-hack-smoke.sh
```

脚本使用本地 Docker hydrojudge 验证输入校验、Hack Successful / Unsuccessful、Hack 输入下载、测试点追加、`hackRejudge`、nSubmit/nAccept 和错误权限。脚本同样只允许 localhost API 和专用测试数据库。

## 尚未完成

- Codeforces 规则的比赛内 Hack
- 生产判题机版本与服务端协议的长期兼容矩阵

真实联调只在本地 Docker 和测试环境中进行，不连接生产 OJ。

`go-judge` 最低安全版本为 `v1.11.0`。本地开发镜像固定使用 `v1.13.3`，并已确认 `fixSymlinkEscape=true`；升级过程不改 `hydrojudge` 源码。详细评审和生产检查清单见 [判题沙箱安全评审](./judge-security.md)。
