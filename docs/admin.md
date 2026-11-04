# 后台管理

## 接口

| 方法 | 路径 | 用途 |
|---|---|---|
| GET | `/api/v1/admin/stats` | 管理员只读统计 |
| GET | `/api/v1/admin/users` | 管理员用户列表和搜索 |
| PATCH | `/api/v1/admin/users/:uid/priv` | 修改指定用户的权限位 |

所有接口需要登录用户拥有 Hydro `PRIV_EDIT_SYSTEM` 权限，当前对应 `priv & 1 != 0`。

修改权限的请求体：

```json
{"priv": 3}
```

接口直接更新 Hydro 用户文档中的 `priv` 位掩码。为避免管理员意外锁死后台，当前禁止修改自己的权限。

用户封禁使用 `PATCH /api/v1/admin/users/:uid/ban`，请求体为 `{"banned": true|false}`。封禁会清除 `PRIV_USER_PROFILE` 位、撤销该用户全部 Session；解封恢复该位。不能封禁自己、管理员或 `PRIV_ALL` 超级管理员。

后台权限变更、封禁/解封、公告修改和批量用户导入会写入 `oplog` 集合；`GET /api/v1/admin/audit?page=1&limit=50` 返回最近审计记录，`/admin/audit` 页面展示操作者、目标、动作、详情和时间。

## 统计项

- 用户数
- 题目数
- 比赛数
- 作业数
- 训练数
- 讨论数
- 提交记录数
- 今日提交数
- 待评测任务数

## 前端

- `/admin` 渲染上游 `manage_dashboard.html`：Messages、Information（域头像/名称）和 `manage_base.html` 控制面板侧栏
- `/admin/settings` 渲染上游 `manage_setting.html` + `partials/setting.html`，管理站点名、Logo、Gravatar 前缀、页脚 HTML、公告、About、Scratchpad 开关、首页布局、判题语言表、文件/代码/频率限制、会话过期和分页默认值
- `/admin/users` 渲染上游 `manage_user_priv.html`：默认权限行 + 按 `priv` 分组的用户表，`show_priv` 宏显示相对默认权限的 `+/-` 位
- 默认权限行可编辑；保存后新注册用户继承新值，仍使用旧默认值的用户同步迁移，自定义权限用户不受影响
- `/admin/ip-login` 提供 IP 自动登录映射的列表、新增/更新、编辑和删除；页面复用 `manage_base.html` 控制面板侧栏
- 后台页面共用 `manage_base.html` 侧栏，`ui.getNodes('ControlPanel')` 只声明重写版已实现的控制面板节点
- 权限编辑复用模板自带的 `.dialog__body--set-priv` 表单：点击「Edit」把这段上游 DOM 克隆进对话框，勾选后 `PATCH /api/v1/admin/users/:uid/priv`
- 「Select User」打开用户搜索对话框（`UserSelectAutoComplete`），选中后直接进入该用户的权限对话框
- 只有管理员在导航栏看到“后台”入口

### 设置存储

设置存储在 Hydro 同名的 `system` 集合里（`{_id: key, value: ...}`）。公开接口 `GET /api/v1/settings` 只返回前台实际消费的字段，管理接口 `GET/PUT /api/v1/admin/settings` 需要管理员权限。首页布局使用 `hydrooj.homepage` YAML，保存前校验栏目结构。

分页默认值使用 Hydro 同名配置：`pagination.problem`、`pagination.contest`、`pagination.discussion`、`pagination.reply`、`pagination.record`、`pagination.ranking`、`pagination.solution`、`pagination.training`。列表请求未显式传 `limit` 时由后端读取对应配置，前端也使用相同的公共设置值；显式 `limit` 仍然优先。

判题语言表使用 `hydrooj.langs` YAML，按 Hydro 的点号继承规则合并配置；保存后同时更新 `/api/v1/languages`、判题 hub 和状态页语言列表。

训练报名用户展示使用 Hydro 原版 `training.enrolled-users` 布尔配置，默认开启。关闭后，训练详情不再返回报名用户列表，也不能通过 `?uid=` 查看其他用户的训练进度。

题目分类使用 Hydro 原版 `problem.categories` YAML，结构为“分类 -> 子分类数组”。题目编辑分类和 `category:` 搜索直接使用该配置；题库列表保留旧站 tag-stat 标签统计侧栏。

