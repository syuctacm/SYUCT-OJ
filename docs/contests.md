# 比赛 API 与页面

## 接口

| 方法 | 路径 | 用途 |
|---|---|---|
| GET | `/api/v1/contests` | 比赛列表 |
| POST | `/api/v1/contests` | 创建比赛 |
| GET | `/api/v1/contests/:tid` | 比赛详情（含当前用户是否已报名） |
| POST | `/api/v1/contests/:tid/attend` | 比赛报名（幂等） |
| GET | `/api/v1/contests/:tid/edit` | 比赛编辑数据 |
| PATCH | `/api/v1/contests/:tid` | 更新比赛 |
| GET | `/api/v1/contests/:tid/problems` | 比赛题目列表 |
| GET | `/api/v1/contests/:tid/scoreboard` | ACM 榜单（含参赛者、每题 AC 记录 id） |
| GET | `/api/v1/contests/:tid/clarifications` | 答疑列表（jury 公告 + 自己的提问） |
| POST | `/api/v1/contests/:tid/clarifications` | 提问（需报名且比赛进行中） |
| POST | `/api/v1/contests/:tid/clarifications/:cid/reply` | 答疑回复（所有者/维护者/管理员） |
| POST | `/api/v1/contests/:tid/clarifications/broadcast` | jury 广播（所有者/维护者/管理员） |
| POST | `/api/v1/contests/:tid/unlock` | 解除封榜（所有者/维护者/管理员，比赛结束后） |

数据直接读取 Hydro `document` 集合中的 `docType=30` 文档。列表支持 `q` 标题搜索和 `rule` 赛制筛选。

## 前端

- `/contest`：比赛列表
- `/contest/new`：创建比赛
- `/contest/:tid`：比赛详情
- `/contest/:tid/problems`：比赛题目列表
- `/contest/:tid/scoreboard`：比赛榜单
- `/contest/:tid/clarification`：比赛答疑管理（jury 回复/广播）
- `/contest/:tid/edit`：编辑比赛

页面结构对应 Hydro `ui-default`：

- `contest_main.html`：由 `HydroTemplate` 直接渲染进行中/即将开始卡片、全部比赛列表、搜索和赛制筛选
- `contest_detail.html`：由 `HydroTemplate` 直接渲染比赛信息、说明、文件、报名入口和比赛侧栏
- `contest_problemlist.html`：由 `HydroTemplate` 直接渲染题目表、本场提交表和答疑区块
- `contest_scoreboard.html`：由 `HydroTemplate` 直接渲染榜单表格、封榜提示和导出按钮位置

比赛列表的搜索和赛制筛选由 React 适配层保持；未实现的团队管理入口会隐藏。列表按上游默认每页 20 条分页，分页链接会保留搜索和赛制筛选参数。

比赛详情不再内嵌题目列表和榜单，改为原版结构下的独立路由 `/contest/:tid/problems` 和 `/contest/:tid/scoreboard`，两个页面都已迁移到上游模板。题目行提供带 `contest` 参数的提交入口，提交会写入比赛关联并进入榜单。比赛结束或未开始时后端拒绝比赛提交。

比赛报名写入 `document.status`（`docType=30`, `attend=1`），重复报名幂等，首次报名递增 `attend` 人数。详情页返回当前登录用户的 `attended` 状态，未登录显示登录入口，报名后侧栏显示已参加状态、题目列表和我的递交入口。

答疑沿用 Hydro `document` 集合布局：`docType=31`、`parentType=30`、`parentId=比赛 docId`，`owner=0` 表示 jury 公告，`reply` 保存回复数组。普通参赛者只能看到 jury 公告和自己的提问；比赛所有者、维护者和管理员可以看到全部答疑。提问要求已报名且比赛进行中，回复仅限所有者/维护者/管理员。

比赛答疑管理页由 `HydroTemplate` 直接渲染上游 `contest_clarification.html`：所有者/维护者/管理员可以看到全部答疑，点击「回复」填充目标答疑后提交；表单不指定答疑时作为 jury 广播发给全体参赛者。比赛详情、题目列表和榜单页的「Contest Management」入口指向该页。

题目列表页复用上游结构：题目表显示每题状态（最近一次提交）、时间/内存限制标签和带比赛参数的提交入口；「本场提交」表列出当前用户在比赛中的提交（状态、题目、提交者、耗时、内存、语言和提交时间）；「答疑」区块从答疑 API 读取可对本用户展示的问答，并允许已报名且比赛进行中的用户提问。题目列表接口同时返回 `attended` 和每题最近提交，题目配置摘要提供 timeMin/timeMax/memoryMin/memoryMax。

榜单页把榜单 API 的数据转换成上游 `partials/scoreboard.html` 需要的行列结构：`rank`/`user`/`solved` 列加上每题 `problem`/`record` 单元，题目表头显示本场比赛的过题/提交数，AC 和失败提交都能链接到对应记录（`rid`）。已报名但没有提交的用户也会以 0 分出现在榜单上。封榜时显示原版提示，封榜后的提交不计入榜单。

比赛创建/编辑支持：

- 赛制、标题、开始时间和时长
- 按 docId 顺序配置题目
- Rated、Markdown 比赛说明和提交语言限制
- 比赛管理员、公开/邀请码/指定分组访问方式
- 自动隐藏题目、赛后查看代码、打印和组队开关
- 比赛管理页 `/contest/:tid/management` 上传、下载和删除公开文件与参赛者资料
- 管理员创建；管理员或比赛所有者编辑
- 时间窗口、题目存在性和赛制校验

编辑页由 `HydroTemplate` 直接渲染上游 `contest_edit.html`，包括原版 section/form 层级、文件侧栏和 Markdown 图片上传。React 只负责数据、提交和交互挂载。

`lockAt` 到达且未设置 `unlocked` 时，封榜后的提交不会计入榜单。比赛结束后，比赛所有者/维护者/管理员可在榜单页点击「解封」清除封榜限制。

榜单页支持导出：HTML 按钮打开当前榜单页，CSV 按钮在前端生成下载（列：排名、用户、过题数、罚时和每题状态，如 `AC(1)/62` 或 `-2`）。

## 比赛管理页

`/contest/:tid/management`（旧路径 `/contest/:tid/files` 保留为别名）由 `HydroTemplate` 渲染上游 `contest_manage.html`：

- 题目分值表：列出 `tdoc.pids` 与 `tdoc.score[pid]`（默认 100）
- 公开文件区：`partials/files.html` + `type=public`
- 参赛者资料区：`partials/files.html` + `type=private`，只有已报名且比赛已开始的参赛者能下载
- 侧栏：`partials/contest_sidebar_management.html`

资料存放于 `contest/system/<tid>/private/<name>`，元数据写入比赛文档的 `privateFiles`。下载接口按上游规则鉴权：所有者、系统管理员或已报名的参赛者（比赛开始后）。比赛详情页按上游逻辑展示参赛者可下载的资料。

## 尚未完成

- 比赛题目分值编辑（`name="set_score"` 目前是占位）
- 参赛者管理页、导出所有代码和气球状态页
- 比赛题目的时间/权限访问控制
- 组队和参赛状态（个人报名已实现，提交暂未强制要求报名）
- 比赛管理中的气球、参赛者管理和导出代码入口（当前为占位）
- 打印入口
- 未报名用户访问题目列表时显示提示（上游为错误页）
- 本场提交表最多展示 100 条
- Ghost 榜单导出和关注/筛选下拉
- 首次通过高亮
- 提前结束比赛操作
- OI 等赛制榜单和封榜后的解封逻辑
