# 题目 API

## 接口

| 方法 | 路径 | 用途 |
|---|---|---|
| GET | `/api/v1/problems` | 题目列表、分页、搜索和筛选 |
| GET | `/api/v1/problems/:pid` | 题目详情，支持 `pid`、`docId` 或 ObjectId |
| POST | `/api/v1/problems` | 创建题目 |
| GET | `/api/v1/problems/:pid/edit` | 编辑数据和原始 YAML 配置 |
| PATCH | `/api/v1/problems/:pid` | 更新题目和判题配置 |
| GET | `/api/v1/problems/:pid/assets` | 测试数据和附加文件列表 |
| POST | `/api/v1/problems/:pid/assets` | multipart 上传文件 |
| DELETE | `/api/v1/problems/:pid/assets` | 批量删除文件 |
| GET | `/api/v1/problems/:pid/assets/download` | 下载单个文件 |
| GET | `/api/v1/problems/:pid/assets/download-all` | 打包下载全部测试数据 |

## 列表参数

- `page`：默认 `1`
- `limit`：默认 `20`，最大 `100`
- `q`：按标题或题号模糊搜索
- `tag`：按标签筛选
- `difficulty`：按难度筛选
- `sort`：`default` 或 `recent`

登录请求会为每题附带 `userStatus`，包含当前用户最近一次提交的 `status` 和 `rid`。

默认只返回 `domainId=system`、`docType=10` 且未隐藏的题目；管理员登录后列表会包含隐藏题目，模板会显示 “(隐藏)” 标记。

## 随机选题

题库侧栏的“试试手气”对应 `GET /api/v1/problems/random?q=...`：

- 从可见题目中随机取一道，按 `q` 里的 `category:` 标签过滤（与上游 `ProblemRandomHandler` 一致）
- 前端 `/problem/random` 取到 pid 后跳转题目详情

## 删除题目

题目编辑页的“删除”按钮对应 `DELETE /api/v1/problems/:pid`（owner 或管理员），会清理：

- 题目文档与 `document.status`
- `problem/<domain>/<docId>` 下的测试数据和附加文件（元数据随文档删除，物理目录由 `FileStore.DeleteProblemDir` 删除）
- 该题的所有提交记录、`record.stat`、判题任务
- 题解（docType 11）和题目下的讨论（docType 21/22）

比赛/训练/作业中对题目的引用不会自动清理（与上游一致；这些页面会把问题视为缺失）。

## 提交计数

题目的 `nSubmit` / `nAccept` 实时维护：

- 创建提交成功后 `nSubmit + 1`
- 判题结果从非 AC 变为 AC 时 `nAccept + 1`，从 AC 变为非 AC（rejudge）时 `nAccept - 1`，且不会小于 0

不再依赖上游 `problemStat` 离线脚本；`problem.Repository.IncStats` 使用聚合管道更新，字段缺失时按 0 处理。

## 数据来源

直接读取 Hydro MongoDB 的 `document` 集合，不复制题目到新表。用户提交状态读取同库的 `record` 集合。

## 前端

题库页由 `HydroTemplate` 直接渲染 Hydro `ui-default`：

- `problem_main.html`：搜索栏、排序、题库表格和侧栏
- `partials/problem_list.html`：分页、题目标题、标签、状态、AC/Tried 和难度列
- `components/paginator.html`：原版分页宏
- `components/problem.html`：题目链接和标签 DOM

搜索、排序、标签筛选、分页、编辑模式、批量隐藏/取消隐藏/删除和题目包下载由 React 适配层接入；多域复制不迁移。


题目详情和提交页由 `HydroTemplate` 直接渲染上游模板：

- `problem_detail.html`：题目标题、标签、题面、操作区和侧栏
- `problem_submit.html`：语言选择、代码输入和提交结构
- `partials/problem_description.html`：题面 Markdown 输出
- `partials/problem_sidebar.html`：提交入口和题目信息侧栏

题面使用与 Hydro 编辑器一致的 markdown-it 插件链（KaTeX、脚注、高亮、图片尺寸、媒体和 XSS）。提交页的代码框由 Monaco 挂载，语言切换会同步语法模式，文本文件会读入编辑器；React 适配层负责语言下拉、提交请求和 Ctrl/Cmd+Enter。

