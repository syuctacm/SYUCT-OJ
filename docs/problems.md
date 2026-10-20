# 题目 API

## 接口

| 方法 | 路径 | 用途 |
|---|---|---|
| GET | `/api/v1/problems` | 题目列表、分页、搜索和筛选 |
| GET | `/api/v1/problems/:pid` | 题目详情，支持 `pid`、`docId` 或 ObjectId |

## 列表参数

- `page`：默认 `1`
- `limit`：默认 `20`，最大 `100`
- `q`：按标题或题号模糊搜索
- `tag`：按标签筛选
- `difficulty`：按难度筛选
- `sort`：`default` 或 `recent`

登录请求会为每题附带 `userStatus`，包含当前用户最近一次提交的 `status` 和 `rid`。

只返回 `domainId=system`、`docType=10` 且未隐藏的题目。

## 数据来源

直接读取 Hydro MongoDB 的 `document` 集合，不复制题目到新表。用户提交状态读取同库的 `record` 集合。

## 前端

题库页结构和类名对应 Hydro `ui-default`：

- `problem_main.html`：搜索栏、排序、题库表格和侧栏
- `partials/problem_list.html`：分页、题目标题、标签、状态、AC/Tried 和难度列
- `components/problem.html`：题目链接和标签 DOM

搜索、排序、分页、标签筛选和用户提交状态均通过 URL 与 API 参数保持。

题目详情和提交页对应：

- `problem_detail.html`：题目标题、标签信息、Markdown 内容
- `problem_submit.html`：语言选择、代码输入和提交按钮
- `partials/problem_sidebar.html`：提交入口和题目信息侧栏

详情 API 额外返回 `problemType`、`timeLimit` 和 `memoryLimit`，由 Hydro 题目 `config` 生成展示信息。

## 响应

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "items": [],
    "total": 0,
    "page": 1,
    "limit": 20
  }
}
```
