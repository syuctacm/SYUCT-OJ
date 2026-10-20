# 比赛 API 与页面

## 接口

| 方法 | 路径 | 用途 |
|---|---|---|
| GET | `/api/v1/contests` | 比赛列表 |
| GET | `/api/v1/contests/:tid` | 比赛详情 |
| GET | `/api/v1/contests/:tid/problems` | 比赛题目列表 |
| GET | `/api/v1/contests/:tid/scoreboard` | 基础 ACM 榜单 |

数据直接读取 Hydro `document` 集合中的 `docType=30` 文档。

## 前端

- `/contest`：比赛列表
- `/contest/:tid`：比赛详情

当前列表展示比赛名称、赛制、开始/结束时间和参赛人数；详情展示比赛说明、基础榜单和按比赛顺序排列的题目列表。题目行提供比赛提交入口。比赛结束或未开始时后端拒绝比赛提交。

`lockAt` 到达且未设置 `unlocked` 时，封榜后的提交不会计入榜单。

## 尚未完成

- 比赛题目的时间/权限访问控制
- 比赛报名、参赛状态和比赛内提交
- OI 等赛制榜单和封榜后的解封逻辑
- 封榜、打印、气球、答疑
