# 提交与判题闭环

## 接口

| 方法 | 路径 | 用途 |
|---|---|---|
| POST | `/api/v1/submissions` | 创建提交并写入 judge task |
| GET | `/api/v1/submissions` | 提交记录列表、筛选和分页 |
| GET | `/api/v1/submissions/:rid` | 查询提交状态和结果 |

创建提交和查看记录列表需要用户 Session。列表默认显示单站点全部记录，可用以下参数筛选：

- `uidOrName`：UID 或用户名
- `pid`：题号或 docId
- `tid`：比赛 ObjectId
- `lang`：语言
- `status`：判题状态
- `page` / `limit`：分页

记录详情仅向提交者返回代码，其他用户只能查看判题状态和统计信息。

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
- 可选 `contestId` 会把提交关联到比赛并写入判题 task；提交时必须处于比赛开始/结束时间窗口内

## 前端

记录页面结构对应 Hydro `ui-default`：

- `record_main.html`：筛选区和记录表格
- `record_main_tr.html`：状态、题目、提交者、时间、内存、语言和提交时间
- `record_detail.html`：状态区、编译器/评测输出、测试点和代码
- `record_detail_status.html` / `record_detail_summary.html`：状态和统计信息

## 验证

本地 Docker MongoDB + WebSocket mock judge 集成测试覆盖：

- 用户登录
- 创建提交
- task 写入和 WebSocket 下发
- `next/end` 结果回写
- 提交状态变为 Accepted

真实 `hydrojudge 4.0.6` + `go-judge` 容器验证覆盖：

- C++ 和 Python 编译运行
- `waiting(0) → compiling(21) → judging(20) → accepted(1)`
- 编译错误、测试点结果和运行时间回写
- 小数毫秒时间 `1.566ms` 可正常持久化和读取

## 任务可靠性

- TaskPump 通过 `leaseUntil` 原子领取任务
- Hub 记录活跃 rid，避免同一连接正在评测时重复下发
- `next` 续租，`end` 持久化成功后才删除 task
- 判题机断线后，lease 到期自动重投
- 默认 lease 为 `2m`，可通过 `JUDGE_TASK_LEASE` 调整

`time` 使用浮点毫秒，展示时按原模板四舍五入。
