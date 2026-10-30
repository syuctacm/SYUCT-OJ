# 前端视觉与交互对照清单

## 1. 原则

- 以 Hydro `ui-default` 模板和旧站实际输出的 HTML 为唯一结构来源，不自行设计近似页面。
- 复用旧站 CSS、字体、图标、类名和 DOM 层级。
- 旧插件样式必须从原站 HTML/源码原样移入 `web/src/legacy/`，不得凭截图手写近似 CSS。
- React 壳层只负责路由、取数和交互挂载；已迁移页面必须通过 `HydroTemplate` 渲染上游 Nunjucks 模板，不在 `.main` 中额外包 `.row`。
- 禁止在 TSX 中复制已迁移页面的等价 DOM；结构差异只能通过模板 context/adapter 或升级上游模板解决。
- 路由必须同步原模板的 `data-page`、`data-layout` 和 `page--*` 类，页面级 CSS 才能生效。
- 截图只用于最后验收，不能代替源码对照。

## 2. 页面优先级

### P0 核心闭环

- 登录 / 登出
- 首页
- 题目列表
- 题目详情
- 代码提交
- 提交记录列表
- 提交详情 / 实时判题状态
- 用户中心基础页

### P1 常用功能

- 首页 / 比赛列表 / 比赛详情 / 比赛题目列表 / 比赛榜单 / 作业列表 / 作业详情 / 训练列表 / 训练详情 / 讨论列表 / 讨论详情 / 用户主页 / 排行榜
- 比赛题目列表
- 比赛榜单
- 训练列表 / 详情
- 作业列表 / 训练列表 / 详情
- 用户资料 / 活跃日历
- 标签统计榜单

### P2 管理功能

- 题目编辑 / 配置 / 文件
- 比赛答疑管理
- 比赛管理
- 训练管理
- 作业管理
- 域管理（删除，不迁移）
- 系统设置 / 用户管理

## 3. 交互检查

- 表单提交和错误提示
- 路由跳转和浏览器历史
- 分页、排序、筛选
- Markdown、KaTeX、代码高亮
- Monaco 编辑器
- 文件上传、下载、拖拽
- 实时 WebSocket 更新
- 比赛倒计时、榜单刷新
- 移动端布局
- 空状态、加载状态、403/404/500 页面

## 4. 自动化验证

- Playwright 登录和核心 E2E
- 原模板结构、类名和关键计算样式核对
- 最后阶段再做关键页面截图回归
- API 契约测试
- 迁移后数据页面抽查
- 权限角色抽查

## 5. 已知差异记录

每次迁移页面时记录：

```text
页面：
旧站行为：
新站行为：
差异级别：
是否接受：
负责人：
后续处理：
```

## 6. Phase 7 回归记录（2026-10-21，本地 Docker）

冒烟回归覆盖全部路由（匿名 + 管理员），页面均无模板渲染错误：

- 前台：`/`、`/p`、`/p/:pid`、`/p/:pid/submit`（未登录提示）、`/p/tag/:tag`、`/contest`、`/contest/:tid`、`/contest/:tid/scoreboard`、`/contest/:tid/problems`（未报名提示）、`/homework`、`/homework/:tid`、`/homework/:tid/scoreboard`、`/training`、`/training/:tid`、`/discuss`、`/discuss/node/:name`、`/discuss/:did`、`/user/:uid`、`/login`
- 登录后：`/record`、`/record/:rid`、`/admin`、`/admin/users`、`/p/new`、`/p/:pid/edit`、`/p/:pid/config`、`/p/:pid/files`、`/contest/new`、`/contest/:tid/edit`、`/contest/:tid/management`、`/homework/new`、`/homework/:tid/edit`、`/homework/:tid/file`、`/training/new`、`/training/:tid/edit`、`/training/:tid/file`、`/discuss/node/:name/create`、`/discuss/:did/edit`
- `/discuss/new` 无节点参数时按上游行为回到讨论首页

关键交互已在对应 PR 中逐项验证：

