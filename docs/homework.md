# 作业 API 与页面

## 接口

| 方法 | 路径 | 用途 |
|---|---|---|
| GET | `/api/v1/homeworks` | 作业列表 |
| POST | `/api/v1/homeworks` | 创建作业 |
| GET | `/api/v1/homeworks/:tid` | 作业详情 |
| GET | `/api/v1/homeworks/:tid/edit` | 作业编辑数据 |
| PATCH | `/api/v1/homeworks/:tid` | 更新作业 |
| GET | `/api/v1/homeworks/:tid/problems` | 作业题目列表（含认领状态和每题最近提交） |
| POST | `/api/v1/homeworks/:tid/attend` | 认领作业（幂等） |

Hydro 中作业是 `document` 集合里 `docType=30`、`rule=homework` 的文档。比赛列表已排除 homework，避免比赛和作业混在一起。

## 前端

页面结构对应 Hydro `ui-default`：

- `homework_main.html`：由 `HydroTemplate` 直接渲染作业列表、搜索、状态和时间信息
- `homework_detail.html`：由 `HydroTemplate` 直接渲染作业说明、题目表格、认领入口和侧栏
- `homework_edit.html`：作业创建和编辑表单

路由：

- `/homework`：作业列表和标题搜索
- `/homework/new`：创建作业
- `/homework/:tid/edit`：编辑作业
- `/homework/:tid`：作业说明和题目列表
- `/homework/:tid/scoreboard`：作业榜单（复用比赛榜单页）
- `/homework/:tid/file`：作业公开文件（渲染上游 `homework_files.html`）
- `GET /api/v1/homeworks/:tid/code`：导出作业提交代码 zip（同比赛导出规则）

列表读取 `beginAt`、`endAt` 和 `penaltySince`，详情通过作业题目接口读取题目标题、统计、难度、认领状态和每题最近提交（rid/status/score）。题目表格沿用上游逻辑：未认领且作业未结束时只显示认领提示，认领后才显示题目和每题状态列（状态图标、分数和最后提交时间）。认领写入 `document.status`（`docType=30`, `attend=1`），重复认领幂等，首次认领递增 `attend` 人数。

创建/编辑页由 `HydroTemplate` 直接渲染上游 `homework_edit.html`，React 只负责取数、表单提交和交互挂载，不再维护一份手写的等价 DOM。作业列表的搜索由 React 适配层保持，日历视图因尚未实现而隐藏；列表按上游默认每页 20 条分页。

## 尚未完成

- 作业分组和 assign 访问控制
- 作业题目依赖和进度
- 作业提交的罚时规则和延期计算（榜单目前复用比赛榜单逻辑）
- 作业代码导出入口（当前为占位）
