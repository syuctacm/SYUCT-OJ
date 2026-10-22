# 提交与判题闭环

## 接口

| 方法 | 路径 | 用途 |
|---|---|---|
| POST | `/api/v1/submissions` | 创建提交并写入 judge task |
| GET | `/api/v1/submissions` | 提交记录列表、筛选和分页 |
| GET | `/api/v1/submissions/:rid` | 查询提交状态和结果 |
| GET | `/api/v1/submissions/:rid/code` | 下载提交代码（所有者或管理员） |
| POST | `/api/v1/submissions/:rid/rejudge` | 单条记录重测（管理员） |
| POST | `/api/v1/submissions/:rid/cancel` | 取消单条记录成绩（管理员） |

创建提交和查看记录列表需要用户 Session。列表默认显示单站点全部记录，可用以下参数筛选：

- `uidOrName`：UID 或用户名
- `pid`：题号或 docId
- `tid`：比赛 ObjectId
- `lang`：语言
- `status`：判题状态
- `page` / `limit`：分页

记录详情仅向提交者和管理员返回代码，其他用户只能查看判题状态和统计信息；代码下载接口使用相同权限。

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

- `record_main.html`：由 `HydroTemplate` 直接渲染筛选区、记录表格和分页
- `record_main_tr.html`：状态、题目、提交者、时间、内存、语言和提交时间
- `record_detail.html`：由 `HydroTemplate` 直接渲染状态区、编译器/评测输出、测试点、代码和信息侧栏
- `record_detail_status.html` / `record_detail_summary.html`：状态和统计信息

记录列表的语言下拉和筛选表单由 React 交互岛接入，分页和重置链接保持原模板结构。记录详情在等待/评测中每秒轮询 API，状态和测试点更新后重新渲染原模板；管理员可使用原模板的 Rejudge/Cancel Score，所有者或管理员可通过 Download 下载代码。

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

## 重测（rejudge）

题目侧栏的“重测整题”对应 `POST /api/v1/problems/:pid/rejudge`（仅管理员）；记录详情中的 Rejudge 对应 `POST /api/v1/submissions/:rid/rejudge`，只重置当前记录并替换其 task。

`POST /api/v1/submissions/:rid/cancel` 把记录设为 `status=9`，清空分数/时间/内存并删除 task，同时维护题目 AC 计数。

- 把该题所有记录重置为 `status=waiting`、`score/time/memory=0`、清空判题输出，并标记 `rejudged=true`
- 为每条记录重建一条判题任务（复用提交时的任务构造逻辑）
- 重置或取消已 AC 记录前回退 `nAccept`；重新判题结果回来后再按 AC 迁移加回
- 隐藏题目也可以重测（管理员通过 `GetForEditor` 解析）

## 已知差异

- 记录列表暂未接入实时 WebSocket 状态推送
- 默认 lease 为 `2m`，可通过 `JUDGE_TASK_LEASE` 调整

`time` 使用浮点毫秒，展示时按原模板四舍五入。