| 交互 | 验证位置 |
|---|---|
| 登录 / 登出 | #97、#133 |
| 题目搜索 / 分页 | #109 |
| 比赛搜索 / 赛制筛选 | #111 |
| 比赛报名 | #119 |
| 比赛答疑提问 | #127 |
| 榜单（封榜、记录链接、解封按钮位置） | #121 |
| 作业认领 / 状态列 | #123 |
| 训练报名 / 章节展开 / 状态列 | #115、#117 |
| 讨论回复 / 楼中楼 / 编辑 / 删除 / 锁定 | #131 |
| 讨论 Markdown 图片上传与匿名展示 | #245 |
| 标签榜单模板、搜索和 CSV 导出 | #247 |
| 头像上传与公开头像读取 | #249 |
| TOTP 两步验证与登录验证码 | #251 |
| 首页签到与倒计时原生模块 | #255 |
| onsite IP 自动登录映射 | #258 |
| 邮箱修改与确认链接 | #253 |
| 用户主页标签页 / 活跃日历 | #133 |
| 首页各区块 | #135 |
| 安全设置 Passkey 注册 / 登录 / 删除 | #260 |

## 6.1 全站冒烟清单（2026-10-21）

用浏览器逐条切换路由（`history.pushState` + `popstate`，每条等待 900ms），在匿名和 admin 两种状态下检查 `document.body.innerText`、`.main` 是否存在以及是否出现 `Template render error`。本次 42 条路由 × 2 状态全部通过。

路由清单：

```text
/  /p  /p/tag/:tag  /p/:pid  /p/:pid/submit  /p/:pid/solution  /p/:pid/statistics  /p/:pid/edit  /p/:pid/config  /p/:pid/files  /problem/import/fps  /problem/import/hydro
/contest  /contest/:tid  /contest/:tid/problems  /contest/:tid/scoreboard
/contest/:tid/management  /contest/:tid/clarification  /contest/:tid/user
/contest/new  /contest/:tid/edit
/homework  /homework/:tid  /homework/:tid/scoreboard  /homework/:tid/file  /homework/:tid/edit  /homework/new
/training  /training/:tid  /training/:tid/file  /training/:tid/edit  /training/new
/discuss  /discuss/node/:name  /discuss/:did  /discuss/:did/edit  /discuss/node/:name/create
/user/:uid  /ranking  /record  /record/:rid
/admin  /admin/users  /home/settings?category=account  /home/messages  /home/security  /file  /login
```

## 6.2 全站回归（2026-10-22）

在本地 Docker（MongoDB 容器 + Go API + Vite）上重跑全站冒烟，覆盖 2026-10-21 之后合并的题解、消息、安全、统计和 FPS 导入：

- 匿名 13 条 + 管理员 37 条路由，均无 `Template render error`，`.main` 全部存在
- 新增页面均已验证：`/p/:pid/solution`、`/p/:pid/statistics`、`/problem/import/fps`、`/home/messages`、`/home/security`
- 匿名访问 `/record` 显示“请先登录查看评测记录”，`/problem/import/fps` 跳转登录页，符合预期
- 回归发现并修复两处问题（issue #203）：
  - 比赛打印页在 `allowPrint=false` 时仍挂载 kiosk 面板并轮询 `/print/tasks`，控制台持续 404；现在与上游一致返回 404 占位
  - 挂载型组件（活跃日历、讨论历史/表情、消息 pad、判题配置面板、打印面板）在 StrictMode 下重复挂载会打印 createRoot/unmount 告警；改为按容器复用 root（`web/src/upstream/roots.ts`），回归后控制台 0 告警

## 6.3 核心路由最终回归（2026-10-22）

本地 Docker（MongoDB） + Go API + Vite 下按实际路由逐页加载：

- 匿名：18 条核心路由
- 管理员：34 条核心路由
- 共 52 条页面加载，全部存在 `.main`
- 未出现 `Template render error`
- 未出现 `HTTP 500`

回归覆盖首页、题库/题目/题解/统计/文件、比赛/榜单/答疑/打印/气球、作业、训练、讨论、记录、用户中心、安全设置、后台、标签榜单以及签到/倒计时首页区块。核心视觉结构来自上游 `@hydrooj/ui-default` 原模板和原插件模板；交互回归已在对应 PR 中逐项记录。

