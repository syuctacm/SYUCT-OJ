# 比赛 API 与页面

## 接口

| 方法 | 路径 | 用途 |
|---|---|---|
| GET | `/api/v1/contests` | 比赛列表 |
| POST | `/api/v1/contests` | 创建比赛 |
| GET | `/api/v1/contests/:tid` | 比赛详情 |
| GET | `/api/v1/contests/:tid/edit` | 比赛编辑数据 |
| PATCH | `/api/v1/contests/:tid` | 更新比赛 |
| GET | `/api/v1/contests/:tid/problems` | 比赛题目列表 |
| GET | `/api/v1/contests/:tid/scoreboard` | 基础 ACM 榜单 |

数据直接读取 Hydro `document` 集合中的 `docType=30` 文档。列表支持 `q` 标题搜索和 `rule` 赛制筛选。

## 前端

- `/contest`：比赛列表
- `/contest/new`：创建比赛
- `/contest/:tid`：比赛详情
- `/contest/:tid/edit`：编辑比赛

页面结构对应 Hydro `ui-default`：

- `contest_main.html`：进行中/即将开始比赛、全部比赛列表和筛选
- `contest_detail.html`：比赛信息、说明和比赛侧栏

详情下方保留题目列表和基础榜单。题目行提供带 `contest` 参数的提交入口，提交会写入比赛关联并进入榜单。比赛结束或未开始时后端拒绝比赛提交。

比赛创建/编辑支持：

- 赛制、标题、开始时间和时长
- 按 docId 顺序配置题目
- Rated、Markdown 比赛说明和提交语言限制
- 比赛管理员、公开/邀请码/指定分组访问方式
- 自动隐藏题目、赛后查看代码、打印和组队开关
- `/contest/:tid/files` 公开文件上传、下载和删除
- 管理员创建；管理员或比赛所有者编辑
- 时间窗口、题目存在性和赛制校验

编辑页对应 Hydro `contest_edit.html` 的 section/form 结构。

`lockAt` 到达且未设置 `unlocked` 时，封榜后的提交不会计入榜单。

## 尚未完成

- 比赛私有文件
- 比赛题目的时间/权限访问控制
- 比赛报名、组队和参赛状态
- OI 等赛制榜单和封榜后的解封逻辑
- 封榜、打印、气球、答疑