详情 API 额外返回 `problemType`、`timeLimit` 和 `memoryLimit`，由 Hydro 题目 `config` 生成展示信息。

### 比赛 / 作业题目模式

比赛和作业题目链接保留上游的 `?tid=<ObjectId>` 查询参数。详情页根据 `contest.rule` 切换：

- `contest_detail_problem`
- `homework_detail_problem`

`GET /api/v1/problems/:pid?tid=...` 在返回题目的同时返回比赛、个人状态、全部题目状态和 `contestMode`。后端校验题目属于该比赛，并复刻上游访问规则：

- 比赛未开始：403
- 比赛中未报名：403
- 首次打开比赛题目或题目列表：写入个人 `startAt`
- 个人时长、提前结束时间和比赛结束时间共同决定实际截止时间
- 比赛结束后仍可查看题目，模式标记为 `view` / `correction`

前端直接复用上游 `problem_sidebar_contest.html`、`problem_sidebar_homework.html`、`contest_sidebar.html` 和 `homework_sidebar.html`，保留比赛题目 A/B/C 导航、已提交状态、比赛内提交链接、比赛侧栏和倒计时。普通 `/p/:pid` 仍使用 `problem_detail` 模式。

### 多语言题面

Hydro 将多语言题面存为以语言代码为键的 JSON 对象。`contentLang` 和 `content` 过滤器现在按同一结构解析：

- 单语言 Markdown/HTML 字符串保持原行为
- JSON 对象返回全部语言 Tab
- `?lang=` 精确选择语言；未命中时回退到用户语言、中文或第一个可用语言
- 语言切换只更新 `lang` 参数，比赛/作业的 `tid` 和其他查询参数保持不变
- Tab 继续使用上游 `data-lang`、`section__tab-header-item` 和 `tab--active` DOM

### Scratchpad

题目详情页恢复上游 `problem-sidebar__open-scratchpad` / `quit-scratchpad`：

- Alt+E 进入、Alt+Q 退出，使用上游 500ms/200ms 展开、收起和淡入淡出
- 复用上游 Allotment 布局、工具栏、Monaco、题目面板、预测试面板和记录面板
- 草稿按用户、题目和比赛维度写入 IndexedDB `hydro/scratchpad-drafts`
- F9 调用 pretest API，F10 使用现有正式提交 API
- 比赛/作业模式保留 `tid`，退出后恢复原题面 DOM 和页面滚动状态
- 编辑器字号、Tab 宽度和主题写入本地 `editor.config`

题目创建和编辑页由 `HydroTemplate` 直接渲染上游 `problem_edit.html`：

- `/p/new`
- `/p/:pid/edit`
- PID、标题、标签、难度、可见性、Markdown 题面和原版分类/文件侧栏
- Markdown 编辑器上传写入题目附加文件，并刷新原版附加文件侧栏
- 管理员可创建，管理员或题目所有者可编辑
- React 只负责数据、提交和上传接入，不复制主表单 DOM

判题配置页对应 `problem_config.html`：

- `/p/:pid/config` 直接渲染上游 `problem_config.html`
- 复用上游 `components/problemconfig` 的 Monaco YAML 编辑器、基础表单、题目类型和子任务树
- YAML 与表单双向同步，非法配置进入 Errors 页签
- 保存仍走现有 `PATCH /api/v1/problems/:pid`，不新增配置存储
- 测试数据侧栏继续接入上传、删除和全部下载接口

题目文件页由 `HydroTemplate` 直接渲染 `problem_files.html` / `partials/problem_files.html`：

- `/p/:pid/files`
- 测试数据与附加文件分栏
- 多文件上传、单文件下载、全部测试数据打包下载和批量删除
- 配置入口和原版题目信息侧栏
- 仅管理员或题目所有者可访问
- 生成测试数据使用 `hydrojudge` 原生 `type=generate` 任务，生成器和标准程序从现有 testdata 中自动补全选择

