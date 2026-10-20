# 作业 API 与页面

## 接口

| 方法 | 路径 | 用途 |
|---|---|---|
| GET | `/api/v1/homeworks` | 作业列表 |
| POST | `/api/v1/homeworks` | 创建作业 |
| GET | `/api/v1/homeworks/:tid` | 作业详情 |
| GET | `/api/v1/homeworks/:tid/edit` | 作业编辑数据 |
| PATCH | `/api/v1/homeworks/:tid` | 更新作业 |
| GET | `/api/v1/homeworks/:tid/problems` | 作业题目列表 |

Hydro 中作业是 `document` 集合里 `docType=30`、`rule=homework` 的文档。比赛列表已排除 homework，避免比赛和作业混在一起。

## 前端

页面结构对应 Hydro `ui-default`：

- `homework_main.html`：作业列表、搜索和时间信息
- `homework_detail.html`：作业说明、题目表格和侧栏
- `homework_edit.html`：作业创建和编辑表单

路由：

- `/homework`：作业列表和标题搜索
- `/homework/new`：创建作业
- `/homework/:tid/edit`：编辑作业
- `/homework/:tid`：作业说明和题目列表
- `/homework/:tid/files`：作业公开文件

列表读取 `beginAt`、`endAt` 和 `penaltySince`，详情通过作业题目接口读取题目标题、统计和难度。创建/编辑页由 `HydroTemplate` 直接渲染上游 `homework_edit.html`，React 只负责取数、表单提交和交互挂载，不再维护一份手写的等价 DOM。

## 尚未完成

- 作业分组和 assign 访问控制
- 作业题目依赖和进度
- 作业提交和罚时规则
