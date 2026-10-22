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
| `hitokoto` | `https://v1.hitokoto.cn`（浏览器直接请求，失败时显示提示） | 已接入 |
| `starred_problems` | 收藏题目 API（`document.status` docType=10、`star=true`） | 已接入 |
| `checkin` | `GET/POST /api/v1/user/checkin`，用户文档 `checkin_*` 字段 | 已接入 |
| `countdown` | `GET /api/v1/home/countdown`，由 `COUNTDOWN_*` 环境变量配置 | 已接入 |

区块顺序对齐上游默认配置（9 栏：bulletin/contest/homework/training/ranking/discussion；3 栏：checkin/countdown/hitokoto/starred_problems/recent_problems/discussion_nodes/suggestion，另加了搜索框）。区块为空时多数 partial 自身不渲染（公告、收藏题目、一言以外的列表类）。

## 签到与倒计时

签到兼容原 `checkin` addon 的用户字段：

- `checkin_time`：最后签到日期（`YYYY-MM-DD`，按 Asia/Shanghai）
- `checkin_luck`：当日运势 0-6
- `checkin_cnt_now`：连续签到天数
- `checkin_cnt_all`：累计签到天数

每天只允许签到一次，重复请求返回当天结果。首页未登录时显示登录提示，登录后显示抽取按钮或当日运势。

倒计时只展示未过期事件，配置示例：

```bash
COUNTDOWN_TITLE=倒计时
COUNTDOWN_MAX=5
COUNTDOWN_DATES='[{"name":"CSP-J/S 初赛","date":"2026-09-19"}]'
```

## 已知差异

- 比赛/作业的「已参加 / 已认领」标记需要列表级参赛状态，暂不显示
- 训练的报名进度依赖列表级报名数据，暂不显示
- 区块配置目前写在前端（等价于上游 `hydrooj.homepage` 默认配置的可用子集），后续可迁移到服务端设置