OAuth、onsite-toolkit 剩余线下能力和真实生产备份迁移仍在 #1 / #4 跟踪，不再阻塞 #6 的核心页面视觉验收。

## 6.4 WebAuthn / Passkey 回归（2026-10-25）

在本地 Docker MongoDB + Go API + Vite（`http://localhost:5174`，RPID `localhost`）下，用 Chromium CDP 虚拟认证器完成真实闭环：

- 密码登录后在 `/home/security` 选择 Passkey，注册并显示在上游 `autherlist` 模板中
- 登出后在 `/login` 输入用户名并使用 Passkey 登录，成功建立 Session 并返回首页
- “添加认证器”仍可选择 TOTP，二维码设置流程正常打开
- 移除 Passkey 后列表清空，再次尝试 Passkey 登录返回“该账号没有可用的 Passkey”
- challenge 跨用户、跨用途和过期校验由 `internal/webauthn/service_test.go` 覆盖

## 6.5 ui-default 全局运行时回归（2026-10-25）

已在本地 Docker MongoDB + Go API + Vite 环境中恢复并验证上游全局运行时。实现位于 `web/src/upstream/`，除路由、鉴权和 Go API 适配外，直接移植 `@hydrooj/ui-default@4.58.5` 的实现和模板，不再手写近似动画。

| 能力 | 来源 | 本地验证 |
|---|---|---|
| nav / mobile header / footer | `partials/*.html` | 桌面菜单、移动端侧滑、页脚展开和登出正常 |
| 页面入场动画 | `hydro.ts` / `slide.ts` | 首屏和 SPA 路由切换后 `.section` 依次进入 `visible`，计算样式最终为 `opacity: 1` |
| Page Loader / Mantine 主题 | `entry.js` / `mantine.page.tsx` | 加载时显示，初始化后隐藏；通知使用上游主题变量 |
| Dialog | `DomDialog.ts` / `dialog/index.tsx` | 淡入、缩放、关闭动画正常；确认框返回布尔值 |
| Tab | `tab/Tab.js` | 用户主页切页、`#tab-N` 和历史记录同步 |
| Tooltip / Dropdown | `tooltip` / `dropdown` | hover 打开 Tether 浮层，路由切换后不累积 |
| Sticky / StyledTable | `sticky` / `table` | 侧栏和表头吸顶正常，动态页面只包装一次 |
| Hotkey / Smooth Scroll / Form | 对应 upstream page | `Ctrl+K` 搜索、锚点滚动、表单 focus 与重复提交保护正常 |
| 全站搜索 | `omnisearch/index.page.tsx` | `Ctrl+K` 打开，可检索题目与用户 |

Playwright 回归结果：

- 管理员身份逐页加载 57 条路由、匿名身份加载 19 条核心路由，未出现 `Template render error`、HTTP 500 或 React 运行时异常
- 桌面 1280px：11 个首页 `.section` 全部完成入场动画，Page Loader 已隐藏
- 移动 390px：侧滑面板位移 `-200px`、overlay 与 hamburger 状态同步；点击 overlay 可关闭；页脚分类可展开
- SPA 导航、Tooltip/Dropdown 生命周期和 Sticky 清理正常，`#menu .nav_more` 与全站搜索入口始终各只有 1 个
- 题库、记录、榜单和训练页的 `.data-table` 均只生成一个 `.section__table-container`，没有重复嵌套
- 唯一预期控制台请求错误是未登录时 `GET /api/v1/auth/me` 返回 401

移动端不播放入场动画与上游样式一致：上游只在 `min-width: 600px` 下启用 `.section` 动画。

## 6.6 题解折叠内容回归（2026-10-25）

已从 `@hydrooj/ui-default@4.58.5` 直接移植 `components/discussion/collapsible.page.tsx` 到 `web/src/upstream/collapsible.ts`，并由全局 UI runtime 在页面和异步内容更新后初始化。

