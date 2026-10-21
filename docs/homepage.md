# 首页

首页由 `HydroTemplate` 直接渲染上游 `main.html`，区块通过 `partials/homepage/*.html` 组合，布局与上游默认配置一致（9/3 两栏）。

## 区块

| 区块 | 数据来源 | 状态 |
|---|---|---|
| `contest` | 比赛列表 API | 已接入 |
| `ranking` | 排行榜 API（前 10 名，More 指向 `/ranking`） | 已接入 |
| `homework` | 作业列表 API | 已接入 |
| `training` | 训练列表 API（章节/题目数来自聚合字段） | 已接入 |
| `discussion` | 讨论列表 API + 节点标题 | 已接入 |
| `recent_problems` | 题目列表 API（最近创建） | 已接入 |
| `discussion_nodes` | 讨论节点 API | 已接入 |
| `problem_search` | 静态搜索框（提交到 `/p?q=`） | 已接入 |
| `suggestion` | 静态推荐链接 | 已接入 |
| `bulletin` | `system` 集合的 `bulletin`（管理员在 `/admin/settings` 编辑） | 已接入 |
| `hitokoto` | 外部一言 API | 未接入 |
| `starred_problems` | 需要题目收藏状态 | 未接入 |

未接入的区块不会出现在首页，其余区块为空时自动隐藏（公告为空时 `partials/homepage/bulletin.html` 自身不会渲染）。

## 已知差异

- 比赛/作业的「已参加 / 已认领」标记需要列表级参赛状态，暂不显示
- 训练的报名进度依赖列表级报名数据，暂不显示
- 区块配置目前写在前端（等价于上游 `hydrooj.homepage` 默认配置的可用子集），后续可迁移到服务端设置