SMTP 使用 Hydro 同名配置 `smtp.host`、`smtp.port`、`smtp.user`、`smtp.pass`、`smtp.from` 和 `smtp.secure`。后台只返回“密码是否已配置”，不会回传密码明文；留空密码表示保持不变，勾选 Clear SMTP Password 可清除。数据库配置优先于 `MAIL_*` 环境变量，保存后当前进程立即更新 mailer。

站点基址使用 Hydro `server.url`，默认 `/`。可以配置绝对 HTTP(S) URL 或站内路径前缀；忘记密码/邮箱确认邮件和 OAuth callback 优先使用该值，未配置时回退到 `OAUTH_CALLBACK_BASE_URL` 或当前请求 Host。

CDN 使用 Hydro `server.cdn` 和 `server.cdn_dynamic`。`server.cdn` 支持单个或逗号分隔的绝对/协议相对/站内路径前缀；`render.ts` 会把选中的前缀统一注入所有原模板的 `UiContext.cdn_prefix`。

文件与代码限制使用 Hydro 同名配置：`limit.user_files`、`limit.user_files_size`、`limit.contest_files`、`limit.contest_files_size`、`limit.problem_files_max`、`limit.problem_files_max_size` 和 `limit.codelength`。上传、附件操作和代码提交会读取当前有效值。

提交与自测频率限制使用 `limit.submission`、`limit.submission_user`、`limit.pretest` 和 `limit.by_user`；超限返回 `429` 与 `Retry-After`。当前限流器是单进程内存实现，多实例部署需要替换为共享存储。

浏览器用户会话使用 `session.saved_expire_seconds`，judge/未保存会话使用 `session.unsaved_expire_seconds`；保存后运行时创建的会话和 Cookie Max-Age 会使用新值。未配置时保留 `SESSION_TTL` 作为兼容默认。

Hydro 的后台脚本/插件任务管理器不迁移；重写版维护任务统一通过仓库中的 CLI、迁移脚本和 Docker 脚本执行，避免重新引入 Cordis/addon 插件运行时。

### 模板层依赖

`manage_user_priv.html` 的 `show_priv` 宏需要 `bitand` 过滤器（`web/src/upstream/render.ts`），PRIV 位值来自 `web/src/upstream/priv.ts`，与 Hydro `packages/common/permission.ts` 保持一致。

## IP 自动登录

线下赛电脑可以按固定 IP 自动登录。默认关闭，必须在可信局域网内显式启用：

```bash
ONSITE_IP_LOGIN=1
```

映射存储在 `iplogin` 集合：

```json
{"_id":"10.0.0.5","uid":123}
```

管理员接口：

- `GET /api/v1/admin/ip-login`
- `PUT /api/v1/admin/ip-login/:ip`，请求体 `{"uid": 123}`
- `DELETE /api/v1/admin/ip-login/:ip`

启用后，没有有效用户 Session 的 `/api/*` 请求会按客户端 IP 自动签发用户 Session；`/api/v1/auth/logout`、`/api/v1/auth/login` 和判题机接口不会被自动登录拦截。生产反向代理必须正确传递真实客户端 IP，并只对可信局域网开放。

## 用户导入

后台 `/admin/users/import` 渲染上游 `manage_user_import.html`，对应 `POST /api/v1/admin/users/import`：

- 每行一个用户，tab 或逗号分隔：`邮箱, 用户名, 密码, 显示名, 附加JSON`
- 附加 JSON 支持 `school`、`studentId`、`group`（分组不存在时自动创建）和 `ip`（同时写入 IP 自动登录映射）
- `draft=true` 只预览并检查重复邮箱/用户名；`draft=false` 真正创建
- 校验规则与注册一致：邮箱格式、用户名 3-31 字符（或 2 个汉字）、密码 6-255 字符

## 尚未完成

- 尚未被当前前台消费的高级系统配置（配置 schema 编辑器）
- `manage_dashboard` 的 WebSocket 连接检查和「Restart」按钮（出于安全考虑未实现，并从 DOM 移除）
- 题目/比赛/训练编辑仍从各自详情页进入，没有复制到后台单独页面

已完成：用户封禁/解封、用户自助注销、权限变更、默认权限、核心站点设置、批量用户导入、审计日志、讨论节点管理和 IP 自动登录管理及对应页面。
