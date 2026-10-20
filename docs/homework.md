# 作业 API 与页面

## 接口

| 方法 | 路径 | 用途 |
|---|---|---|
| GET | `/api/v1/homeworks` | 作业列表 |
| GET | `/api/v1/homeworks/:tid` | 作业详情 |
| GET | `/api/v1/homeworks/:tid/problems` | 作业题目列表 |

Hydro 中作业是 `document` 集合里 `docType=30`、`rule=homework` 的文档。比赛列表已排除 homework，避免比赛和作业混在一起。

## 前端

页面结构对应 Hydro `ui-default`：

- `homework_main.html`：作业列表、搜索和时间信息
- `homework_detail.html`：作业说明、题目表格和侧栏

路由：

- `/homework`：作业列表和标题搜索
- `/homework/:tid`：作业说明和题目列表

列表读取 `beginAt`、`endAt` 和 `penaltySince`，详情通过作业题目接口读取题目标题、统计和难度。

## 尚未完成

- 作业分组和 assign 访问控制
- 作业题目依赖和进度
- 作业提交和罚时规则
- 作业管理页面