- 源码核对确认上游只对题解与题解回复应用 `collapsible-content`；讨论正文和回复本身没有折叠，因此没有修改模板制造额外差异
- 长题解从 300px 折叠状态显示“显示更多”，展开后按钮变为“显示更少”
- 短题解自动标记为 `expanded`，不显示折叠按钮
- 从带锚点 URL 打开时自动展开目标所在题解并滚动到目标位置
- 多次 SPA 路由切换后每个内容仍只有一个 `.collapsible-content__inner`
- 折叠与展开继续使用上游 0.4s `cubic-bezier(0.4, 0, 0.2, 1)` max-height 动画和渐变遮罩

## 6.7 多选 Checkbox 回归（2026-10-25）

已从 `@hydrooj/ui-default@4.58.5` 直接移植 `components/form/multiSelectCheckbox.page.js` 到 `web/src/upstream/checkboxes.ts`，并在全局 UI runtime 中只安装一次。

- 文件管理、活动文件和比赛管理页面不再各自手写 select-all 逻辑
- 表头全选和取消全选只作用于同组、可见且未禁用的 checkbox
- Shift 范围选择与上游一致
- SPA 路由切换后重新进入文件页，点击一次仍只选中当前项，没有重复监听导致的异常
- Playwright 在 `/file` 使用 5 个本地文件验证：全选 5 个、取消后 0 个、Shift 从第 1 到第 4 项选中 4 个

## 6.8 题目列表编辑模式回归（2026-10-25）

已恢复上游题目列表的编辑模式和选择状态展示，并接入单站点允许的批量隐藏、取消隐藏和删除。

- “进入编辑模式 / 退出编辑模式”使用上游的 body `display-mode` / `edit-mode` 类切换
- 选中题目的数量和上游选择对话框已恢复，支持按题目搜索追加选择、清空和复制 ID/Pid
- 原有全选与 Shift 范围选择继续由上游 multi-select runtime 处理
- 隐藏、取消隐藏、删除通过现有 Go Problem API 执行；删除使用上游样式 Dialog 二次确认
- 批量操作完成后清空选择并重新加载当前筛选结果
- 多域复制和批量下载不适合当前单站点/现有 API 范围，入口暂时不展示，批量下载另开 issue 跟踪
- Playwright 验证：3 道本地测试题进入编辑模式、三选、隐藏、取消隐藏、删除 1 道、退出编辑模式全部通过；操作后 `hidden` 状态和列表数量与数据库一致
- 390px 移动布局下编辑模式、选择区和批量菜单正常显示
- 修复了选择区 React root 在模板刷新时同步 unmount 的控制台告警，改用项目已有的 root 复用工具

## 6.9 SPA 导航与比赛倒计时回归（2026-10-30）

### SPA 导航

- 路由切换触发 `vjPageLoading`，新页面完成数据加载和 section 初始化后触发 `vjPageFullyInitialized` 并淡出 Page Loader
- Loader 会等待数据页完成，不会在仅渲染“加载中...”占位时提前消失；15 秒超时只作为异常兜底
- 普通前进导航滚到页面顶部，浏览器后退恢复离开时的 `scrollY`
- 浏览器原生 `scrollRestoration` 切换为 `manual`，避免浏览器恢复与 SPA 恢复互相覆盖
- Hash 地址不触发回到顶部，继续由 smooth-scroll 展开折叠内容并定位锚点
- 快速连续导航最终只保留一个 Loader，并最终正常隐藏
- MutationObserver 增加加载占位文本变化监听，修复 404 边界页从“加载中”改为“404 Not Found”时 Loader 卡住的问题

Playwright 验证：

- 从已滚动 1000px 的首页进入 `/p`：Loader 立即出现，`/p` 完成后消失，`scrollY=0`
- 在 `/p` 滚动到 300px 后进入题目详情再后退：恢复 `scrollY=300`
- 手动 Hash 定位后 `scrollY=1332`，没有被导航恢复逻辑拉回顶部
- 延迟 `/api/v1/problems` 1.5 秒时，Loader 在“加载中...”阶段持续显示，数据到达后消失
- 管理员 58 条路由全量回归：无 Loader 残留、无模板错误、无页面异常

