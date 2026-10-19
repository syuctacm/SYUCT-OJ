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
| POST | `/judge/upload` | 接收判题机生成/上传的文件 |
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
- 当前 Session 存储是内存实现，仅用于本地开发；正式认证阶段会替换为持久化实现。

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

## 当前验证

- 登录 Cookie
- 文件检查与提交代码下载
- 题目测试文件链接与下载
- WebSocket Bearer 认证
- language 消息
- task 下发契约

## 尚未完成

- 真实 `hydrojudge` 进程联调
- 任务队列从提交数据库到判题机的完整链路
- 判题结果持久化和状态广播
- 测试数据缓存、重判、Hack 等高级流程
- 持久化 Session

真实联调只在本地 Docker 和测试环境中进行，不连接生产 OJ。