`GET /api/v1/problems/:pid/assets/download-all` 返回包含全部测试数据和 `config.yaml` 的 zip；附加文件不进入该压缩包。测试数据为空时返回 404，未授权用户返回 403。

## 题解

题解使用 Hydro 的 `document` 文档（`docType=11`），页面直接渲染上游模板：

- `/p/:pid/solution`：对应 `problem_solution.html`，按票数倒序分页
- `/p/:pid/solution/:sid`：单篇题解视图，与上游 `problem_solution_detail` 一致
- `components/comments_solution.html`：题解正文、投票按钮、楼中楼回复和编辑/删除操作

接口：

| 方法 | 路径 | 用途 |
|---|---|---|
| GET | `/api/v1/problems/:pid/solutions` | 分页列表；`sid` 返回单篇 |
| POST | `/api/v1/problems/:pid/solutions` | 发表题解 |
| PATCH | `/api/v1/problems/:pid/solutions/:sid` | 编辑题解（作者或管理员） |
| DELETE | `/api/v1/problems/:pid/solutions/:sid` | 删除题解（作者或管理员） |
| POST | `/api/v1/problems/:pid/solutions/:sid/vote` | 投票，`value` 为 `1`、`-1` 或 `0` |
| POST | `/api/v1/problems/:pid/solutions/:sid/replies` | 回复题解 |
| PATCH | `/api/v1/problems/:pid/solutions/:sid/replies/:rid` | 编辑回复（回复作者或管理员） |
| DELETE | `/api/v1/problems/:pid/solutions/:sid/replies/:rid` | 删除回复（回复作者或管理员） |

投票沿用 Hydro 的增量语义：重复投同一票是空操作，改票只累计差值。未登录用户可浏览，登录用户可以发表、投票和回复。

## 题目统计

`/p/:pid/statistics` 直接渲染上游 `problem_statistics.html`：

- 状态饼图和分数柱状图由 echarts 绘制，数据来自 `GET /api/v1/problems/:pid/statistics`
- 提交表格支持 `sort`（`time` / `memory` / `length` / `date`）、`direction`（`1` / `-1`）和 `lang` 过滤，改动下拉框即提交
- 表格里的代码大小由 `$strLenCP(code)` 在 MongoDB 侧计算，响应里不带代码本身
- 统计数字按需聚合 `record` 集合（状态计数 + 0..100 分数直方图），不依赖上游的 `problemStat` 离线脚本

接口：

| 方法 | 路径 | 用途 |
|---|---|---|
| GET | `/api/v1/problems/:pid/statistics` | `page` / `limit` / `sort` / `direction` / `lang` |

## Hydro 压缩包导入

迁移自上游核心的 `problem_import_hydro`（`/problem/import/hydro`，仅管理员）：

- 支持 Hydro 导出格式（每个子目录一份 `problem.yaml`）和 ICPC 题包格式（根目录 `problem.yaml`）
- 题面取 `problem.md`、`problem_<lang>.md`（优先 `zh`）或 `problem.yaml` 的 `content`
- 判题配置合并 `testdata/config.yaml`、`domjudge-problem.ini`（`timelimit` / `externalid`）与 `problem.yaml` 的 `limits`
- 目录映射沿用上游：
  - `testdata/`、`attachments/`、`generators/`、`include/`、`data/<case>/` → 测试数据
  - `additional_file/`、`attachments/`、`statement/`、`problem_statement/`、`data/sample/` → 附加文件
  - `output_validators/checker/`、`interactor/` → 测试数据并写入 `checker_type` / `type`
  - `solution/` → 题解
- 支持 `preferredPrefix`（替换 pid 前缀）、`hidden`、`keepUser`（保留原 owner，仅管理员）
- pid 冲突时自动改用 `P<docId>`，与上游默认行为一致

接口：

| 方法 | 路径 | 用途 |
|---|---|---|
| POST | `/api/v1/problems/import/hydro` | multipart `file`（zip）+ `preferredPrefix` / `hidden` / `keepUser`，返回 `{items, failures}` |

隐藏题目在导入后仍可被管理员和其 owner 通过详情接口查看（对齐上游 `PERM_VIEW_PROBLEM_HIDDEN` 行为）。

