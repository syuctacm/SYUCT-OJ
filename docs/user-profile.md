# 用户主页 API 与页面

## 接口

| 方法 | 路径 | 用途 |
|---|---|---|
| GET | `/api/v1/users/:uid` | 公开用户资料（含已通过题目、RP、排名） |
| GET | `/api/v1/ranking` | 全站 RP 排行榜（分页） |
| GET | `/api/v1/users?search=...` | 用户名/显示名搜索 |
| GET | `/api/v1/users?auto=1,2` | 按 UID 批量读取 |
| GET | `/api/v1/groups?search=...` | 分组名搜索 |
| GET | `/api/v1/groups?names=2026` | 按分组名批量读取 |

公开资料返回用户 ID、用户名、显示名、头像、简介、注册/登录时间、提交/通过统计、已通过题目（`acceptedProblems`，最多 100 道、按 docId 排序、隐藏题不返回）、最近参加的比赛/作业（`tdocs`）以及从 `domain.user` 读取的 RP、等级和全站排名（RP 为 0 时排名为 0），并返回公开的 `qq`、`wechat` 和最近浏览器会话活跃时间 `lastSeenAt`；不返回邮箱、密码、学号和学校字段。自动补全接口只返回 ID、用户名、显示名和头像；分组接口只返回分组名与成员 UID。

## 前端

- `/user/:uid`：用户主页
- `/ranking`：全站 RP 排行榜（上游 `ranking.html`）
- 登录后的导航栏用户名会链接到自己的主页
- 页面由 `HydroTemplate` 直接渲染上游 `user_detail.html`：资料头部、联系方式图标、简介标签页、已通过题目标签页和侧栏统计都来自原模板
- 活跃日历（原 Activity Calendar 插件）作为标签页挂载到原版标签结构中
- 最近活动标签页直接渲染上游 `partials/user_detail/activity.html`，列出参加过的比赛和作业
- 在线状态读取 `lastSeenAt`，沿用原模板的五分钟规则显示「刚刚」/离线
- 微信号来自公开资料，原模板的微信复制按钮由 clipboard 适配层接管

## 头像

用户文档里的 `avatar` 是带 scheme 的字符串，渲染前统一走 `web/src/upstream/avatar.ts`（移植自上游 `packages/hydrooj/src/lib/avatar.ts`）：

| 值 | 渲染结果 |
|---|---|
| `gravatar:mail@example.com` | `//cn.gravatar.com/avatar/<md5(小写邮箱)>?d=mm&s=<size>` |
| `qq:123456789` | `//q1.qlogo.cn/g?b=qq&nk=123456789&s=160` |
| `github:hydro-dev` | `//github.com/hydro-dev.png?size=<min(size,460)>` |
| `url:https://…` | 原样输出 |
| 空值 | 空邮箱的 gravatar（与旧站一致） |

gravatar 需要 MD5，浏览器没有同步实现，因此在 `web/src/upstream/md5.ts` vendored 了一份 RFC 1321 实现，并用 RFC 测试向量和 `md5sum` 对照验证（含 UTF-8 中文）。Gravatar 前缀沿用上游默认值 `//cn.gravatar.com/avatar/`，等设置系统接入后再改成可配置项。

所有页面和两个用户自动补全组件都使用同一个 `avatarUrl()`，不再各自内联 `value || '/logo.png'`。

上传头像使用 `POST /api/v1/user/avatar`，只接受 JPG/PNG 且不超过 8 MiB，按上游路径保存为 `user/<uid>/.avatar.<ext>`，并把 `avatar` 写成带版本参数的公开地址。`GET /api/v1/users/:uid/avatar` 不依赖 `_files` 元数据，匿名即可读取；上传新格式时会清理旧的 `.avatar.*` 文件。

## 复制联系方式

联系方式栏的 `[data-copy]` 按钮由 `web/src/upstream/clipboard.ts` 接管（移植自 `components/clipboard.page.ts`）：base64 解码后写入剪贴板，成功/失败都通过 Mantine 通知提示（`web/src/components/Notification.tsx`，移植自 `components/notification/index.ts`，`NotificationsHost` 挂在 `Layout` 上）。

## 账户设置

`/home/settings?category=account` 由 `HydroTemplate` 直接渲染上游 `home_settings.html` + `partials/setting.html`，字段来自 `packages/hydrooj/src/model/setting.ts` 的 `AccountSetting(...)`：

- 个人信息：Avatar、QQ、Gender、Bio（挂原版 Markdown 编辑器）、School、Student ID，Phone 保持 disabled
- 个性化：个人主页背景图

`PATCH /api/v1/user/settings` 校验规则：头像必须是 `gravatar:` / `qq:` / `github:` / `url:` 形式（与上游 `avatar.validate()` 一致），性别 0-2，背景图不允许空白/引号/括号，QQ ≤32、Bio ≤10000、School ≤100、Student ID ≤64。

`school` 和 `studentId` 按上游 `FLAG_PRIVATE` 处理：只有本人和管理员能从 `/api/v1/users/:uid` 读到；`qq`、`gender`、`backgroundImage`、`bio` 公开。

账号侧栏复用上游 `components/home.html`，只删除了多域入口；站内消息、我的文件、账户设置和安全设置都已接入。头像上传区块保留上游 `#upload_avatar`，点击“Change avatar”打开 JPG/PNG 选择器；provider 文本字段仍可继续使用。

## 我的文件

`/file` 由 `HydroTemplate` 渲染上游 `home_files.html`，文件表来自 `partials/files.html`：

- 存储路径 `user/<uid>/<name>`，元数据写入用户文档的 `_files`
- `GET/POST/DELETE /api/v1/user/files`，本人下载 `GET /api/v1/user/files/:name`
- 公开下载 `GET /api/v1/users/:uid/files/:name`，用于讨论图片等可匿名访问的场景
- 配额沿用上游默认：最多 100 个文件、总计 128 MiB；重名上传返回 409
- 上传、全选、批量删除、拖拽上传都挂在上游 DOM 上，React 只做事件适配
- 账户侧栏「我的文件」入口随 `PRIV_CREATE_FILE` 显示并高亮

## 题解统计

用户主页的“Solutions Liked”读取题解投票总数：`GET /api/v1/users/:uid` 返回 `nSolution`（发表题解数）和 `nLiked`（收到的票数合计），由 `problem.Repository.SolutionStats` 统计。

## 尚未完成

- 用户主页的完整题目/比赛历史（当前只展示已通过题目）