### 比赛倒计时

- 移植上游 `contest.page.ts` 和 `formatSeconds`
- 比赛详情、题目列表和榜单均显示 `HH:MM:SS` 剩余时间
- NProgress 顶部进度条按比赛已用时间推进，关闭 trickle 和 spinner
- 未开始、已结束和作业页面不显示倒计时
- 离开比赛路由或提前结束比赛时清理计时器、DOM 节点和 NProgress
- Playwright 验证三个比赛页面倒计时每秒变化；进行中显示，已结束隐藏，作业页和离开后的 DOM 中均无计时器/NProgress 残留

## 6.10 记录筛选与实时更新回归（2026-10-30）

- 用户名/UID 和题目筛选复用上游 `UserSelectAutoComplete` / `ProblemSelectAutoComplete`
- 原模板输入框保留为表单数据源，自动补全值通过 `change` 同步，过滤和分页 URL 结构不变
- WebSocket 仍原位更新当前页已有提交状态
- 第一页收到新 rid 时防抖刷新当前筛选结果，不手写另一套记录行 DOM
- 第二页不插入新记录，与上游行为一致
- 筛选条件变化时重建 WebSocket，避免旧筛选条件继续生效
- SPA 离开页面时关闭 WebSocket、重连定时器和刷新防抖定时器

Playwright 验证：

- 用户补全选择 `alice` 后隐藏字段为 UID `2`
- 题目补全选择 `P201` 后隐藏字段为 docId `201`
- 提交过滤后 URL 为 `/record?uidOrName=2&pid=201`，表格正确收缩到 3 行
- 第一页创建新提交后无需刷新，新记录自动出现在首行并保持 20 行分页
- 浏览器控制台 0 error

## 6.11 用户偏好设置回归（2026-10-30）

恢复 `home_settings.html` 的 `category=preference` 分类，只提供当前站点实际消费的选项：

| 设置 | 生效位置 |
|---|---|
| Font Family | 根 CSS `--font-family`，立即更新 |
| Code Font Family | Monaco、代码 CSS `--code-font-family` |
| Skip Animation | 上游 section 入场动画 |
| Default Code Language | 提交页语言选择默认值 |
| Default Code Template | 提交页代码编辑器初始内容 |
| viewLang | `UserContext`，当前 UI 只有中文 |

Go User 模型和 settings API 增加字段及校验；非法语言、非法字体和超过 64 KiB 的代码模板返回 400。登录和 `/auth/me` 会返回偏好，前端刷新后重新同步 `UserContext` 与 CSS。

未实现的上游字段没有被做成空壳 UI：`theme`、`rounded`、`timeZone`、highlight 选项等当前站点没有实际消费方。

Playwright 验证：

- 保存 `PingFang SC` / `Fira Code` / `python3` / `print("hello")` / skipAnimate
- CSS 字体变量和 `UserContext` 立即更新
- 刷新及重新进入设置页后值保持
- 提交页默认语言为 Python 3，Monaco 初始内容为 `print("hello")`
- `skipAnimate=true` 时首页 section 无入场 transition
- 账户设置页原有头像、资料、隐私字段仍正常渲染
- Go 集成测试覆盖非法值 400 和合法值持久化

## 6.12 文件预览（2026-10-30，部分完成 #277）

已移植上游 `[data-preview]` 点击入口和文件类型判断，文件字节通过 Go API 获取后使用共享 Hydro Dialog 展示。

- 文本文件：1 MiB 内直接预览
- 图片：PNG/JPEG/GIF/WebP/BMP/SVG
- 视频：MP4/WebM/OGG
- PDF：iframe 预览
- ZIP/RAR/7z、Office 文件和超过 8 MiB 的文件直接下载
- 支持用户文件、题目文件、活动文件等所有带 `data-preview` 的上游模板
- 关闭 Dialog 后释放 object URL，不残留在 DOM

Playwright 验证用户文件页：

- `preview-text.txt` 打开 Hydro Dialog 并显示原始文本
- `preview-image.png` 打开图片预览，使用 blob URL
- 控制台 0 error