## FPS 导入

迁移自上游 `fps-importer` 插件（`/problem/import/fps`，仅管理员）：

- 支持单个 FPS XML 或包含多个 XML 的 zip，单文件上限 64MiB
- 每个 `<item>` 创建一道题：标题、HTML 题面、来源拆成标签、`time_limit` / `memory_limit` 写入 `config.yaml`
- 测试数据按 `filename` → `name` → `<序号>.<in|out>` 的顺序命名；只有 `test_input` 的 HUSTOJ 导出按 in/out 成对处理
- `<img>` 里的 base64 图片写入附加文件，题面里的 `src` 替换为 `file://<文件名>`
- `<solution>` 生成对应题解
- 题面默认按 HTML 存储（`html: true`），命中 `[md]` 标记时按 Markdown 处理

接口：

| 方法 | 路径 | 用途 |
|---|---|---|
| POST | `/api/v1/problems/import/fps` | multipart `file`，返回 `{items, failures}` |

导入失败的单条会记录在 `failures` 里，其余继续导入；题库侧栏的 “Create Problem” 区域会同时显示“创建题目”和“从 FPS 文件导入”。

## 判题配置页

`/p/:pid/config` 由 `HydroTemplate` 渲染上游 `problem_config.html`：标题「评测设置(Beta)」、testdata 侧栏（`partials/problem_files.html`，支持上传）和题目侧栏都来自原模板。

`#ProblemConfig` 是上游留给编辑器的挂载点，当前由 `web/src/components/ProblemConfigPanel.tsx` 填充，内部直接复用 `web/src/upstream/problemconfig/` 中从 `@hydrooj/ui-default@4.58.5` 迁移的编辑器、表单、子任务树和 reducer。Monaco 使用 `monaco-yaml` 和上游 schema；编辑器只挂载在配置页，并通过动态 import 隔离到独立 chunk，避免影响首屏。

## 语言列表

| 方法 | 路径 | 用途 |
|---|---|---|
| GET | `/api/v1/languages` | 判题语言注册表（`internal/judge.DefaultLanguages()` 过滤 hidden/remote） |
| GET | `/api/v1/problems/:pid/langs` | 该题可用的语言，可带 `?contest=<tid>` 叠加比赛限制 |

语言列表由服务端提供：注册表来自服务端向判题机广播的那份 `DefaultLanguages()`，再与题目 `config.langs`、比赛 `langs` 求交集（与上游 `handler/problem.ts` 的 intersection 语义一致）。提交页默认选中列表第一项，提交记录的语言筛选也走同一份注册表；前端保留一份静态列表作为首屏兜底。

## 题目收藏

| 方法 | 路径 | 用途 |
|---|---|---|
| POST | `/api/v1/problems/:pid/star` | 收藏 / 取消收藏（`{star: bool}`） |
| GET | `/api/v1/starred-problems` | 当前用户收藏的题目（默认 50 条，按收藏时间倒序） |

状态存在 `document.status`（`docType=10`、`docId=题目 docId`、`star`），与 Hydro `problem.setStar` 一致。题目详情接口会返回 `starred`，模板据此决定按钮的 `activated` 状态；首页的 `starred_problems` 区块读取同一份数据。

## 文件存储

存储布局与 Hydro 对齐：

```text
problem/system/<docId>/testdata/<name>
problem/system/<docId>/additional_file/<name>
```

上传 `config.yaml` 或 `config.yml` 时，内容会同步写入题目的 `config` 字段，供判题机读取。编辑接口会将 Hydro 的 `- key: ... / value: ...` 配置列表规范化为 YAML 对象，避免旧数据编辑后无法保存。

## 已知差异

- 题目分类侧栏仍使用标签统计，不读取旧多域时代的分类配置
- 题目的特殊 checker/交互题配置需要与判题机协议做进一步兼容验证
- 题目文件的批量重命名、批量下载、创建、重命名和测试数据生成已实现；活动文件页不属于题目文件工具

## 响应

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "items": [],
    "total": 0,
    "page": 1,
    "limit": 20
  }
}
```
