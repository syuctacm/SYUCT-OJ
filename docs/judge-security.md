# 判题沙箱安全评审

## 结论

本地开发镜像原先固定使用 `go-judge v1.9.4`。该版本会触发 `hydrojudge 4.0.6` 的告警：

```text
Your sandbox version is vulnerable to symlink escape issue, please upgrade!
```

上游在 `v1.11.0` 中修复了符号链接逃逸问题：

- `96ee886`：打开容器内文件时禁止解析符号链接
- `74482b8`：修复 symlink escape
- `c95a65a`：撤回所有受影响版本

因此最低安全版本为 `v1.11.0`，本地镜像已升级并固定到 `v1.13.3`，同时校验下载文件的 SHA-256：

```text
3f5f791eb2b7e4bdf9932f65d8e4c50a52905dc496444e96a1abe9453c70f48a
```

本次升级只替换 `go-judge` 可执行文件，不修改 `hydrojudge` 源码，也不修改 Go 后端的判题协议。

## 本地验证结果

升级后运行：

```bash
docker exec syuct-oj-dev-judge /usr/bin/sandbox --version
docker exec syuct-oj-dev-judge wget -qO- http://127.0.0.1:5050/version
docker exec syuct-oj-dev-judge wget -qO- http://127.0.0.1:5050/config
```

结果：

- 版本：`v1.13.3`
- `fixSymlinkEscape=true`
- `runnerConfig.seccomp=true`
- memory cgroup controller 可用
- `hydrojudge 4.0.6` 不再输出 symlink escape 告警
- C++ 提交闭环：`waiting -> compiling -> judging -> accepted`
- Python 提交闭环：`waiting -> compiling -> judging -> accepted`
- 判题过程中重启 judge 容器后任务重新领取，最终 Accepted

## 复验记录（2026-10-21）

在本地 Docker 重新创建判题容器（`deploy/compose.dev.yaml --profile judge`）后复验：

- `sandbox --version` → `v1.13.3`
- `GET /version` → `buildVersion: v1.13.3`、`fixSymlinkEscape: true`
- `hydrojudge 4.0.6` 启动日志不再输出 symlink escape 告警
- C++ 提交闭环：`waiting -> compiling -> judging -> accepted`（`scripts/real-judge-smoke.sh`）
- Python 3 提交闭环：`waiting -> compiling -> judging -> accepted`（`scripts/real-judge-python-smoke.sh`）

## 升级风险

`v1.13.x` 相比 `v1.9.4` 启用了默认 seccomp 策略，并调整了部分文件复制/缺失文件行为。当前本地只验证了 C++ 和 Python；Java、Pascal、PHP、Rust、GHC、Go、Ruby、Mono 等镜像内语言仍需在发布前回归。

必须在测试环境检查：

- 所有启用语言的编译和运行
- 文件读写、工作目录、`/dev/*` 和测试数据复制
- 交互题、Special Judge、Hack 和重判
- 判题机断线恢复和任务 lease
- 容器是否仍只需要现有 mount 配置

## 生产检查清单

没有生产环境访问权限，因此本仓库不能确认线上判题机版本。上线前必须由运维在只读环境中确认：

1. `sandbox --version` 不低于 `v1.11.0`，建议使用 `v1.13.3`。
2. `/version` 返回的 `fixSymlinkEscape` 为 `true`。
3. `/config` 中 `runnerConfig.seccomp` 状态明确。
4. 用 C++、Python 和全部生产启用语言各跑一次测试提交。
5. 观察 go-judge 启动日志、OOM、cgroup 和文件系统异常。
6. 保留旧镜像和回滚清单，切换前完成数据库与判题配置备份。

未完成上述生产确认前，不将当前开发镜像直接暴露到公网。