题目文件创建、单文件重命名和批量打包下载仍由 #277 跟踪。

## 6.13 题目文件创建/重命名（2026-10-30，部分完成 #277）

- 恢复题目 testdata 和附加文件的“创建”入口
- 创建 Dialog 支持文件名和初始文本内容，创建后走现有上传 API
- 恢复每行重命名/删除图标
- 新增 `POST /api/v1/problems/:pid/assets/rename`，复制存储对象、更新元数据、删除旧对象
- 重命名 config 文件时同步刷新题目判题配置
- 权限与普通上传/删除一致，仅题目所有者或管理员可操作
- 批量重命名、批量下载和测试数据生成仍待后续 API

Playwright 验证：

- 创建 `input.txt`，内容 `1 2\n`，表格立即出现新文件
- 重命名为 `renamed.txt`，旧文件消失
- 新文件可通过全局预览打开并显示原始文本
- 单文件删除有 Hydro Dialog 确认，删除后回到空状态
- 后端集成测试覆盖文件字节和元数据迁移、旧对象清理、非所有者 403
- `go test ./... -count=1`、`go vet ./...`、`go build ./...`
- `pnpm --dir web lint`、`pnpm --dir web build`

## 6.14 题目文件批量下载（2026-10-30，部分完成 #277）

- 新增 `POST /api/v1/problems/:pid/assets/archive`
- API 校验类型、文件名、重复项和文件存在性后再开始写 ZIP，避免返回被截断的非法压缩包
- 前端恢复“下载选中”，通过 blob 下载 `${pid}-${type}.zip`
- 仅题目所有者和管理员可调用；testdata 与附加文件分别打包
- 文件名、修改时间和内容来自实际存储对象

Playwright 验证：

- 在 testdata 全选 2 个文件后触发浏览器下载
- 下载文件名为 `ARCHE2E-testdata.zip`
- `unzip -l` 显示 2 个条目，内容与上传文件一致
- 后端集成测试覆盖选中文件 ZIP
- `go test ./... -count=1`、`go vet ./...`、`go build ./...`
- `pnpm --dir web lint`、`pnpm --dir web build`

批量重命名和 testdata 生成仍待完成。

## 6.15 题目文件批量重命名（2026-10-30，部分完成 #277）

重新展示“重命名选中”并使用上游的批量替换交互思路：

- 支持普通字符串替换
- 支持 `/pattern/flags` 正则替换
- 支持统一添加前缀/后缀
- 确认表实时展示原文件名和新文件名
- 空文件名、重复文件名、无实际变化和非法正则会在 Dialog 内阻止提交
- 复用 `POST /assets/rename`，批量更新存储对象和元数据

Playwright 验证：

- 选择 `batch-a.in`、`batch-b.in`，添加前缀 `new-`
- 预览为 `new-batch-a.in`、`new-batch-b.in`
- 提交后列表和元数据正确更新
- 无变化时 Dialog 不关闭并显示错误
- 控制台 0 error

## 6.17 题目测试数据自动生成（2026-10-30）

恢复上游题目文件页的“生成测试数据(Beta)”区域，不新增独立执行器，直接复用 `hydrojudge` 原生 `type=generate` 任务：

- Go API 的 `POST /api/v1/problems/:pid/generate` 校验生成器和标准程序必须是已有 testdata，并创建 `type=generate` task
- TaskPump 同时领取 `judge` 与 `generate` 任务，继续沿用 lease、续租和断线恢复
- `/judge/upload` 把生成的 `1.in`、`1.out` 等文件写入题目 testdata 和题目元数据
- 兼容 `hydrojudge@4.0.6` 使用 `superagent.attach()` 时不发送 filename 的 multipart 文件字段；此前 Go `FormFile` 会把该字段当作普通表单值并返回 400
- 前端复用上游 `FileSelectAutoComplete` 选择生成器和标准程序，提交后轮询生成记录，完成后刷新文件表并展示完成提示
- 普通记录列表、重判和题目提交/通过计数排除 `type=generate`，不会混入用户评测数据

