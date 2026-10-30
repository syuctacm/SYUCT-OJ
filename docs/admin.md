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
- `/admin/settings` 渲染上游 `manage_setting.html` + `partials/setting.html`，目前只暴露站点公告 `bulletin`（Markdown），保存后立即写入 `system` 集合并被首页读取
- `/admin/users` 渲染上游 `manage_user_priv.html`：默认权限行 + 按 `priv` 分组的用户表，`show_priv` 宏显示相对默认权限的 `+/-` 位
- 两个页面共用 `manage_base.html` 侧栏，`ui.getNodes('ControlPanel')` 只声明重写版已实现的控制面板节点
- 权限编辑复用模板自带的 `.dialog__body--set-priv` 表单：点击「Edit」把这段上游 DOM 克隆进对话框，勾选后 `PATCH /api/v1/admin/users/:uid/priv`
- 「Select User」打开用户搜索对话框（`UserSelectAutoComplete`），选中后直接进入该用户的权限对话框
- 只有管理员在导航栏看到“后台”入口

### 设置存储

`bulletin` 存在 Hydro 同名的 `system` 集合里（`{_id: "bulletin", value: "..."}`）。公开接口 `GET /api/v1/settings` 只返回 `bulletin`，管理接口 `GET/PUT /api/v1/admin/settings` 需要管理员权限，长度上限 20000。

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

- 用户删除
- 题目/比赛/训练编辑
- 操作日志和审计
- 控制面板其余节点（脚本管理、配置管理）
- `manage_dashboard` 的 WebSocket 连接检查（`#messages` 目前为空）和「Restart」按钮（未实现，已从 DOM 移除）
- 修改系统默认权限（默认权限行的 Edit 链接已移除）
- IP 自动登录的可视化管理页面（目前提供管理 API）
