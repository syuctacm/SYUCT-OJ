# 作业 API 与页面

## 接口

| 方法 | 路径 | 用途 |
|---|---|---|
| GET | `/api/v1/homeworks` | 作业列表 |
| GET | `/api/v1/homeworks/:tid` | 作业详情 |

Hydro 中作业是 `document` 集合里 `docType=30`、`rule=homework` 的文档。比赛列表已排除 homework，避免比赛和作业混在一起。

## 前端

- `/homework`：作业列表
- `/homework/:tid`：作业详情

当前展示作业标题、起止时间、题目数量和说明。

## 尚未完成

- 作业分组和 assign 访问控制
- 作业题目依赖和进度
- 作业提交和罚时规则
- 作业管理页面