Playwright 验证：

- 生成器与标准程序自动补全均可搜索并选中
- 点击“生成”后显示生成状态，完成后显示“测试数据生成完成”
- testdata 表新增 `1.in` 至 `10.in`、`1.out` 至 `10.out`
- 新生成的 `1.in` 可通过上游 `[data-preview]` Dialog 预览
- 真实本地 `hydrojudge@4.0.6 + go-judge v1.13.3` 任务状态最终为 `accepted(1)`，所有生成文件写入 MongoDB 元数据和磁盘存储

## 6.16 题目包导出（2026-10-30）

- 新增 `POST /api/v1/problems/archive`，接受一个或多个 pid/docId
- 每题输出 `problem.yaml`、`problem.md`、`testdata/` 和 `additional_file/`
- DB 中的判题配置在缺少 testdata/config.yaml 时自动补入
- 文件名、存储对象和全部文件可读性在写响应头前校验，避免下载截断 ZIP
- 普通用户只能导出自己的题目，管理员可导出全部
- 题目详情侧栏恢复“下载”，题目列表编辑模式恢复“下载选中”
- 多域复制入口仍不迁移

Playwright 验证：

- 题目详情下载单题 ZIP，包含 metadata、题面、testdata 和 config.yaml
- 题目列表 Shift 选择两题后下载 `problems.zip`
- ZIP 中每题目录和文件完整
- 后端集成测试覆盖 P5 的 metadata、题面、testdata、config 和附加文件，以及非所有者 403
- `go test ./... -count=1`、`go vet ./...`、`go build ./...`
- `pnpm --dir web lint`、`pnpm --dir web build`

## 7. 当前已知差异

```text
页面：首页 / 作业文件 / 训练文件 / 比赛管理页 / 后台控制面板 / 用户权限 / 登录 / 比赛 / 比赛答疑管理 / 作业 / 训练 / 讨论列表 / 讨论详情 / 讨论创建与编辑 / 用户主页 / 排行榜 / 题目列表 / 题目详情 / 题目编辑 / 比赛列表 / 比赛详情 / 比赛榜单 / 作业列表 / 作业详情 / 训练列表 / 训练详情 / 题目文件 / 题目提交 / 记录列表 / 记录详情
旧站行为：Nunjucks 模板、Markdown 编辑器、日期控件、自动补全和 Ctrl+Enter
新站行为：直接渲染 @hydrooj/ui-default 4.58.5 原模板和 tag-stat 插件原模板；比赛/作业/训练上传接入活动文件 API，题目上传接入题目附加文件 API；日期控件、自动补全和编辑器作为交互岛挂载
差异级别：中
是否接受：部分接受
负责人：待定
后续处理：继续处理剩余未迁移功能
```

## 8. 尚未迁移的模块（2026-10-22 更新）

| 模块 | 状态 | 说明 |
|---|---|---|
| `/home/security` 的 OAuth 部分 | 未迁移 | 第三方账号绑定尚未迁移；Passkey、TOTP、密码、邮箱和活动会话已实现 |
| onsite-toolkit | 部分迁移 | 打印站、气球页、IP 自动登录和批量导入映射已实现；线下赛模式、CDP 导出未迁移 |
| a11y | 无需迁移 | 上游 `a11y` 包是安装器辅助脚本（自动设置超管、nexus 布局迁移），与无障碍功能无关 |

已完成的对应记录：

- 题解：见 `docs/problems.md` 的“题解”一节
- 用户主页“题解被赞”：见 `docs/user-profile.md`
- 站内消息：见 `docs/messages.md`
- FPS 导入：见 `docs/problems.md` 的“FPS 导入”一节
- 题目统计页：见 `docs/problems.md` 的“题目统计”一节
- 修改密码与活动会话：见 `docs/security.md`
- Hydro 压缩包导入：见 `docs/problems.md` 的“Hydro 压缩包导入”一节
- 标签统计、活跃日历：见 `docs/tag-stat.md`、`docs/activity-calendar.md`
- 签到、倒计时：见 `docs/homepage.md`
