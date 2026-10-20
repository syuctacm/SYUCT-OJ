# 比赛 API 与页面

## 接口

| 方法 | 路径 | 用途 |
|---|---|---|
| GET | `/api/v1/contests` | 比赛列表 |
| GET | `/api/v1/contests/:tid` | 比赛详情 |
| GET | `/api/v1/contests/:tid/problems` | 比赛题目列表 |

数据直接读取 Hydro `document` 集合中的 `docType=30` 文档。

## 前端

- `/contest`：比赛列表
- `/contest/:tid`：比赛详情

当前列表展示比赛名称、赛制、开始/结束时间和参赛人数；详情展示比赛说明、题目数量和按比赛顺序排列的题目列表。

## 尚未完成

- 比赛题目的时间/权限访问控制
- 比赛报名、参赛状态和比赛内提交
- ACM/OI 等赛制榜单
- 封榜、打印、气球、答疑
