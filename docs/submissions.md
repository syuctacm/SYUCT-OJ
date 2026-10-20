# 提交与判题闭环

## 接口

| 方法 | 路径 | 用途 |
|---|---|---|
| POST | `/api/v1/submissions` | 创建提交并写入 judge task |
| GET | `/api/v1/submissions` | 当前用户的提交记录列表 |
| GET | `/api/v1/submissions/:rid` | 查询提交状态和结果 |

创建提交和查询自己的记录列表需要用户 Session。

## 请求

```json
{
  "pid": "P1001",
  "lang": "cpp",
  "code": "int main() {}",
  "contestId": "比赛 ObjectId（可选）"
}
```

## 数据流

```text
用户提交
→ record(status=waiting)
→ task(type=judge)
→ TaskPump
→ WebSocket /judge/conn
→ hydrojudge 兼容客户端
→ next/end 结果
→ record 更新
```

## 当前实现

- 直接写入 Hydro 兼容的 `record` 和 `task` 集合
- 任务通过现有 `hydrojudge` WebSocket 协议下发
- 结果支持 `status`、`score`、`time`、`memory`、`message`、`compilerText`、`case/cases`
- `domainId` 固定为 `system`
- 可选 `contestId` 会把提交关联到比赛并写入判题 task

## 验证

本地 Docker MongoDB + WebSocket mock judge 集成测试已验证：

- 用户登录
- 创建提交
- task 写入
- judge 收到任务
- judge 回写 `end`
- 提交状态变为 Accepted

尚未使用真实 `hydrojudge` 进程执行编译/运行，真实进程联调仍是后续任务。

## 注意

TaskPump 当前在任务下发后从 MongoDB 删除任务；如果判题机在返回结果前断线，任务不会自动重试。生产化前需要增加领取确认、超时和重试机制。
